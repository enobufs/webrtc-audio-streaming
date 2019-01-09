package main

// API: https://godoc.org/github.com/gorilla/websocket

import (
	"fmt"
	"log"

	//"github.com/gorilla/websocket"
	"github.com/hajimehoshi/oto"
	"github.com/pions/webrtc"
	"github.com/pions/webrtc/examples/util"
	"github.com/pions/webrtc/pkg/ice"
	//"github.com/pions/webrtc/pkg/rtcp"

	"gopkg.in/hraban/opus.v2"
)

/* RemoteCandidate example (the one sent by Chrome):
{
	"candidate":"candidate:2186074647 1 udp 2113937151 192.168.86.206 49800 typ host generation 0 ufrag 4WgR network-cost 999","sdpMid":"audio",
	"sdpMLineIndex":0,
	"usernameFragment":"4WgR"
}
*/

// RemoteCandidate ...
type RemoteCandidate struct {
	Candidate        string `json:"candidate"`
	SdpMLineIndex    int    `json:"sdpMLineIndex"`
	UsernameFragment string `json:"usernameFragment"`
}

// Pion ...
type Pion struct {
	pc *webrtc.RTCPeerConnection
}

// NewPion instantiates a new Pion
func NewPion() *Pion {
	// Setup the codecs you want to use.
	// We'll use the default ones but you can also define your own
	//webrtc.RegisterDefaultCodecs()
	opusCodec := webrtc.NewRTCRtpOpusCodec(webrtc.DefaultPayloadTypeOpus, 48000, 2)
	webrtc.RegisterCodec(opusCodec)

	// Prepare the configuration
	config := webrtc.RTCConfiguration{
		IceServers: []webrtc.RTCIceServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	}

	log.Println("Create PeerConnection")
	// Create a new RTCPeerConnection
	pc, err := webrtc.New(config)
	util.Check(err)

	// Set a handler for when a new remote track starts, this handler creates a gstreamer pipeline
	// for the given codec
	pc.OnTrack(func(track *webrtc.RTCTrack) {
		codec := track.Codec
		fmt.Printf("Track PayloadType: %d\n", track.PayloadType)
		fmt.Printf("Codec Name       : %s\n", codec.Name)
		fmt.Printf("Codec MimeType   : %v\n", codec.MimeType)
		fmt.Printf("Codec ClockRate  : %v\n", codec.ClockRate)
		fmt.Printf("Codec Channels   : %v\n", codec.Channels)
		fmt.Printf("Codec SdpFmtpLine: %v\n", codec.SdpFmtpLine)

		sampleRate := int(codec.ClockRate)
		nChannels := int(codec.Channels) // 1:mono; 2:stereo

		player, err := oto.NewPlayer(sampleRate, nChannels, 2, 15360)
		if err != nil {
			panic(err)
		}
		defer player.Close()

		dec, err := opus.NewDecoder(sampleRate, nChannels)
		if err != nil {
			panic(err)
		}

		// Allocate PCM buffers at maximum size
		frameSizeMs := 60 // for max frameSize
		frameSize := int(float32(frameSizeMs) * float32(sampleRate) / 1000)
		pcm16 := make([]int16, frameSize*nChannels)
		pcm := make([]byte, len(pcm16)*2)

		fmt.Printf("sampleRate : %d\n", sampleRate)
		fmt.Printf("frameSizeMs: %d\n", frameSizeMs)
		fmt.Printf("frameSize  : %d\n", frameSize)
		fmt.Printf("pcm16 size : %d\n", len(pcm16))
		fmt.Printf("pcm   size : %d\n", len(pcm))

		for {
			p := <-track.Packets
			//nPkts++

			payloadSize := len(p.Payload)

			if p.Padding {
				fmt.Println("PADDING")
			}
			if p.Extension {
				fmt.Println("EXTENTSON")
			}
			if p.Marker {
				fmt.Println("MARKER")
			}
			if p.PayloadOffset != 12 {
				fmt.Printf("PAYLOADOFFSET: %d\n", p.PayloadOffset)
			}

			// Decode OPUS frame and output to pcm16 buffer.
			nSamples, err := dec.Decode(p.Payload, pcm16)
			if err != nil {
				fmt.Printf("Decode err  : %v\n", err)
				fmt.Printf(" - payload  : %d(%d)\n", payloadSize, p.PayloadOffset)
				fmt.Printf(" - padding  : %v\n", p.Padding)
				fmt.Printf(" - extension: %v\n", p.Extension)
				fmt.Printf(" - marker   : %v\n", p.Marker)
				fmt.Printf(" - seq      : %v\n", p.SequenceNumber)
				fmt.Println(err.Error())
				continue
			}

			// Convert 16-bit PCM into an byte array.
			if nSamples*nChannels > len(pcm16) {
				fmt.Printf("WOULD: index out of range: samples=%d channels=%d payloadSize=%d", nSamples, nChannels, payloadSize)
				//panic(fmt.Errorf("index out of range: samples=%d channels=%d payloadSize=%d",
				//nSamples, nChannels, payloadSize))
			}
			nBytes, err := toByteArray(pcm16[:nSamples*nChannels], pcm)
			if err != nil {
				fmt.Printf("toByteArray err: %v\n", err)
				fmt.Printf(" - payload  : %d(%d)\n", payloadSize, p.PayloadOffset)
				fmt.Printf(" - padding  : %v\n", p.Padding)
				fmt.Printf(" - extension: %v\n", p.Extension)
				fmt.Printf(" - marker   : %v\n", p.Marker)
				fmt.Printf(" - seq      : %v\n", p.SequenceNumber)
				fmt.Printf(" - pcm16    : %v\n", nSamples)
				panic(err)
			}

			// See: https://github.com/hajimehoshi/oto/blob/master/player.go
			// The format is as follows:
			//   [data]      = [sample 1] [sample 2] [sample 3] ...
			//   [sample *]  = [channel 1] ...
			//   [channel *] = [byte 1] [byte 2] ...
			// Byte ordering is little endian.
			_, err = player.Write(pcm[:nBytes])
			if err != nil {
				panic(err)
			}

			//fmt.Printf("frameSize=%d payload=%d(%d) pcm16=%d pcm=%d written=%d padding=%v ext=%v mrk=%v seq=%v\n", frameSize, payloadSize, p.PayloadOffset, n, len(pcm), written, p.Padding, p.Extension, p.Marker, p.SequenceNumber)
		}
	})

	// Set the handler for ICE connection state
	// This will notify you when the peer has connected/disconnected
	pc.OnICEConnectionStateChange(func(connectionState ice.ConnectionState) {
		log.Printf("Connection State has changed %s \n", connectionState.String())
	})
	// OnSignalingStateChange sets an event handler which is invoked when the
	// peer connection's signaling state changes
	pc.OnSignalingStateChange(func(sigState webrtc.RTCSignalingState) {
		log.Printf("Signaling State has changed %s \n", sigState.String())
	})

	return &Pion{pc: pc}
}

// CreateOffer creates an offer.
func (p *Pion) CreateOffer() (interface{}, error) {
	return p.pc.CreateOffer(nil)
}

// SetRemoteDescription ....
func (p *Pion) SetRemoteDescription(desc webrtc.RTCSessionDescription) error {
	return p.pc.SetRemoteDescription(desc)
}

// AddIceCandidate ....
// Pion's method takes only the `candidate` field which is not ideal...
// This should be fixed in pion in the future.
func (p *Pion) AddIceCandidate(candidate RemoteCandidate) error {
	log.Printf("rtc: remote candidate: %s\n", candidate.Candidate)
	return p.pc.AddIceCandidate(candidate.Candidate)
}
