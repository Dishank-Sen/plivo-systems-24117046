package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"time"
)

func mainReceiver() {
	// Read environment variables
	t0Str := os.Getenv("T0")
	durationStr := os.Getenv("DURATION_S")
	delayMsStr := os.Getenv("DELAY_MS")

	t0, err := strconv.ParseFloat(t0Str, 64)
	if err != nil {
		log.Printf("Warning: T0 not set or invalid, using current time")
		t0 = float64(time.Now().Unix())
	}

	delayMs, err := strconv.ParseFloat(delayMsStr, 64)
	if err != nil {
		log.Printf("Warning: DELAY_MS not set or invalid, using 100ms")
		delayMs = 100.0
	}

	duration, err := strconv.ParseFloat(durationStr, 64)
	if err != nil {
		log.Printf("Warning: DURATION_S not set, running indefinitely")
		duration = 3600.0 // 1 hour default
	}

	t0Time := time.Unix(0, int64(t0*1e9))
	delayDuration := time.Duration(delayMs) * time.Millisecond

	log.Printf("Receiver starting: t0=%v, delay=%v, duration=%.1fs", t0Time, delayDuration, duration)

	// Setup sockets
	recvConn, err := net.ListenUDP("udp", &net.UDPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: 47002,
	})
	if err != nil {
		log.Fatal("bind 47002:", err)
	}
	defer recvConn.Close()

	// Create unconnected socket for sending to player
	playoutConn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1")})
	if err != nil {
		log.Fatal("create playout socket:", err)
	}
	defer playoutConn.Close()

	playerAddr := &net.UDPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: 47020,
	}

	// Create jitter buffer
	jb := NewJitterBuffer(0)

	// Goroutine 1: Receive packets
	go receiveLoop(recvConn, jb)

	// Small delay to let receiver start
	time.Sleep(100 * time.Millisecond)

	// Goroutine 2: Playout frames
	playoutLoop(playoutConn, jb, t0Time, delayDuration, duration, playerAddr)

	// Print final stats
	stats := jb.GetStats()
	log.Printf("Receiver stats: received=%d, missed=%d, duplicates=%d, fec_recoveries=%d",
		stats.FramesReceived, stats.FramesMissed, stats.DuplicatesDropped, stats.FECRecoveries)
	if stats.FramesReceived > 0 {
		log.Printf("Miss rate: %.2f%%, Expected frames: %d",
			float64(stats.FramesMissed)/float64(stats.FramesReceived+stats.FramesMissed)*100,
			stats.FramesReceived+stats.FramesMissed)
	}
}

func receiveLoop(conn *net.UDPConn, jb *JitterBuffer) {
	buf := make([]byte, 2048)
	for {
		n, _, err := conn.ReadFromUDP(buf)
		if err != nil {
			continue
		}

		pkt, err := UnmarshalMediaPacket(buf[:n])
		if err != nil {
			continue
		}

		jb.Insert(pkt)
	}
}

func playoutLoop(conn *net.UDPConn, jb *JitterBuffer, t0 time.Time, delay time.Duration, durationSec float64, playerAddr *net.UDPAddr) {
	frameInterval := 20 * time.Millisecond
	frameNum := uint32(0)
	endTime := t0.Add(time.Duration(durationSec * float64(time.Second)))

	// Wait until t0 to start
	waitUntil := t0
	now := time.Now()
	if waitUntil.After(now) {
		time.Sleep(waitUntil.Sub(now))
	}

	for time.Now().Before(endTime.Add(time.Second)) {
		// Calculate deadline for this frame
		deadline := t0.Add(delay).Add(time.Duration(frameNum) * frameInterval)

		// Sleep until slightly before deadline to allow time for sending
		now := time.Now()
		targetSendTime := deadline.Add(-5 * time.Millisecond) // Send 5ms before deadline
		if sleepDur := targetSendTime.Sub(now); sleepDur > 0 {
			time.Sleep(sleepDur)
		}

		// Try to get frame - peek multiple times in case it arrives late
		var frame *MediaPacket
		var ok bool
		maxRetries := 5
		for i := 0; i < maxRetries; i++ {
			frame, ok = jb.PeekNextFrame()
			if ok && frame != nil {
				break
			}
			// Frame not ready yet, wait a tiny bit for it to arrive
			if i < maxRetries-1 && time.Now().Before(deadline.Add(-1*time.Millisecond)) {
				time.Sleep(500 * time.Microsecond)
				continue
			}
			break
		}

		if ok && frame != nil {
			// Send to player in harness format
			hf := &HarnessFrame{
				Seq:     frame.Seq,
				Payload: frame.Payload,
			}
			sendTime := time.Now()
			n, err := conn.WriteToUDP(hf.Marshal(), playerAddr)
			if err != nil {
				log.Printf("Error sending to player: %v", err)
			}
			jb.ConsumeNextFrame()
			if frameNum < 5 {
				late := sendTime.After(deadline)
				log.Printf("Frame %d: sent %d bytes at %v, deadline %v, late=%v",
					frameNum, n, sendTime.Sub(t0), deadline.Sub(t0), late)
			}
		} else {
			// Frame missed
			jb.MarkNextFrameMissed()
		}

		frameNum++
	}

	fmt.Println("Receiver playout complete")
}
