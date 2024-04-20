<p align="center">
  <img src="https://github.com/MutsiMutsi/novon/blob/main/images/card.png" width="480" title="">
</p>

# go-novon
A decentralised video streaming host client to stream RTMP to novon

# Prerequisites

- ffmpeg: https://ffmpeg.org/
- Golang: https://go.dev/ (version 1.21 or above)

# Building from source

1) install the latest go
2) build the app: ```go build```
3) run ```./gonovon```

# Video bitrate, codecs, encoding configuration

Generally the same as all major streaming platforms, stick to h264 codecs for compatibility.
Set your keyframe to 2s for a good balance between fast delivery and efficiency.
High quality fast moving streams of 1080p 60hz should aim for a 6000kbps video bitrate.

Generally lower bitrates provide faster delivery, and allow for more viewers, lower the bitrate if buffering is an issue, or if your source media does not require these high bitrate for a good representation to improve viewer experience.

# Dependencies
- MediaMTX - [https://github.com/bluenviron/mediamtx/](https://github.com/bluenviron/mediamtx/) [MIT license]

  A fork of MediaMTX is encapsulated to host the RTMP server and mux the stream to MPEG-TS segments for delivery

- nkn-sdk-go - [https://github.com/nknorg/nkn-sdk-go](https://github.com/nknorg/nkn-sdk-go) [Apache-2.0 license]

  The nkn network is used to amplify and distribute your video stream by multicasting; minimizing bandwidth requirements for the host while being able to reach a large number of viewers