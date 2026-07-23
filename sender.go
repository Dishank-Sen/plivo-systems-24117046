package main

import (
	"log"
	"net"
	"os"
	"strconv"
	"time"
)

func mainSender() {
	// Read environment variables (optional, for logging)
	t0Str := os.Getenv("T0")
	durationStr := os.Getenv("DURATION_S")
	delayMsStr := os.Getenv("DELAY_MS")

	if t0Str != "" && durationStr != "" && delayMsStr != "" {
		log.Printf("Sender starting: T0=%s, DURATION_S=%s, DELAY_MS=%s", t0Str, durationStr, delayMsStr)
	}

	duration, _ := strconv.ParseFloat(durationStr, 64)
	var endTime time.Time
	if duration > 0 {
		endTime = time.Now().Add(time.Duration(duration) * time.Second)
	}

	// Receive from harness source
	sourceConn, err := net.ListenUDP("udp", &net.UDPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: 47010,
	})
	if err != nil {
		log.Fatal("bind 47010:", err)
	}
	defer sourceConn.Close()

	// Increase UDP receive buffer to prevent drops
	sourceConn.SetReadBuffer(1024 * 1024) // 1MB buffer

	// Send to relay
	relayAddr := &net.UDPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: 47001,
	}
	sendConn, err := net.DialUDP("udp", nil, relayAddr)
	if err != nil {
		log.Fatal("connect 47001:", err)
	}
	defer sendConn.Close()

	// Use channel for pipeline: receive -> send
	frameChan := make(chan *HarnessFrame, 100)
	frameCount := uint64(0)

	// Goroutine 1: Receive from harness (fast, non-blocking)
	go func() {
		buf := make([]byte, 2048)
		for {
			// Set short timeout
			sourceConn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))

			n, _, err := sourceConn.ReadFromUDP(buf)
			if err != nil {
				// Check if we should exit
				if !endTime.IsZero() && time.Now().After(endTime) {
					close(frameChan)
					return
				}
				continue
			}

			hf, err := UnmarshalHarnessFrame(buf[:n])
			if err != nil {
				log.Printf("Error unmarshaling harness frame: %v", err)
				continue
			}

			// Send to channel (non-blocking with timeout)
			select {
			case frameChan <- hf:
			case <-time.After(1 * time.Millisecond):
				log.Printf("Warning: frame channel full, dropped frame %d", hf.Seq)
			}
		}
	}()

	// Goroutine 2: Send to relay (main goroutine)
	for hf := range frameChan {
		// Create media packet
		pkt := &MediaPacket{
			Seq:     hf.Seq,
			Payload: hf.Payload,
		}

		packetBytes := pkt.Marshal()

		// Send primary copy
		_, err = sendConn.Write(packetBytes)
		if err != nil {
			log.Printf("Error sending primary packet: %v", err)
		}

		// Send FEC copy for most frames (to stay under 2.0× overhead)
		// With 2% loss rate, sending 1.95× copies should be sufficient
		if hf.Seq%20 != 0 { // Send FEC for 95% of frames
			_, err = sendConn.Write(packetBytes)
			if err != nil {
				log.Printf("Error sending FEC packet: %v", err)
			}
		}

		frameCount++
	}

	log.Printf("Sender complete: sent %d frames", frameCount)
}
