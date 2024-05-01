package main

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/bluenviron/mediamtx/core"
	"github.com/nknorg/nkn-sdk-go"
)

var client *nkn.MultiClient

const NUM_SUB_CLIENTS = 96
const VIEWER_SUB_CLIENTS = 3
const CHUNK_SIZE = 64000

var segmentId = 0

var lastSegment [][]byte
var thumbnail []byte
var config *Config

var viewers *Viewers

var segmentSendConfig = &nkn.MessageConfig{
	Unencrypted: true,
	NoReply:     true,
}

var lastRtmpSegment = time.Time{}

var sourceResolution int
var sourceFramerate int
var sourceCodec string

var transcoders []Transcode

func main() {
	fmt.Println("Welcome to go-novon a golang client for RTMP streaming to novon")
	fmt.Println("")

	checkFfmpegInstalled()

	var err error
	config, err = NewConfig("./config.json")
	if err != nil {
		panic(err)
	}

	viewers = NewViewers(30 * time.Second)
	viewers.StartCleanup(time.Second)
	defer viewers.Cleanup()

	client = createClient()

	s, ok := core.New(os.Args[1:], publishTSPart)
	if !ok {
		os.Exit(1)
	}

	maintainStream()
	loadPanels()
	receiveMessages()

	s.Wait()
}

