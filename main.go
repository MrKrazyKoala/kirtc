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

// CameraInfo represents the JSON structure for camera identification
type CameraInfo struct {
    Serial string `json:"serial"`
    MAC    string `json:"mac"`
}

// sendCameraInfo sends the camera's serial number and MAC address to the server
func sendCameraInfo(c *websocket.Conn, serial, mac string) {
    cameraInfo := CameraInfo{
        Serial: serial,
        MAC: mac,
    }
    message, err := json.Marshal(cameraInfo)
    if err != nil {
        log.Println("Error marshalling camera info:", err)
        return
    }
    if err := c.WriteMessage(websocket.TextMessage, message); err != nil {
        log.Println("Error sending camera info:", err)
    }
}

// connectAndSend initializes the WebSocket connection and sends the camera info
func connectAndSend(addr string) *websocket.Conn {
    u := url.URL{Scheme: "ws", Host: addr, Path: "/ws"}
    log.Printf("Connecting to %s", u.String())

    c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
    if err != nil {
        log.Println("Dial failed:", err)
        return nil
    }

    sendCameraInfo(c, "123456789", "00:1A:2B:3C:4D:5E")
    return c
}

func main() {
    flag.Parse()
    log.SetFlags(0)

    // Setup interrupt handling to gracefully shutdown the application
    interrupt := make(chan os.Signal, 1)
    signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)

    // Establish WebSocket connection and handle reconnection if necessary
    var c *websocket.Conn
    for c == nil {
        c = connectAndSend(*addr)
        if c == nil {
            log.Println("Trying to reconnect...")
            time.Sleep(5 * time.Second) // wait before retrying
        }
    }
    defer c.Close()

    done := make(chan struct{})

    // Goroutine to handle incoming WebSocket messages
    go func() {
        defer close(done)
        for {
            _, message, err := c.ReadMessage()
            if err != nil {
                log.Println("Read error:", err)
                return
            }

            log.Printf("Received: %s", message)

            var msg SignalMessage
            if err := json.Unmarshal(message, &msg); err != nil {
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
            log.Println("Interrupted")
            // Cleanly close the connection by sending a close message
            if err := c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")); err != nil {
                log.Println("Write close error:", err)
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
