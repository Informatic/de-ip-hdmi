package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"os"
	"time"
)

const HeartbeatPort = 48689

type SenderHeartbeat struct {
	_               [27]byte
	SignalPresent   byte
	SignalWidth     uint16
	SignalHeight    uint16
	SignalFPS       uint16
	EncodedWidth    uint16
	EncodedHeight   uint16
	_               [2]byte
	Uptime          uint32
	_               [6]byte
	ReceiverPresent uint8
}

func BroadcastHeartbeat(ifname string, senderip string) {
	packet := []byte{
		0x54, 0x46, 0x36, 0x7a,
		0x60, 0x02, // Source (sender / receiver) 0x6002 / 0x6301
		0x00, 0x00, // Padding
		0x00, 0x00, // Heartbeat counter

		0x00, 0x03, 0x03, 0x01, 0x00, 0x26, 0x00, 0x00, 0x00, // Magic sequence
		0x00, 0x00, 0x00, 0x00, // Uptime
	}

	for {
		saddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", senderip, HeartbeatPort))
		if err != nil {
			log.Printf("Unable to resolve addr, %s", err.Error())
			time.Sleep(time.Second)
			continue
		}
		laddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", HeartbeatPort))
		if err != nil {
			log.Printf("Unable to resolve addr, %s", err.Error())
			time.Sleep(time.Second)
			continue
		}
		conn, err := net.DialUDP("udp", laddr, saddr)
		if err != nil {
			log.Printf("Unable to keep broadcasting the keepalives, %s", err.Error())
			time.Sleep(time.Second)
			continue
		}
		_, err = conn.Write(packet)
		if err != nil {
			log.Printf("Unable to keep broadcasting the keepalives, %s", err.Error())
			time.Sleep(time.Second)
			continue
		}
		conn.Close()
		time.Sleep(time.Second)
		log.Println("Heartbeat sent")
	}
}

var EncodedWidth uint16 = 0
var EncodedHeight uint16 = 0

var LastFrame = 0
var LastFrameTS time.Time

func ProcessHeartbeat(data []byte) {
	heartbeat := SenderHeartbeat{}
	buffer := bytes.NewBuffer(data)
	err := binary.Read(buffer, binary.BigEndian, &heartbeat)
	if err == nil {
		// Calculate effective framerate
		EncodedFramerate := 0
		if LastFrame > 0 {
			EncodedFramerate = (TotalFrames - LastFrame) * int(time.Now().Sub(LastFrameTS)/time.Millisecond)
		}
		LastFrame = TotalFrames
		LastFrameTS = time.Now()

		log.Printf("[signal present: %t] %dx%d@%.1f - %dx%d@%.1f",
			heartbeat.SignalPresent == 3,
			heartbeat.SignalWidth, heartbeat.SignalHeight,
			float32(heartbeat.SignalFPS)/10.0,
			heartbeat.EncodedWidth, heartbeat.EncodedHeight,
			float32(EncodedFramerate)/1000.0)

		if (EncodedWidth != 0 || EncodedHeight != 0) &&
			(heartbeat.EncodedWidth != EncodedWidth || heartbeat.EncodedHeight != EncodedHeight) {
			// FIXME dirty hack for gstreamer being unable to handle resolution
			// changes
			log.Println("Restarting due to format change")
			os.Exit(1)
		}

		EncodedWidth = heartbeat.EncodedWidth
		EncodedHeight = heartbeat.EncodedHeight
	}
}