func createClient() *nkn.MultiClient {
	seed, _ := hex.DecodeString(config.Seed)
	account, err := nkn.NewAccount(seed)
	if err != nil {
		log.Panic(err)
	}

	client, _ := nkn.NewMultiClient(account, "", NUM_SUB_CLIENTS, false, &nkn.ClientConfig{
		ConnectRetries:   10,
		AllowUnencrypted: true,
	})

	//5% startup connection leniency, improves startup time dramatically with minimal risk for service disruption.
	for i := 0; i < NUM_SUB_CLIENTS-(NUM_SUB_CLIENTS/20); i++ {
		<-client.OnConnect.C
	}
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

func receiveMessages() {
	go func() {
		for {
			msg := <-client.OnMessage.C
			if msg == nil {
				continue
			}

			//Always reply to panel, this can be displayed when we are not broadcasting.
			if len(msg.Data) == 9 && string(msg.Data[:]) == "getpanels" {
				go replyText(panels, msg)
				continue
			}

			//Always reply to panel, this can be displayed when we are not broadcasting.
			if len(msg.Data) == 11 && string(msg.Data[:]) == "channelinfo" {

				role := ""
				if msg.Src == config.Owner {
					role = "owner"
				}

				qualityLevels := make([]Transcode, 0)
				qualityLevels = append(qualityLevels, Transcode{
					Resolution: sourceResolution,
					Framerate:  sourceFramerate,
				})

				qualityLevels = append(qualityLevels, transcoders...)

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

				go replyText(string(json), msg)
				continue
			}

			//If we're not broadcasting don't reply to anything.
			if !isBroadcasting() {
				time.Sleep(time.Millisecond * 100)
				continue
			}

			if len(msg.Data) == 4 && string(msg.Data[:]) == "ping" {
				isNew := viewers.AddOrUpdateAddress(msg.Src)
				if isNew {
					log.Println("viewer joined: ", msg.Src)
				}
				//Send last segment to newly joined
				if isNew {
					for _, chunk := range lastSegment {
						go sendToClient(msg.Src, chunk)
					}
				}
			} else if len(msg.Data) == 9 && string(msg.Data[:]) == "thumbnail" {
				go reply(thumbnail, msg)
			} else if len(msg.Data) == 10 && string(msg.Data[:]) == "disconnect" {
				viewers.Remove(msg.Src)
			} else if len(msg.Data) == 9 && string(msg.Data[:]) == "viewcount" {
				go replyText(strconv.Itoa(len(viewerAddresses)), msg)
			} else if len(msg.Data) == 10 && string(msg.Data[:]) == "donationid" {
				go replyText(generateDonationEntry(), msg)
			} else if len(msg.Data) == 8 && string(msg.Data[:]) == "quality0" {
				viewers.viewerQuality[msg.Src] = 0
				go replyText(strconv.Itoa(segmentId), msg)
			} else if len(msg.Data) == 8 && string(msg.Data[:]) == "quality1" {
				viewers.viewerQuality[msg.Src] = 1
				go replyText(strconv.Itoa(segmentId), msg)
			} else if len(msg.Data) == 8 && string(msg.Data[:]) == "quality2" {
				viewers.viewerQuality[msg.Src] = 2
				go replyText(strconv.Itoa(segmentId), msg)
			} else {
				DecodeMessage(msg)
			}
		}
	}()
}

func maintainStream() {
	isSubscribed := false
	lastSubscribe := time.Time{}

	go func() {
		for {
			if isBroadcasting() {
				// We're receiving segments, subscribe if not already, or if we need a resub
				if !isSubscribed || time.Since(lastSubscribe).Seconds() > 100*20 {
					lastSubscribe = time.Now()
					go client.Subscribe("", "novon", 100, config.Title, nil)
					isSubscribed = true
				}
			} else {
				// No recent segments, unsubscribe if subscribed
				if isSubscribed {
					go client.Unsubscribe("", "novon", nil)
					isSubscribed = false
				}
			}
			time.Sleep(time.Second)
		}
	}()
}

func ChunkByByteSizeWithMetadata(data []byte, chunkSize int, segmentId int) [][]byte {
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

func publishTSPart(segment []byte) {

	if !isBroadcasting() {
		info, err := probeVideoInfo(segment)
		if err != nil {
			panic(err)
		}

		sourceCodec = info["codec"]
		sourceResolution, _ = strconv.Atoi(strings.Split(info["resolution"], "x")[1])
		sourceFramerate, _ = strconv.Atoi(strings.Split(info["framerate"], "/")[0])

		log.Println("Receiving codec:", sourceCodec, "resolution:", sourceResolution, "framerate:", sourceFramerate)

		transcoders = getTranscoders(config)
		for _, v := range transcoders {
			log.Println("Stream will be transcoded in:", v.Resolution, "p", v.Framerate)
		}
	}

	lastRtmpSegment = time.Now()
	//os.WriteFile("test.ts", segment, os.FileMode(0644))

	go func() {
		sourceChunks := ChunkByByteSizeWithMetadata(segment, CHUNK_SIZE, segmentId)
		transcodedChunksArray := make([][][]byte, 0)
		transcodedChunksArray = append(transcodedChunksArray, sourceChunks)

		//No transcoding, publish to all viewers in source quality.
		if len(transcoders) == 0 {
			log.Println("Broadcasting -", "viewers:", len(viewerAddresses), "source size:", len(segment), "source chunks:", len(sourceChunks))
			for i := 0; i < len(sourceChunks); i++ {
				go publish(sourceChunks[i])
			}
			segmentId++
		} else {

			startTranscoderTime := time.Now()
			log.Println("Broadcasting -", "viewers:", len(viewerAddresses), "source size:", len(segment), "source chunks:", len(sourceChunks))
			for _, t := range transcoders {

				beginTime := time.Now()
				segment = resizeSegment(t, segment)
				timeSpent := time.Since(beginTime).Milliseconds()

				tChunks := ChunkByByteSizeWithMetadata(segment, CHUNK_SIZE, segmentId)
				transcodedChunksArray = append(transcodedChunksArray, tChunks)
				log.Printf("Transcoded -%v@%v size: %v, chunks: %v, timeSpent: %v\n", t.Resolution, t.Framerate, len(segment), len(tChunks), timeSpent)
			}
			segmentId++

			if len(viewerAddresses) > 0 {
				publishQualityLevels(transcodedChunksArray...)
			}

			totalTranscodingMs := time.Since(startTranscoderTime).Milliseconds()
			if totalTranscodingMs > 1000 && totalTranscodingMs < 2000 {
				log.Printf("WARNING: Total transcoding time '%vms' approaching segment duration, consider less transcoding configurations.", totalTranscodingMs)
			} else if totalTranscodingMs > 2000 {
				log.Printf("DANGER: Total transcoding time '%vms' exceeds segment duration, stream will suffer interrupts, reduce or remove transcoding configurations.", totalTranscodingMs)
			}
		}

		if (segmentId-1)%10 == 0 {
			go screengrabSegment(segment)
		}

		//For fastest join times we take the lowest quality level
		lastSegment = transcodedChunksArray[len(transcodedChunksArray)-1]
	}()
}

func screengrabSegment(segment []byte) {
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
	thumbnail, err = cmd.Output()

	if err != nil {
		log.Println("Error capturing screenshot:", err)
		log.Println("FFmpeg stderr:", stderrPipe.String())
		return
	}

	log.Println("Screenshot captured successfully.")
}

func resizeSegment(transcode Transcode, segment []byte) []byte {
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

func probeVideoInfo(segment []byte) (map[string]string, error) {
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

func checkFfmpegInstalled() {
	// Command to check for ffmpeg (replace with actual command if needed)
	cmd := exec.Command("ffmpeg", "-version")

	err := cmd.Run()
	if err != nil {
		// Handle ffmpeg not found error
		log.Println("Error: ffmpeg is not installed. Please install ffmpeg and try again.")
		return
	}

	// ffmpeg is available, continue with your application logic
	log.Println("ffmpeg is installed. Proceeding...")
}

func isBroadcasting() bool {
	return time.Since(lastRtmpSegment).Seconds() < 5
}
