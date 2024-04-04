package main

import (
	"bufio"
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"log"
	"os"
	"runtime"
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
var lastSegment []byte
var thumbnail []byte
var title string = "Mutsi's all day adventure ⚔🛡"

var viewerAddresses []string

var segmentSendConfig = &nkn.MessageConfig{
	Unencrypted: true,
	NoReply:     true,
}
var streamPath = ""

const SCREENSHOT_HOTKEY string = "OBSBasic.Screenshot"

func main() {
	fmt.Println("Welcome to go-noice a golang client for OBS streaming to noice")
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
	fmt.Println("go-noice will automatically detect the recording path and start broadcasting to noice")
	fmt.Println("")
	fmt.Println("")

	fmt.Println("")

	//Try without password first
	obs, err := goobs.New("localhost:4455")
	if err != nil {
		for {
			reader := bufio.NewReader(os.Stdin)
			fmt.Print("OBS WebSocket password (leave empty if no password is used): ")
			password, _ := reader.ReadString('\n')

			fmt.Println(password)
			obs, err = goobs.New("localhost:4455", goobs.WithPassword(strings.TrimSpace(password)))
			if err != nil {
				fmt.Println(err.Error())
			} else {
				break
			}
		}
	}

	defer obs.Disconnect()

	recordStatus, _ := obs.Record.GetRecordStatus()
	if !recordStatus.OutputActive {
		obs.Record.StartRecord()
		fmt.Println("OBS started recording")
		defer obs.Record.StopRecord()
	} else {
		fmt.Println("OBS is recording")
	}
	directoryResponse, _ := obs.Config.GetRecordDirectory()
	streamPath = directoryResponse.RecordDirectory

	fmt.Println("Stream path found: ", streamPath)

	client = createClient()
	<-client.OnConnect.C

	viewers := NewViewers(30 * time.Second)
	viewers.StartCleanup(time.Second)
	defer viewers.Cleanup()

	announceStream()

	receiveMessages(client, viewers)
	takeScreenshot(obs)

	fmt.Println("connected to NKN")
	fmt.Println("Your address", client.Address())

	// Create new watcher.
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()

	// Start listening for events.
	go func() {
		prevName := ""
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Has(fsnotify.Write) && strings.HasSuffix(event.Name, ".png") {
					resizeAndCacheScreenshot(event.Name)
					continue
				}
				if event.Has(fsnotify.Create) && strings.HasSuffix(event.Name, ".ts") {
					name := prevName
					prevName = event.Name

					if len(name) == 0 {
						continue
					}

					b, err := os.ReadFile(name)
					if err != nil {
						// panic(err)
						fmt.Println(err)
						continue
					}

					viewerAddresses = viewers.GetAddresses()

					fmt.Println("Broadcasting video segment to: ", len(viewerAddresses), "viewers")

					if len(viewerAddresses) > 0 {
						_, err = client.Send(nkn.NewStringArray(viewers.GetAddresses()...), b, segmentSendConfig)
						if err != nil {
							panic(err)
						}
					}
					lastSegment = b

					err = os.Remove(name)
					if err != nil {
						fmt.Println(err)
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				fmt.Println("error:", err)
			}
		}
	}()

	// Add a path.
	err = watcher.Add(streamPath)
	if err != nil {
		log.Fatal(err)
	}

	reader := bufio.NewReader(os.Stdin)
	reader.ReadLine()

	runtime.Goexit()
	os.Exit(0)
}

func createClient() *nkn.MultiClient {
	account, _ := nkn.NewAccount(nil)
	client, _ := nkn.NewMultiClient(account, "", 4, false, &nkn.ClientConfig{
		ConnectRetries: 10,
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
					client.Send(nkn.NewStringArray(msg.Src), lastSegment, segmentSendConfig)
				}
			} else if len(msg.Data) == 9 && string(msg.Data[:]) == "thumbnail" {
				err := msg.Reply(thumbnail)
				if err != nil {
					log.Println(err)
				}
			} else if len(msg.Data) == 10 && string(msg.Data[:]) == "disconnect" {
				viewers.Remove(msg.Src)
			} else if len(msg.Data) == 9 && string(msg.Data[:]) == "viewcount" {
				err := msg.Reply([]byte(strconv.Itoa(len(viewerAddresses))))
				if err != nil {
					fmt.Println(err.Error())
				}
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

		//thumbnail = base64.StdEncoding.EncodeToString(newImageBytes.Bytes())
		thumbnail = newImageBytes.Bytes()
	}()
}

func announceStream() {
	go func() {
		for {
			client.Subscribe("", "noice", 100, title, nil)
			time.Sleep(20 * 100 * time.Second)
		}
	}()
}
