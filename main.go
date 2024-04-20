package main

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
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

func main() {
	fmt.Println("Welcome to go-novon a golang client for RTMP streaming to novon")
	fmt.Println("")

	checkFfmpegInstalled()

	var err error
	config, err = NewConfig("./config.json")
	if err != nil {
		panic(err)
	}

	client = createClient()
	for i := 0; i < NUM_SUB_CLIENTS; i++ {
		<-client.OnConnect.C
	}

	fmt.Println("connected to NKN")
	fmt.Println("Your address", client.Address())

	viewers = NewViewers(30 * time.Second)
	viewers.StartCleanup(time.Second)
	defer viewers.Cleanup()

	s, ok := core.New(os.Args[1:], publishTSPart)
	if !ok {
		os.Exit(1)
	}

	announceStream()

	receiveMessages(client, viewers)

	s.Wait()
}

func Attack() {
	//SPAM ATTACK
	for i := 0; i < 1000; i++ {
		rngAddr, _ := nkn.RandomBytes(32)
		viewers.AddOrUpdateAddress(hex.EncodeToString(rngAddr))
	}
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

	return client
}

func receiveMessages(client *nkn.MultiClient, viewers *Viewers) {
	go func() {
		for {
			msg := <-client.OnMessage.C

			if len(msg.Data) == 4 && string(msg.Data[:]) == "ping" {
				isNew := viewers.AddOrUpdateAddress(msg.Src)
				if isNew {
					fmt.Println("viewer joined: ", msg.Src)
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
			} else if len(msg.Data) == 7 && string(msg.Data[:]) == "getrole" {
				role := ""
				if msg.Src == config.Owner {
					role = "owner"
				}
				go replyText(role, msg)
			} else {
				DecodeMessage(msg)
			}

		}
	}()
}

func announceStream() {
	go func() {
		for {
			client.Subscribe("", "novon", 100, config.Title, nil)
			time.Sleep(20 * 100 * time.Second)
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
	log.Println("SHOW TIME", len(segment))
	//Segment the data to max CHUNK_SIZE chunks
	chunks := ChunkByByteSizeWithMetadata(segment, CHUNK_SIZE, segmentId)
	segmentId++

	fmt.Println("Broadcasting -", "viewers:", len(viewerAddresses), "size:", len(segment), "chunks:", len(chunks))

	if len(viewerAddresses) > 0 {
		for _, chunk := range chunks {
			publish(chunk)
		}
	}

	if (segmentId-1)%10 == 0 {
		go screengrabSegment(segment)
	}

	lastSegment = chunks
}

func screengrabSegment(segment []byte) {
	// Output image file
	outputFile := "screenshot.jpg"
	width := "256"
	height := "144"

	// Command arguments for ffmpeg
	cmd := exec.Command("ffmpeg",
		"-i", "-", // read from stdin (pipe)
		"-vframes", "1",
		"-y",                                             // overwrite output file
		"-vf", fmt.Sprintf("scale=%s:%s", width, height), // resize filter
		outputFile)

	var stdinPipe, stderrPipe bytes.Buffer
	cmd.Stdin = &stdinPipe
	cmd.Stderr = &stderrPipe

	// Write MPEG-TS data to stdin pipe
	stdinPipe.Write(segment)

	err := cmd.Run()

	if err != nil {
		fmt.Println("Error capturing screenshot:", err)
		fmt.Println("FFmpeg stderr:", stderrPipe.String())
		return
	}

	// Read the temporary image file into memory
	thumbnail, err = os.ReadFile(outputFile)
	if err != nil {
		fmt.Println("Error reading temporary image file:", err)
		return
	}

	fmt.Println("Screenshot captured successfully:", outputFile)
}

func checkFfmpegInstalled() {
	// Command to check for ffmpeg (replace with actual command if needed)
	cmd := exec.Command("ffmpeg", "-version")

	err := cmd.Run()
	if err != nil {
		// Handle ffmpeg not found error
		fmt.Println("Error: ffmpeg is not installed. Please install ffmpeg and try again.")
		return
	}

	// ffmpeg is available, continue with your application logic
	fmt.Println("ffmpeg is installed. Proceeding...")
}
