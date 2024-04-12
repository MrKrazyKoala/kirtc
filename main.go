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

    log.Println("Starting Version:", version)

    interrupt := make(chan os.Signal, 1)
    signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)

    var c *websocket.Conn
    for c == nil {
        c = connectAndSend(*addr)
        if c == nil {
            log.Println("Trying to reconnect...")
            time.Sleep(5 * time.Second)
        }
    }
    defer c.Close()

    done := make(chan struct{})

    ticker := time.NewTicker(5 * time.Second)
    defer ticker.Stop()
    go func() {
        for {
            select {
            case <-ticker.C:
                log.Println("Sending heartbeat message")
                heartbeatMsg := SignalMessage{Type: "heartbeat", Data: "ping"}
                msg, err := json.Marshal(heartbeatMsg)
                if err != nil {
                    log.Println("Failed to marshal heartbeat message:", err)
                    return
                }
                if err := c.WriteMessage(websocket.TextMessage, msg); err != nil {
                    log.Println("Failed to send heartbeat message:", err)
                    return
                }
            case <-interrupt:
                log.Println("Interrupted, stopping heartbeat")
                ticker.Stop()
                return
            }
        }
    }()

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
        }
    }()

    for {
        select {
        case <-done:
            return
        case <-interrupt:
            log.Println("Interrupted")
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
