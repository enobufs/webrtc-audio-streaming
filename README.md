# Audio Streaming with WebRTC</h1>

## What does this do?
This demonstrates an audio streaming using WebRTC between the two browsers.
Instead of using microphones, sender side browser reads a MP3 file, then
streams it out to the receiver using WebRTC, while the receiver plays the audio
stream in real-time.

```
+-----+           +-----------+
| MP3 |  +------->| Signaling |<---------+
+-----+  |        |(socket.io)|          |
   |     |        +-----------+          |
   v     v                               v
+-----------+                         +-----------+
|  Browser  |   WebRTC (OPUS/RTP)     |  Browser  |
|           +----------------+------->|           |--> Speaker ))
|  (Sender) |                |  .     | (Receiver)|
+-----------+                | {or}   +-----------+
                             |  .     +-----------+
                             |  .     |pion/webrtc|
                             +------->|           |--> Speaker ))
                                      | (Receiver)|
                                      +-----------+
```

## How to run?

Check out the repo, cd into the root folder, then:
```sh
npm install
npm start
```

### Browser to browser
HTTP server with signaling service (socket.io) should be running at URL: `http://0.0.0.0:8080`.
Open your browser at the URL, then follow the further instruction.

### Browser to pion/webrtc
Open your browser at the URL (same as above), then click "sender tab" page only.

cd into `pion` folder, then:
```sh
go build
./pion --use-stun
```
