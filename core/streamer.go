package core

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	mtx "github.com/bluenviron/mediamtx/core"
	"github.com/nknorg/nkn-sdk-go"
)

var segmentSendConfig = &nkn.MessageConfig{
	Unencrypted: true,
	NoReply:     true,
}

type Streamer struct {
	isActive bool
	quit     chan struct{}

	nknClient   *nkn.MultiClient
	mtxCore     *mtx.Core
	transcoders []Transcode

	EventHandler     Event
	lastRtmpSegment  time.Time
	sourceResolution int
	sourceFramerate  int
	sourceCodec      string
	viewers          *Viewers
	lastSegment      [][]byte
	thumbnail        []byte
	config           *Config
	segmentId        int
}

func NewStreamer() *Streamer {
	return &Streamer{
		quit: make(chan struct{}),
	}
}

func (s *Streamer) Start() error {
	ffmpegInstalled := checkFfmpegInstalled()
	ffmpegInstalledStr := "TRUE"
	if !ffmpegInstalled {
		ffmpegInstalledStr = "FALSE"
	}

	payload := map[string]string{
		"Type":        "FFMPEG_UPDATE",
		"IsInstalled": ffmpegInstalledStr,
		"OS":          runtime.GOOS,
	}
	s.EventHandler.Emit(payload)

	if !ffmpegInstalled {
		return nil
	}

	// Save original stdout/stderr
	origStdout := os.Stdout
	origStderr := os.Stderr

	// Create a pipe
	r, w, _ := os.Pipe()

	// Redirect stdout and stderr to the pipe
	os.Stdout = w
	os.Stderr = w

	// Read from the pipe in a goroutine
	watcher := NewLogWatcher()

	// Register RTMP listener watcher
	watcher.Register("[RTMP] listener opened on :", func(line string) bool {
		const prefix = "[RTMP] listener opened on :"
		idx := strings.Index(line, prefix)
		if idx == -1 {
			return false
		}

		port := strings.TrimSpace(line[idx+len(prefix):])
		fmt.Println("RTMP port detected:", port)

		payload := map[string]string{
			"Type":  "RTMP_PORT",
			"Value": port,
		}
		s.EventHandler.Emit(payload)

		// Return true so it unregisters after this
		return true
	})

	// Register RTMP publishing watcher
	watcher.Register("is publishing to path", func(line string) bool {
		payload := map[string]string{
			"Type": "RTMP_PUBLISH",
		}
		s.EventHandler.Emit(payload)

		// Return true so it unregisters after this
		return true
	})

	//Once we are running listen to exits;
	watcher.Register("destroyed: terminated", func(line string) bool {
		payload := map[string]string{
			"Type": "RTMP_TERMINATED",
		}
		s.EventHandler.Emit(payload)
		return false
	})

	go func() {
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			line := scanner.Text()

			// 2. Echo to the original terminal (dev output)
			watcher.Dispatch(line)
			fmt.Fprintln(origStdout, "Captured: "+line)
		}
	}()

	if s.isActive {
		fmt.Fprintln(origStdout, "stream already active")
	}

	payload = map[string]string{
		"Type":       "NKN_UPDATE",
		"NumClients": "0",
		"Status":     "Starting",
	}
	s.EventHandler.Emit(payload)

	s.isActive = true

	fmt.Println("Welcome to go-novon a golang client for RTMP streaming to novon")
	fmt.Println("")

	ctx, cancel := context.WithCancel(context.Background())

	var err error
	s.config, err = NewConfig("./config.json")
	if err != nil {
		panic(err)
	}

	s.viewers = NewViewers(30 * time.Second)
	s.viewers.StartCleanup(ctx, time.Second)
	defer s.viewers.Cleanup()

	s.nknClient = s.createClient()

	var ok bool
	s.mtxCore, ok = mtx.New(os.Args[1:], s.publishTSPart)
	if !ok {
		os.Exit(1)
	}

	loadPanels()
	s.maintainStream(ctx)
	s.receiveMessages(ctx)
	s.reportNumClients(ctx)

	go func() {
		s.mtxCore.Wait()
		log.Println("mtxCore awaited")

		// Flush and restore
		_ = w.Close()
		os.Stdout = origStdout
		os.Stderr = origStderr

		cancel()
	}()

	return nil
}

