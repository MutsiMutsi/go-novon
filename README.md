# go-novon
A decentralised video streaming host client to stream from OBS to novon

# Prerequisites

OBS Studio: https://obsproject.com/download
Golang: https://go.dev/ (with version 1.17 or above recommended)

# OBS configuration

Open OBS Studio.
Navigate to Tools -> WebSocket Server Settings.

Ensure that the "Enable Websockets" checkbox is selected.

Authentication (Optional)

If your OBS WebSocket server requires authentication, note down the server password. This will be prompted during the go-novon application startup.

HLS Recording Output:

Open OBS Studio settings.
Go to the Output tab.
Set the Output Mode to Advanced.
Navigate to the Recording tab.
Set the Recording Format to HLS (.m3u8 + ts).
Choose your preferred Video Encoder (x264 variant required).

HLS Recording Encoder Settings:

Scroll down to the Encoder Settings section.
Set the Keyframe Interval to 1s.

# Start streaming!
Make sure OBS is running, setup your scene as normal, but you do not have to start streaming or recording.
The application will automatically detect the HLS recording path from OBS Studio and initiate the streaming process to the novon platform.
