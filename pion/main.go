package main

import (
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	//"time"

	"github.com/pions/webrtc"
	"github.com/pions/webrtc/examples/util"
)

func toByteArray(buf []int16, bytes []byte) (int, error) {
	if len(buf)*2 > len(bytes) {
		return 0, fmt.Errorf("invalid buffer sizes: buf=%d bytes=%d", len(buf), len(bytes))
	}

	bi := 0
	for i := 0; i < len(buf); i++ {
		binary.LittleEndian.PutUint16(bytes[bi:], uint16(buf[i]))
		bi += 2
	}
	return bi, nil
}

func main() {
	// Flags
	useStun := flag.Bool("use-stun", false, "A boolean flag whether to use STUN server.")
	flag.Parse()

	// Logger setup
	log.SetPrefix("LOG: ")
	log.SetFlags(log.Ldate | log.Lmicroseconds)
	log.Println("Started")

	var pion *Pion

	chReady := make(chan bool)
	chDone := make(chan bool)

	// Set up signaling
	sig := New("ws://0.0.0.0:8080/socket.io/?EIO=3&transport=websocket", "Server", false)
	sig.SetOnConnected(func(err error) {
		if err != nil {
			log.Fatal("sig: connection failed:", err)
		}
		log.Printf("signaling ready. ID=%s SenderID=%s\n", sig.GetID(), sig.GetSenderID())
		chReady <- true
	})
	sig.SetOnSignaled(func(s *Signal) {
		if s.Type == "description" {
			log.Println("received remote description")
			bytes, err := json.Marshal(s.Body)
			if err != nil {
				log.Fatal("sig: failed to marshal signal body:", err)
			}
			answer := webrtc.RTCSessionDescription{}
			err = json.Unmarshal(bytes, &answer)
			util.Check(err)
			err = pion.SetRemoteDescription(answer)
			util.Check(err)
			return
		}

		if s.Type == "candidate" {
			log.Println("received remote candidate")
			bytes, err := json.Marshal(s.Body)
			if err != nil {
				log.Fatal("sig: failed to marshal signal body:", err)
			}
			candi := RemoteCandidate{}
			err = json.Unmarshal(bytes, &candi)
			util.Check(err)
			err = pion.AddIceCandidate(candi)
			util.Check(err)
			return
		}
	})

	// Connect to signaling server
	sig.Connect()
	defer sig.Disconnect()

	done := false
	for !done {
		select {
		case <-chReady:
			// Setup Pion-WebRTC
			pion = NewPion(*useStun)
			offer, err := pion.CreateOffer()
			util.Check(err)

			sm := &Signal{
				Type: "description",
				To:   sig.GetSenderID(),
				Body: offer,
			}
			sig.Send(sm, func(err error) {
				log.Println("sig: failed to signal:", err)
			})

		case done = <-chDone:
		}
	}
}