func (s *Streamer) reportNumClients(ctx context.Context) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				log.Println("maintainStream: stopping")
				return
			default:
				time.Sleep(time.Second * 3)

				connectedCount := NUM_SUB_CLIENTS
				for _, v := range s.nknClient.GetClients() {
					if v.IsClosed() {
						connectedCount--
					}
				}

				payload := map[string]string{
					"Type":       "NKN_UPDATE",
					"NumClients": strconv.Itoa(connectedCount),
					"Status":     "Connected",
				}

				s.EventHandler.Emit(payload)
			}
		}
	}()
}

func (s *Streamer) Stop() {
	if !s.isActive {
		log.Println("Stream not active.")
		return
	}

	s.mtxCore.Close()
	log.Println("mtxCore closed")

	s.nknClient.Close()
	log.Println("nkn client closed")

	close(s.quit)
	s.isActive = false
}

func (s *Streamer) IsActive() bool {
	return s.isActive
}

func (s *Streamer) ClientAddress() string {
	return s.nknClient.Address()
}

func (s *Streamer) ClientWalletAddress() string {
	return s.nknClient.Account().WalletAddress()
}

func (s *Streamer) createClient() *nkn.MultiClient {
	seed, _ := hex.DecodeString(s.config.Seed)
	account, err := nkn.NewAccount(seed)
	if err != nil {
		log.Panic(err)
	}

	client, _ := nkn.NewMultiClient(account, "", NUM_SUB_CLIENTS, false, &nkn.ClientConfig{
		ConnectRetries:   10,
		AllowUnencrypted: true,
	})

	connectedClientsCount := 0

	//5% startup connection leniency, improves startup time dramatically with minimal risk for service disruption.
	for i := 0; i < NUM_SUB_CLIENTS-(NUM_SUB_CLIENTS/20); i++ {
		<-client.OnConnect.C
		connectedClientsCount++

		payload := map[string]string{
			"Type":       "NKN_UPDATE",
			"NumClients": strconv.Itoa(connectedClientsCount),
			"Status":     "Starting",
		}
		s.EventHandler.Emit(payload)
	}

	//Then wait for the rest!
	go func() {
		for i := 0; i < NUM_SUB_CLIENTS/20; i++ {
			<-client.OnConnect.C
			connectedClientsCount++

			payload := map[string]string{
				"Type":       "NKN_UPDATE",
				"NumClients": strconv.Itoa(connectedClientsCount),
				"Status":     "Connected",
			}
			s.EventHandler.Emit(payload)
		}
	}()

	log.Println("connected to NKN")
	log.Println("Your address", client.Address())

	return client
}

type ChannelInfo struct {
	Panels        string      `json:"panels"`
	Viewers       int         `json:"viewers"`
	Role          string      `json:"role"`
	QualityLevels []Transcode `json:"qualityLevels"`
}

