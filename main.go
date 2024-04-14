package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"image"
	"image/jpeg"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/andreykaipov/goobs"
	"github.com/andreykaipov/goobs/api/requests/general"
	"github.com/nfnt/resize"

	"github.com/fsnotify/fsnotify"
	"github.com/nknorg/nkn-sdk-go"

	_ "image/png"
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
var streamPath = ""

const SCREENSHOT_HOTKEY string = "OBSBasic.Screenshot"

func main() {
	fmt.Println("Welcome to go-novon a golang client for OBS streaming to novon")
	fmt.Println("")
	fmt.Println("Make sure you have websockets enabled in OBS under Tools -> WebSocket Server Settings")
	fmt.Println("If you have authentication enabled take note of the server password this will be required")
	fmt.Println("")
	fmt.Println("Make sure that OBS is configured for outputting HLS recordings")
	fmt.Println("In OBS go to Settings -> Output tab, set the Output Mode to Advanced")
	fmt.Println("Proceed to the Recording tab")
	fmt.Println("Recording Format: 'HLS (.m3u8 + ts)'")
	fmt.Println("Video Encoder: your preferred choice for 'x264'")
	fmt.Println("")
	fmt.Println("Scroll down to the Encoder Settings and set the following configuration")
	fmt.Println("Keyframe Interval:'1s'")
	fmt.Println("")
	fmt.Println("go-novon will automatically detect the recording path and start broadcasting to novon")
	fmt.Println("")
	fmt.Println("")
	fmt.Println("")

	var err error
	config, err = NewConfig("./config.json")
	if err != nil {
		panic(err)
	}

	obs, err := goobs.New("localhost:4455")
	if err != nil {
		for {
			if err.Error() == "websocket: close 4009: Authentication failed." {
				reader := bufio.NewReader(os.Stdin)
				fmt.Print("OBS WebSocket password: ")
				password, _ := reader.ReadString('\n')

				fmt.Println(password)
				obs, err = goobs.New("localhost:4455", goobs.WithPassword(strings.TrimSpace(password)))
				if err != nil {
					fmt.Println(err.Error())
				} else {
					break
				}
			} else {
				fmt.Println("could not find a running OBS instance")
				time.Sleep(time.Second)
				obs, err = goobs.New("localhost:4455")
				if err == nil {
					break
				}
			}
		}
	}

	defer obs.Disconnect()

	directoryResponse, _ := obs.Config.GetRecordDirectory()
	streamPath = directoryResponse.RecordDirectory

	fmt.Println("Stream path found: ", streamPath)

	client = createClient()
	for i := 0; i < NUM_SUB_CLIENTS; i++ {
		<-client.OnConnect.C
	}

	fmt.Println("connected to NKN")
	fmt.Println("Your address", client.Address())

	recordStatus, _ := obs.Record.GetRecordStatus()
	if !recordStatus.OutputActive {
		obs.Record.StartRecord()
		fmt.Println("OBS started recording")
		defer obs.Record.StopRecord()
	} else {
		fmt.Println("OBS is recording")
	}

	viewers = NewViewers(30 * time.Second)
	viewers.StartCleanup(time.Second)
	defer viewers.Cleanup()

	announceStream()

	receiveMessages(client, viewers)
	takeScreenshot(obs)

	//Attack()
	dedup(streamPath)
}

func Attack() {
	//SPAM ATTACK
	for i := 0; i < 1000; i++ {
		rngAddr, _ := nkn.RandomBytes(32)
		viewers.AddOrUpdateAddress(hex.EncodeToString(rngAddr))
	}
}

func processFiles(event fsnotify.Event) {
	if strings.HasSuffix(event.Name, ".png") {
		resizeAndCacheScreenshot(event.Name)
		return
	}
	if strings.HasSuffix(event.Name, ".ts") {
		b, err := os.ReadFile(event.Name)
		if err != nil {
			// panic(err)
			fmt.Println(err)
			return
		}

		//Segment the data to max CHUNK_SIZE chunks
		chunks := ChunkByByteSizeWithMetadata(b, CHUNK_SIZE, segmentId)
		segmentId++

		fmt.Println("Broadcasting video segment to: ", len(viewerAddresses), "viewers")

		if len(viewerAddresses) > 0 {
			for _, chunk := range chunks {
				publish(chunk)
			}
		}
		lastSegment = chunks

		err = os.Remove(event.Name)
		if err != nil {
			fmt.Println(err)
		}
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
						sendToClient(msg.Src, chunk)
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

func takeScreenshot(obsClient *goobs.Client) {
	go func() {
		for {
			screenshotHotkeyName := "OBSBasic.Screenshot"
			obsClient.General.TriggerHotkeyByName(&general.TriggerHotkeyByNameParams{
				HotkeyName: &screenshotHotkeyName,
			})
			time.Sleep(30 * time.Second)
		}
	}()
}

func resizeAndCacheScreenshot(path string) {
	go func() {
		imgBytes, err := os.ReadFile(path)
		if err != nil {
			return
		}
		image, _, err := image.Decode(bytes.NewReader(imgBytes))
		if err != nil {
			return
		}
		newImage := resize.Resize(256, 144, image, resize.NearestNeighbor)

		if newImage == nil {
			return
		}

		newImageBytes := new(bytes.Buffer)
		jpeg.Encode(newImageBytes, newImage, nil)

		os.Remove(path)

		thumbnail = newImageBytes.Bytes()
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
