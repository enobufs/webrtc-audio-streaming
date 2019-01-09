package main

// API: https://godoc.org/github.com/gorilla/websocket

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"regexp"
	"sync"
	//"time"

	"github.com/gorilla/websocket"
)

var addr = flag.String("addr", "localhost:8080", "http service address")

// Socket.io message types
const (
	SigOpenT      = "0"
	SigCloseT     = "1"
	SigPingT      = "2"
	SigPongT      = "3"
	SigMsgT       = "4"
	SigEmptyMsgT  = "40"
	SigCommonMsgT = "42"
	SigAckT       = "43"
)

// Signalint State
const (
	SigOffline = iota + 1
	SigConnecting
	SigOnline
)

type sigBase struct {
	MsgID int64 `json:"msgId"`
}

type sigSyn struct {
	sigBase
	Name     string `json:"name"`
	IsSender bool   `json:"isSender"`
}

type sigAckBase struct {
	sigBase
	Success bool   `json:"success"`
	Reason  string `json:"reason"`
}

type sigSynAck struct {
	sigAckBase
	SenderID string `json:"senderId"`
}

type sigSigAck struct {
	sigAckBase
}

// Signal is passed Send method by users.
type Signal struct {
	sigBase
	Type string      `json:"type"`
	To   string      `json:"to"`
	From string      `json:"from"` // filled by server
	Body interface{} `json:"body"`
}

type openMsg struct {
	SID          string        `json:"sid"`
	Upgrades     []interface{} `json:"upgrades"`
	PingInterval int           `json:"pingInterval"`
	PingTimeout  int           `json:"pingTimeout"`
}

func parseSynAck(ack interface{}) (*sigSynAck, error) {
	v, err := json.Marshal(ack)
	if err != nil {
		return nil, err
	}
	parsed := &sigSynAck{}
	err = json.Unmarshal(v, &parsed)
	if err != nil {
		return nil, err
	}
	return parsed, nil
}

func parseSigAck(ack interface{}) (*sigSigAck, error) {
	v, err := json.Marshal(ack)
	if err != nil {
		return nil, err
	}
	parsed := &sigSigAck{}
	err = json.Unmarshal(v, &parsed)
	if err != nil {
		return nil, err
	}
	return parsed, nil
}

func parseSignal(s interface{}) (*Signal, error) {
	v, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	parsed := &Signal{}
	err = json.Unmarshal(v, &parsed)
	if err != nil {
		return nil, err
	}
	return parsed, nil
}

// ConnectCallback ...
type ConnectCallback func(error)

// SignaledCallback ...
type SignaledCallback func(s *Signal)

// SendCallback ...
type SendCallback func(error)

// Signaling context
type Signaling struct {
	url         string
	name        string
	id          string
	isSender    bool
	senderID    string
	c           *websocket.Conn
	lastMsgID   int64
	onConnected ConnectCallback
	onSignaled  SignaledCallback
	sendCbs     sync.Map
}

// New instantiate Signaling
func New(url string, dispName string, isSender bool) *Signaling {
	return &Signaling{
		url:      url,
		name:     dispName,
		isSender: isSender,
		c:        nil,
	}
}

// SetOnConnected ...
func (sig *Signaling) SetOnConnected(cb ConnectCallback) {
	sig.onConnected = cb
}

// SetOnSignaled ...
func (sig *Signaling) SetOnSignaled(cb SignaledCallback) {
	sig.onSignaled = cb
}

// GetID returnes a valid ID of this signaling endpoint. Valid ID
// is returned after connection to server is established.
func (sig *Signaling) GetID() string {
	return sig.id
}

// GetSenderID returns senders ID obtained during SYN transaction.
func (sig *Signaling) GetSenderID() string {
	return sig.senderID
}