func (s *Streamer) receiveMessages(ctx context.Context) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				log.Println("receiveMessages: stopping")
				return
			case msg := <-s.nknClient.OnMessage.C:
				if msg == nil {
					continue
				}

				//Always reply to panel, this can be displayed when we are not broadcasting.
				if len(msg.Data) == 9 && string(msg.Data[:]) == "getpanels" {
					go s.replyText(panels, msg)
					continue
				}

				//Always reply to panel, this can be displayed when we are not broadcasting.
				if len(msg.Data) == 11 && string(msg.Data[:]) == "channelinfo" {

					role := ""
					if msg.Src == s.config.Owner {
						role = "owner"
					}

					qualityLevels := make([]Transcode, 0)
					qualityLevels = append(qualityLevels, Transcode{
						Resolution: s.sourceResolution,
						Framerate:  s.sourceFramerate,
					})

					qualityLevels = append(qualityLevels, s.transcoders...)

					response := ChannelInfo{
						Panels:        panels,
						Viewers:       len(viewerAddresses),
						Role:          role,
						QualityLevels: qualityLevels,
					}

					json, err := json.Marshal(response)
					if err != nil {
						log.Println("error on creating channel info response", err.Error())
					}

					go s.replyText(string(json), msg)
					continue
				}

				//If we're not broadcasting don't reply to anything.
				if !s.isBroadcasting() {
					time.Sleep(time.Millisecond * 100)
					continue
				}

				if len(msg.Data) == 4 && string(msg.Data[:]) == "ping" {
					isNew := s.viewers.AddOrUpdateAddress(msg.Src)
					if isNew {
						log.Println("viewer joined: ", msg.Src)
					}
					//Send last segment to newly joined
					if isNew {
						for _, chunk := range s.lastSegment {
							go s.sendToClient(msg.Src, chunk)
						}
					}
				} else if len(msg.Data) == 9 && string(msg.Data[:]) == "thumbnail" {
					go s.reply(s.thumbnail, msg)
				} else if len(msg.Data) == 10 && string(msg.Data[:]) == "disconnect" {
					s.viewers.Remove(msg.Src)
				} else if len(msg.Data) == 9 && string(msg.Data[:]) == "viewcount" {
					go s.replyText(strconv.Itoa(len(viewerAddresses)), msg)
				} else if len(msg.Data) == 10 && string(msg.Data[:]) == "donationid" {
					go s.replyText(generateDonationEntry(), msg)
				} else if len(msg.Data) == 8 && strings.Contains(string(msg.Data[:]), "quality") {
					qLevelStr, _ := strings.CutPrefix(string(msg.Data[:]), "quality")
					qLevel, _ := strconv.Atoi(qLevelStr)
					s.viewers.viewerQuality[msg.Src] = qLevel
					go s.replyText(strconv.Itoa(s.segmentId), msg)
				} else {
					s.DecodeMessage(msg)
				}
			}
		}
	}()
}

func (s *Streamer) maintainStream(ctx context.Context) {
	go func() {
		isSubscribed := false
		lastSubscribe := time.Time{}

		for {
			select {
			case <-ctx.Done():
				log.Println("maintainStream: stopping")
				return
			default:
				if s.isBroadcasting() {
					if !isSubscribed || time.Since(lastSubscribe).Seconds() > 100*20 {
						lastSubscribe = time.Now()
						go s.nknClient.Subscribe("", "novon", 100, s.config.Title, nil)
						isSubscribed = true
					}
				} else {
					if isSubscribed {
						go s.nknClient.Unsubscribe("", "novon", nil)
						isSubscribed = false
					}
				}
				time.Sleep(time.Second)
			}
		}
	}()
}

func (s *Streamer) ChunkByByteSizeWithMetadata(data []byte, chunkSize int, segmentId int) [][]byte {
	if chunkSize <= 0 {
		panic("chunkSize must be positive")
	}

	totalChunks := (len(data) / chunkSize) + 1
	chunks := make([][]byte, 0, totalChunks)

	chunkId := 0

	buffer := bytes.NewBuffer(data)
	for {
		chunk := buffer.Next(chunkSize)
		if len(chunk) == 0 {
			break
		}

		prefix := make([]byte, 3*4)
		binary.LittleEndian.PutUint32(prefix[:4], uint32(segmentId))
		binary.LittleEndian.PutUint32(prefix[4:8], uint32(chunkId))
		binary.LittleEndian.PutUint32(prefix[8:], uint32(totalChunks))

		chunks = append(chunks, append(prefix, chunk...))
		chunkId++
	}

	return chunks
}

