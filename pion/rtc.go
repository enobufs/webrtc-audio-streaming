package main

// API: https://godoc.org/github.com/gorilla/websocket

import (
	"fmt"
	"log"

	"github.com/hajimehoshi/oto"
	"github.com/pion/rtp/codecs"
	"github.com/pion/webrtc/v2"

	"gopkg.in/hraban/opus.v2"
)

const (
	stunServer = "stun:stun.l.google.com:19302"
)

// Pion ...
type Pion struct {
	pc *webrtc.PeerConnection
}

// NewPion instantiates a new Pion
func NewPion(useStun bool) *Pion {
	// Setup the codecs you want to use.
	opusCodec := webrtc.NewRTPCodec(
		webrtc.RTPCodecTypeAudio,
		"opus",
		48000,
		2,
		"minptime=10;useinbandfec=1;stereo=1",
		webrtc.DefaultPayloadTypeOpus,
		&codecs.OpusPayloader{})

	mediaEngine := webrtc.MediaEngine{}
	mediaEngine.RegisterCodec(opusCodec)
	api := webrtc.NewAPI(webrtc.WithMediaEngine(mediaEngine))

	iceServers := []webrtc.ICEServer{}
	if useStun {
		log.Printf("Use STUN server at %s", stunServer)
		iceServer := webrtc.ICEServer{
			URLs: []string{stunServer},
		}
		iceServers = append(iceServers, iceServer)
	} else {
		log.Println("No STUN server")
	}

	// Prepare the configuration
	config := webrtc.Configuration{
		ICEServers: iceServers,
	}

	log.Println("Create PeerConnection")
	// Create a new PeerConnection
	pc, err := api.NewPeerConnection(config)
	checkError(err)

	// Allow us to receive 1 audio track
	if _, err = pc.AddTransceiver(webrtc.RTPCodecTypeAudio); err != nil {
		checkError(err)
	}

	// Set a handler for when a new remote track starts, this handler creates a gstreamer pipeline
	// for the given codec
	pc.OnTrack(func(track *webrtc.Track, rtpReceiver *webrtc.RTPReceiver) {
		codec := track.Codec()
		fmt.Printf("Track PayloadType: %d\n", track.PayloadType)
		fmt.Printf("Codec Name       : %s\n", codec.Name)
		fmt.Printf("Codec MimeType   : %v\n", codec.MimeType)
		fmt.Printf("Codec ClockRate  : %v\n", codec.ClockRate)
		fmt.Printf("Codec Channels   : %v\n", codec.Channels)
		fmt.Printf("Codec SDPFmtpLine: %v\n", codec.SDPFmtpLine)

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
			p, err := track.ReadRTP()
			checkError(err)

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
	pc.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		log.Printf("Connection State has changed %s \n", connectionState.String())
	})
	// OnSignalingStateChange sets an event handler which is invoked when the
	// peer connection's signaling state changes
	pc.OnSignalingStateChange(func(sigState webrtc.SignalingState) {
		log.Printf("Signaling State has changed %s \n", sigState.String())
	})

	return &Pion{pc: pc}
}

// CreateOffer creates an offer.
func (p *Pion) CreateOffer() (webrtc.SessionDescription, error) {
	desc, err := p.pc.CreateOffer(nil)
	if err != nil {
		return desc, err
	}

	log.Printf("rtc: local SDP: %s\n", desc.SDP)

	if err = p.pc.SetLocalDescription(desc); err != nil {
		return desc, err
	}
	return desc, nil
}

// SetRemoteDescription ....
func (p *Pion) SetRemoteDescription(desc webrtc.SessionDescription) error {
	log.Printf("rtc: remote SDP: %s\n", desc.SDP)
	return p.pc.SetRemoteDescription(desc)
}

// AddICECandidate ....
func (p *Pion) AddICECandidate(candidate webrtc.ICECandidateInit) error {
	log.Printf("rtc: remote candidate: %s\n", candidate.Candidate)
	return p.pc.AddICECandidate(candidate)
}
