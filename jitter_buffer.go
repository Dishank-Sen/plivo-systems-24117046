package main

import (
	"sync"
	"time"
)

const (
	MaxBufferSize = 128 // Must be power of 2 for efficient modulo
)

type BufferedFrame struct {
	Frame       *MediaPacket
	ArrivalTime time.Time
	Received    bool
	Played      bool
}

type JitterBuffer struct {
	mu          sync.Mutex
	buffer      [MaxBufferSize]BufferedFrame
	nextPlaySeq uint32
	stats       Stats
}

type Stats struct {
	FramesReceived    uint64
	FramesMissed      uint64
	DuplicatesDropped uint64
	FECRecoveries     uint64
	LateArrivals      uint64
}

func NewJitterBuffer(startSeq uint32) *JitterBuffer {
	return &JitterBuffer{
		nextPlaySeq: startSeq,
	}
}

// Insert incoming packet (called by receive goroutine)
func (jb *JitterBuffer) Insert(pkt *MediaPacket) {
	jb.mu.Lock()
	defer jb.mu.Unlock()

	idx := pkt.Seq % MaxBufferSize
	slot := &jb.buffer[idx]

	// Deduplication - count both primary and FEC copies
	if slot.Received && slot.Frame.Seq == pkt.Seq {
		jb.stats.DuplicatesDropped++
		// Still update if this is an FEC copy and we haven't played it yet
		if !slot.Played {
			// Keep the frame, might be useful for recovery
		}
		return
	}

	// Store frame
	slot.Frame = pkt
	slot.ArrivalTime = time.Now()
	slot.Received = true
	slot.Played = false
	jb.stats.FramesReceived++
}

// Peek at next frame without consuming it
func (jb *JitterBuffer) PeekNextFrame() (*MediaPacket, bool) {
	jb.mu.Lock()
	defer jb.mu.Unlock()

	idx := jb.nextPlaySeq % MaxBufferSize
	slot := &jb.buffer[idx]

	if !slot.Received || slot.Frame.Seq != jb.nextPlaySeq || slot.Played {
		return nil, false
	}

	return slot.Frame, true
}

// Consume the next frame (must call after Peek succeeds)
func (jb *JitterBuffer) ConsumeNextFrame() {
	jb.mu.Lock()
	defer jb.mu.Unlock()

	idx := jb.nextPlaySeq % MaxBufferSize
	slot := &jb.buffer[idx]
	slot.Played = true
	jb.nextPlaySeq++
}

// Mark the next frame as missed and advance
func (jb *JitterBuffer) MarkNextFrameMissed() {
	jb.mu.Lock()
	defer jb.mu.Unlock()

	jb.stats.FramesMissed++
	jb.nextPlaySeq++
}

// Get next frame for playout (called by playout goroutine)
func (jb *JitterBuffer) GetNextFrame() (*MediaPacket, bool) {
	jb.mu.Lock()
	defer jb.mu.Unlock()

	idx := jb.nextPlaySeq % MaxBufferSize
	slot := &jb.buffer[idx]

	if !slot.Received || slot.Frame.Seq != jb.nextPlaySeq {
		// Frame missing or out of sequence
		jb.stats.FramesMissed++
		jb.nextPlaySeq++
		return nil, false
	}

	if slot.Played {
		// Already played (shouldn't happen)
		jb.nextPlaySeq++
		return nil, false
	}

	// Mark as played and advance
	slot.Played = true
	frame := slot.Frame
	jb.nextPlaySeq++

	return frame, true
}

// Attempt FEC recovery for missing frame
func (jb *JitterBuffer) TryFECRecovery(seq uint32) *MediaPacket {
	jb.mu.Lock()
	defer jb.mu.Unlock()

	// Look for FEC copy of this sequence
	// In simple 2x FEC, we might have received the duplicate
	idx := seq % MaxBufferSize
	slot := &jb.buffer[idx]

	if slot.Received && slot.Frame.Seq == seq && !slot.Played {
		jb.stats.FECRecoveries++
		slot.Played = true
		return slot.Frame
	}

	return nil
}

func (jb *JitterBuffer) GetStats() Stats {
	jb.mu.Lock()
	defer jb.mu.Unlock()
	return jb.stats
}