func (s *Streamer) EmitEvent(eventType string, data ...map[string]string) {
	payload := map[string]string{
		"Type": eventType,
	}
	if len(data) > 0 {
		for k, v := range data[0] {
			payload[k] = v
		}
	}
	s.EventHandler.Emit(payload)
}

func (s *Streamer) publishTSPart(segment []byte) {

	if !s.isBroadcasting() {
		info, err := s.probeVideoInfo(segment)
		if err != nil {
			panic(err)
		}

		s.sourceCodec = info["codec"]
		s.sourceResolution, _ = strconv.Atoi(strings.Split(info["resolution"], "x")[1])
		s.sourceFramerate, _ = strconv.Atoi(strings.Split(info["framerate"], "/")[0])

		log.Println("Receiving codec:", s.sourceCodec, "resolution:", s.sourceResolution, "framerate:", s.sourceFramerate)

		s.transcoders = s.getTranscoders(s.config)
		for _, v := range s.transcoders {
			log.Println("Stream will be transcoded in:", v.Resolution, "p", v.Framerate)
		}
	}

	s.lastRtmpSegment = time.Now()
	//os.WriteFile("test.ts", segment, os.FileMode(0644))

	go func() {
		sourceChunks := s.ChunkByByteSizeWithMetadata(segment, CHUNK_SIZE, s.segmentId)
		transcodedChunksArray := make([][][]byte, 0)
		transcodedChunksArray = append(transcodedChunksArray, sourceChunks)

		s.EmitEvent("PUBLISH", map[string]string{
			"numViewers":  strconv.Itoa(len(viewerAddresses)),
			"segmentSize": strconv.Itoa(len(segment)),
			"numChunks":   strconv.Itoa(len(sourceChunks)),
		})

		//No transcoding, publish to all viewers in source quality.
		if len(s.transcoders) == 0 {
			for i := 0; i < len(sourceChunks); i++ {
				go s.publish(sourceChunks[i])
			}
			s.segmentId++
		} else {
			startTranscoderTime := time.Now()

			for _, t := range s.transcoders {

				beginTime := time.Now()
				segment = s.resizeSegment(t, segment)
				timeSpent := time.Since(beginTime).Milliseconds()

				tChunks := s.ChunkByByteSizeWithMetadata(segment, CHUNK_SIZE, s.segmentId)
				transcodedChunksArray = append(transcodedChunksArray, tChunks)
				log.Printf("Transcoded -%v@%v size: %v, chunks: %v, timeSpent: %v\n", t.Resolution, t.Framerate, len(segment), len(tChunks), timeSpent)
			}
			s.segmentId++

			if len(viewerAddresses) > 0 {
				s.publishQualityLevels(transcodedChunksArray...)
			}

			totalTranscodingMs := time.Since(startTranscoderTime).Milliseconds()
			if totalTranscodingMs > 1000 && totalTranscodingMs < 2000 {
				log.Printf("WARNING: Total transcoding time '%vms' approaching segment duration, consider less transcoding configurations.", totalTranscodingMs)
			} else if totalTranscodingMs > 2000 {
				log.Printf("DANGER: Total transcoding time '%vms' exceeds segment duration, stream will suffer interrupts, reduce or remove transcoding configurations.", totalTranscodingMs)
			}
		}

		if (s.segmentId-1)%10 == 0 {
			go s.screengrabSegment(segment)
		}

		//For fastest join times we take the lowest quality level
		s.lastSegment = transcodedChunksArray[len(transcodedChunksArray)-1]
	}()
}