// Connect to server
func (sig *Signaling) Connect() error {
	if sig.onConnected == nil {
		log.Fatal("ConnectedCallback is not set")
	}
	if sig.onSignaled == nil {
		log.Fatal("SignaledCallback is not set")
	}
	c, _, err := websocket.DefaultDialer.Dial(sig.url, nil)
	if err != nil {
		log.Fatal("failed to connect to server:", err)
	}

	sig.c = c

	re, _ := regexp.Compile("^([0-9]*)(.*)$")

	go func() {
		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				log.Println("read error:", err)
				break
			}
			subs := re.FindStringSubmatch(string(message))

			switch subs[1] {
			case SigOpenT:
				omsg := openMsg{}
				err = json.Unmarshal([]byte(subs[2]), &omsg)
				if err != nil {
					log.Fatal("error:", err)
				}
				/*
					log.Printf("sid=%s pingInterval=%d pingTimeout=%d\n",
						omsg.SID,
						omsg.PingInterval,
						omsg.PingTimeout)
				*/
				sig.id = omsg.SID

				// TODO: Start ping timer

				// Send sigSyn
				log.Println("sending sigSyn")
				syn := &sigSyn{Name: sig.name, IsSender: sig.isSender}
				sig.Send(syn, func(err error) {
					sig.onConnected(err) // connection complete
				})

			case SigCloseT:
				log.Println("don't know what to do yet")
			case SigPingT:
				log.Println("don't know what to do yet")
			case SigPongT:
				log.Println("don't know what to do yet")
			case SigMsgT:
				log.Println("don't know what to do yet")
			case SigEmptyMsgT:
				// Do nothing
			case SigCommonMsgT:
				args := []interface{}{}
				json.Unmarshal([]byte(subs[2]), &args)
				evID, ok := args[0].(string)
				if !ok {
					panic(errors.New("failed to parse args[0]"))
				}

				// args[1] contains the event object of type interface{}

				if evID == "syn-ack" {
					ack, err := parseSynAck(args[1])
					if err != nil {
						panic(err)
					}

					sig.senderID = ack.SenderID

					cb, ok := sig.sendCbs.Load(ack.MsgID)
					if !ok {
						panic(errors.New("SendCallback is missing")) // must be a bug!
					}
					sig.sendCbs.Delete(ack.MsgID)

					if ack.Success {
						cb.(SendCallback)(nil)
					} else {
						cb.(SendCallback)(errors.New("syn failed: " + ack.Reason))
					}
					break
				}

				if evID == "sig-ack" {
					ack, err := parseSigAck(args[1])
					if err != nil {
						panic(err)
					}

					cb, ok := sig.sendCbs.Load(ack.MsgID)
					if !ok {
						panic(errors.New("SendCallback is missing")) // must be a bug!
					}
					sig.sendCbs.Delete(ack.MsgID)

					if ack.Success {
						cb.(SendCallback)(nil)
					} else {
						cb.(SendCallback)(errors.New("sig failed: " + ack.Reason))
					}
					break
				}

				if evID == "sig" {
					s, err := parseSignal(args[1])
					if err != nil {
						panic(err)
					}

					if sig.onSignaled == nil {
						panic(errors.New("SignaledCallback is not set"))
					}
					sig.onSignaled(s)
					break
				}

				log.Printf("Unknown evID: %s\n", evID)

			case SigAckT:
				log.Println("don't know what to do yet")
			default:
				log.Println("unhandled event:", subs[1])
			}
		}
	}()

	return nil
}

// Disconnect from server
func (sig *Signaling) Disconnect() {
	// TODO: graceful shutdown
	sig.c.Close()
}

// Send ...
func (sig *Signaling) Send(msg interface{}, cb SendCallback) {
	switch msg.(type) {
	case *sigSyn:
		obj, _ := msg.(*sigSyn)
		sig.lastMsgID++
		obj.MsgID = sig.lastMsgID
		bytes, err := json.Marshal(obj)
		if err != nil {
			cb(err)
			break
		}
		str := []byte(fmt.Sprintf(`%s["syn",%s]`, SigCommonMsgT, string(bytes)))
		err = sig.c.WriteMessage(websocket.TextMessage, str)
		if err != nil {
			cb(err)
			break
		}
		sig.sendCbs.Store(sig.lastMsgID, cb)

	case *Signal:
		obj, _ := msg.(*Signal)
		sig.lastMsgID++
		obj.MsgID = sig.lastMsgID
		bytes, err := json.Marshal(obj)
		if err != nil {
			cb(err)
			break
		}
		str := []byte(fmt.Sprintf(`%s["sig",%s]`, SigCommonMsgT, string(bytes)))
		err = sig.c.WriteMessage(websocket.TextMessage, str)
		if err != nil {
			cb(err)
			break
		}
		sig.sendCbs.Store(sig.lastMsgID, cb)

	default:
		cb(errors.New("Unknown message to send"))
	}
}

/*
func main() {
	log.SetFlags(0)
	var sig1, sig2 *Signaling
	chReady := make(chan bool)
	chDone := make(chan bool)

	sig1 = New("ws://localhost:8080/socket.io/?EIO=3&transport=websocket", "Server", true)
	defer sig1.Disconnect()
	sig1.SetOnConnected(func(err error) {
		if err != nil {
			log.Fatal("sig1: connection failed:", err)
		}
		fmt.Println("sig1: connected...")

		sig2.Connect()
	})
	sig1.SetOnSignaled(func(s *Signal) {
		fmt.Printf("sig1: received: %+v\n", s)

		// sig1 sends "good-bye" to sig2
		sm := &Signal{
			Type: "greeting",
			To:   sig2.GetID(),
			Body: "good-bye",
		}
		sig1.Send(sm, func(err error) {
			fmt.Println("sig1: failed to signal:", err)
		})
	})

	sig2 = New("ws://localhost:8080/socket.io/?EIO=3&transport=websocket", "Server", false)
	sig2.SetOnConnected(func(err error) {
		if err != nil {
			log.Fatal("sig2: connection failed:", err)
		}
		fmt.Println("sig2: connected...")
		chReady <- true
	})
	defer sig2.Disconnect()
	sig2.SetOnSignaled(func(s *Signal) {
		fmt.Printf("sig2: received: %+v\n", s)

		chDone <- true
	})

	// Connect to signaling server
	sig1.Connect()

	done := false

	for !done {
		select {
		case <-chReady:
			// sig2 sends "hello" to sig1
			sm := &Signal{
				Type: "greeting",
				To:   sig1.GetID(),
				Body: "hello",
			}
			sig2.Send(sm, func(err error) {
				fmt.Println("sig2: failed to signal:", err)
			})
		case done = <-chDone:
		}
	}
}
*/
