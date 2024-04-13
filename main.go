package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"net"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
)

var addr = flag.String("addr", "ncus.signal.kinnode.io:8080", "WebSocket service address for signaling")
var version = "0.0.003"

type SignalMessage struct {
	Type string `json:"type"`
	Data string `json:"data"`
}

type CameraInfo struct {
	Serial string `json:"serial"`
	MAC    string `json:"mac"`
}

func sendCameraInfo(c *websocket.Conn, serial, mac string) {
	cameraInfo := CameraInfo{
		Serial: serial,
		MAC:    mac,
	}
	message, err := json.Marshal(cameraInfo)
	if err != nil {
		log.Printf("Error marshalling camera info: %v", err)
		return
	}
	if err := c.WriteMessage(websocket.TextMessage, message); err != nil {
		log.Printf("Error sending camera info: %v", err)
	}
}

func connectAndSend(addr string) *websocket.Conn {
	u := url.URL{Scheme: "ws", Host: addr, Path: "/ws"}
	log.Printf("Connecting to %s", u.String())

	dialer := websocket.Dialer{
		HandshakeTimeout: 45 * time.Second,
		WriteBufferSize:  1024,
		ReadBufferSize:   1024,
		NetDialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			conn, err := net.Dial(network, addr)
			if err != nil {
				return nil, err
			}
			if tcpConn, ok := conn.(*net.TCPConn); ok {
				tcpConn.SetNoDelay(true) // Disable Nagle's algorithm for real-time communication.
			}
			return conn, nil
		},
	}

	// Attempt to connect with retry on failure.
	for {
		c, _, err := dialer.Dial(u.String(), nil)
		if err != nil {
			log.Printf("Dial failed: %v, retrying in 5 seconds...", err)
			time.Sleep(5 * time.Second)
			continue
		}
		log.Println("Connected successfully.")
		return c
	}
}

func setupCloseHandler(c *websocket.Conn) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-ch
		log.Println("Interrupt received, closing websocket connection")
		c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		os.Exit(0)
	}()
}

func main() {
	flag.Parse()
	log.Println("Starting Version:", version)

	c := connectAndSend(*addr)
	defer c.Close()

	sendCameraInfo(c, "123456789", "00:1A:2B:3C:4D:5E")

	setupCloseHandler(c)

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	go func() {
		for range ticker.C {
			log.Println("Sending heartbeat message")
			heartbeatMsg := SignalMessage{Type: "heartbeat", Data: "ping"}
			msg, err := json.Marshal(heartbeatMsg)
			if err != nil {
				log.Printf("Failed to marshal heartbeat message: %v", err)
				continue
			}
			if err := c.WriteMessage(websocket.TextMessage, msg); err != nil {
				log.Printf("Failed to send heartbeat message: %v", err)
				continue
			}
		}
	}()

	// Handle incoming messages
	for {
		_, message, err := c.ReadMessage()
		if err != nil {
			log.Printf("Read error: %v", err)
			break
		}
		log.Printf("Received: %s", message)
	}
}