func (s *Streamer) screengrabSegment(segment []byte) {
	// Output image file
	width := "256"
	height := "144"

	// Command arguments for ffmpeg
	cmd := exec.Command("ffmpeg",
		"-i", "-", // read from stdin (pipe)
		"-vframes", "1",
		"-vf", fmt.Sprintf("scale=%s:%s", width, height), // resize filter
		"-f",
		"image2pipe",
		"-")

	var stdinPipe, stderrPipe bytes.Buffer
	cmd.Stdin = &stdinPipe
	cmd.Stderr = &stderrPipe

	// Write MPEG-TS data to stdin pipe
	stdinPipe.Write(segment)

	var err error
	s.thumbnail, err = cmd.Output()

	if err != nil {
		log.Println("Error capturing screenshot:", err)
		log.Println("FFmpeg stderr:", stderrPipe.String())
		return
	}

	log.Println("Screenshot captured successfully.")
}

func (s *Streamer) resizeSegment(transcode Transcode, segment []byte) []byte {
	//ultrafast superfast veryfast faster fast medium (default) slow slower veryslow

	// Command arguments for ffmpeg
	cmd := exec.Command("ffmpeg",
		"-hwaccel", "auto",
		"-i", "-", // read from stdin (pipe)
		"-c:v", "libx264", // specify video encoder (optional)
		"-crf", "30", // set constant rate factor (quality)
		"-preset", "ultrafast", // set encoding preset for faster processing
		"-acodec", "copy",
		"-filter:v", fmt.Sprintf("scale=-2:%d,fps=%d", transcode.Resolution, transcode.Framerate),
		"-copyts",
		"-f", "mpegts",
		"-")

	var stdinPipe, stderrPipe bytes.Buffer
	cmd.Stdin = &stdinPipe
	cmd.Stderr = &stderrPipe

	// Write MPEG-TS data to stdin pipe
	stdinPipe.Write(segment)

	resizedSegment, err := cmd.Output()

	if err != nil {
		log.Println("Error capturing screenshot:", err)
		log.Println("FFmpeg stderr:", stderrPipe.String())
		return nil
	}

	return resizedSegment
}

func (s *Streamer) probeVideoInfo(segment []byte) (map[string]string, error) {
	// Create ffprobe command with pipe input
	cmd := exec.Command("ffprobe", "-v", "quiet", "-print_format", "json", "-show_format", "-show_streams", "-i", "-")

	var stdinPipe bytes.Buffer
	cmd.Stdin = &stdinPipe

	// Write MPEG-TS data to stdin pipe
	stdinPipe.Write(segment)

	// Capture ffprobe output
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("error probing video info: %w", err)
	}

	// Parse ffprobe JSON output
	var info map[string]interface{}
	err = json.Unmarshal(out, &info)
	if err != nil {
		return nil, fmt.Errorf("error parsing ffprobe output: %w", err)
	}

	// Extract relevant info (modify as needed)
	result := map[string]string{}
	if streams, ok := info["streams"].([]interface{}); ok {
		for _, stream := range streams {
			if streamMap, ok := stream.(map[string]interface{}); ok {
				if codecType, ok := streamMap["codec_type"].(string); ok && codecType == "video" {
					result["codec"] = streamMap["codec_name"].(string)
					if width, ok := streamMap["width"].(float64); ok {
						result["resolution"] = fmt.Sprintf("%.0fx%.0f", width, streamMap["height"].(float64))
					}
					if r_frame_rate, ok := streamMap["r_frame_rate"].(string); ok {
						result["framerate"] = r_frame_rate
					}
					break // Extract info only from the first video stream
				}
			}
		}
	}

	return result, nil
}

func checkFfmpegInstalled() bool {
	// Command to check for ffmpeg (replace with actual command if needed)
	cmd := exec.Command("ffmpeg", "-version")

	err := cmd.Run()
	if err != nil {
		// Handle ffmpeg not found error
		log.Println("Error: ffmpeg is not installed. Please install ffmpeg and try again.")
		return false
	}

	// ffmpeg is available, continue with your application logic
	log.Println("ffmpeg is installed. Proceeding...")
	return true
}

func (s *Streamer) isBroadcasting() bool {
	return time.Since(s.lastRtmpSegment).Seconds() < 5
}
