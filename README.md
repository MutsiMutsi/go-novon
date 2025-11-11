<p align="center">
  <img src="https://github.com/MutsiMutsi/novon/blob/main/images/card.png" width="480" title="">
</p>

# go-novon
![Go](https://pkg.go.dev/badge/github.com/mutsimutsi/nkn-sdk-go)
![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/mutsimutsi/go-novon)
![GitHub Tag](https://img.shields.io/github/v/tag/mutsimutsi/go-novon)
[![Go](https://github.com/MutsiMutsi/go-novon/actions/workflows/go.yml/badge.svg)](https://github.com/MutsiMutsi/go-novon/actions/workflows/go.yml)
[![License](https://img.shields.io/badge/License-MIT-yellow.svg)](https://choosealicense.com/licenses/mit/)

A decentralised video streaming host client to stream RTMP to novon

# Prerequisites

- ffmpeg: https://ffmpeg.org/
- Golang: https://go.dev/ (version 1.21 or above)

# Building from source

1) install the latest go
2) build the app: ```go build -o ./gonovon ./cmd/cli```
3) run ```./gonovon```

# Video bitrate, codecs, encoding configuration

Generally the same as all major streaming platforms, stick to h264 codecs for compatibility.
Set your keyframe to 2s for a good balance between fast delivery and efficiency.
High quality fast moving streams of 1080p 60hz should aim for a 6000kbps video bitrate.

Generally lower bitrates provide faster delivery, and allow for more viewers, lower the bitrate if buffering is an issue, or if your source media does not require these high bitrate for a good representation to improve viewer experience.



# Setup

First you will want to install OBS studio for your platform (https://obsproject.com/download) or any other RTMP capable streaming software, such as VLC, or even FFMPEG directly from the commandline.

For the sake of ease of use we will go with OBS for this tutorial.

Then download the GoNovon .EXE from the releases (or build it yourself using the instructions above)

Run the binary (.exe) and hit "Start GoNovon"
<img width="1010" height="741" alt="Start Novon" src="https://github.com/user-attachments/assets/8fcdb8eb-3a87-4d2e-942d-d7a2ef7d453b" />

Once done proceed with the following settings.

- Settings -> Stream (shown in above screenshot)
  - Service: Custom
  - Server: the location of the running gonovon application, if its on the same machine as your OBS application this would be "http://127.0.0.1" or "http://localhost/"
  - Stream Key: "anything" (this is not displayed on novon.tv but is required and can be used to utilize more features MediaMTX has to offer
- Settings -> Output -> Streaming
  - Video Encoder: x264 or if you have a recent NVIDIA GPU: NVIDIA NVENC H.264
  - Keyframe interval: 2s (you have to go into the advanced menu to change this)
  - Refer to the previous paragraph for more info on bitrate, codecs, and encoding configuration.

Then as the last step click "Start Streaming" in the OBS UI, and you should be good to go!<img width="914" height="322" alt="OBS stream" src="https://github.com/user-attachments/assets/dd43c6e1-f7f5-4cd8-9b8e-f46266ea7569" />

# Novon Stream Customization

On first startup a file `config.json` will be created next to the gonovon application.

It contains the following fields:
- **seed** this is your wallet seed for the stream this determines your address and is where tips will arrive. Make sure you do not reuse this seed to also watch your own stream, you can't broadcast to yourself!
- **title** this is the title of your stream as shown on novon.tv
- **owner** if you have a main NKN wallet that you use normally, you can put the publickey here, this will make it so that you get admin rights when you view your own stream in chat.
- **transcoders** this is an array in which you can set transcoding resolutions and framerates, if you stream in 1080p60 native, you can for example set it to array with values `['720p60', '540p30']` then your stream will be transcoded automatically from 1080p60 to 720p60 and progressively from 720p60 to 540p30. Viewers then have the option to select the quality in which they want to view your stream, this however does take considerable processing power from the streamer machine, so be mindful of that.

You can also place a file called `panels.json` next to the gonovon executable, and in here define an array with strings of markdown, each array entry will be converted to a column under your stream profile.
For example I use the following configuration: 

```js
[
	"![](https://i.imgur.com/h7Qwkax.png)\nI'm Mutsi!\n\nCurrently im fulltime developing my game **Arcturus**.",
	"![](https://i.imgur.com/LsH9qtD.png)\nBe civil.",
	"![](https://i.imgur.com/BWJDIFu.png)\nYou can send me tips in the chat window."
]
```

To get the following profile:

<img width="1398" height="1057" alt="image" src="https://github.com/user-attachments/assets/4055a292-912a-41c5-bc54-3fbe3b5dfa8a" />


# Dependencies
- MediaMTX - [https://github.com/bluenviron/mediamtx/](https://github.com/bluenviron/mediamtx/) [MIT license]

  A fork of MediaMTX is encapsulated to host the RTMP server and mux the stream to MPEG-TS segments for delivery

- nkn-sdk-go - [https://github.com/nknorg/nkn-sdk-go](https://github.com/nknorg/nkn-sdk-go) [Apache-2.0 license]

  The nkn network is used to amplify and distribute your video stream by multicasting; minimizing bandwidth requirements for the host while being able to reach a large number of viewers (you DO NOT need to download the .ZIP from the nkn SDK github as all dependencies are included in the binary, you do however need to install FFMPEG seperatley)

