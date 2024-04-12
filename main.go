package main

import (
	"encoding/json"
	"flag"
	"log"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
)

// Address of the signaling server as a command-line flag
var addr = flag.String("addr", "ncus.signal.kinnode.io:8080", "WebSocket service address for signaling")

// SignalMessage represents the JSON structure for signaling messages
type SignalMessage struct {
	Type string `json:"type"`
	Data string `json:"data"`
}

func main() {
	flag.Parse()
	log.SetFlags(0)

	// Setup interrupt handling to gracefully shutdown the application
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)

	// Parse the URL for the WebSocket connection
	u := url.URL{Scheme: "ws", Host: *addr, Path: "/ws"}
	log.Printf("connecting to %s", u.String())

	// Establish WebSocket connection
	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Fatal("dial:", err)
	}
	defer c.Close()

	done := make(chan struct{})

	// Goroutine to handle incoming WebSocket messages
	go func() {
		defer close(done)
		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				log.Println("read:", err)
				return
			}

			// Log received message and attempt to unmarshal into SignalMessage
			log.Printf("recv: %s", message)

			var msg SignalMessage
			err = json.Unmarshal(message, &msg)
			if err != nil {
				log.Println("Error parsing JSON message:", err)
				continue
			}

			// Add handling for different types of messages here
			// Example: if msg.Type == "offer" { ... }
		}
	}()

	// Main loop to handle interrupts and shutdown gracefully
	for {
		select {
		case <-done:
			return
		case <-interrupt:
			log.Println("interrupt")
			// Cleanly close the connection by sending a close message
			err := c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				log.Println("write close:", err)
				return
			}
			select {
			case <-done:
			case <-time.After(time.Second):
			}
			c.Close()
			return
		}
	}
}

