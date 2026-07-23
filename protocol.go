package main

import (
	"encoding/binary"
	"fmt"
)

// Wire format for packets between sender and receiver
type MediaPacket struct {
	Seq     uint32    // Frame sequence number
	Payload [160]byte // Audio frame data
}

// Encode packet to wire format (network byte order)
func (p *MediaPacket) Marshal() []byte {
	buf := make([]byte, 4+160) // seq + payload (same as harness format!)
	binary.BigEndian.PutUint32(buf[0:4], p.Seq)
	copy(buf[4:], p.Payload[:])
	return buf
}

// Decode packet from wire format
func UnmarshalMediaPacket(data []byte) (*MediaPacket, error) {
	if len(data) < 164 {
		return nil, fmt.Errorf("packet too short: %d bytes", len(data))
	}
	p := &MediaPacket{
		Seq: binary.BigEndian.Uint32(data[0:4]),
	}
	copy(p.Payload[:], data[4:164])
	return p, nil
}

// Harness format (47010 input, 47020 output)
type HarnessFrame struct {
	Seq     uint32
	Payload [160]byte
}

func UnmarshalHarnessFrame(data []byte) (*HarnessFrame, error) {
	if len(data) < 164 {
		return nil, fmt.Errorf("frame too short: %d bytes", len(data))
	}
	f := &HarnessFrame{
		Seq: binary.BigEndian.Uint32(data[0:4]),
	}
	copy(f.Payload[:], data[4:164])
	return f, nil
}

func (f *HarnessFrame) Marshal() []byte {
	buf := make([]byte, 164)
	binary.BigEndian.PutUint32(buf[0:4], f.Seq)
	copy(buf[4:], f.Payload[:])
	return buf
}
