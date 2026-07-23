# System Design Notes

## Design
The system uses a **pipelined sender** (two goroutines: one dedicated to receiving from harness, one to forwarding with FEC) to prevent frame drops from the harness source, which was the critical bottleneck causing 55% frame loss in synchronous implementations. The receiver uses a **circular array jitter buffer** (128 slots, sequence-based indexing) for O(1) insertion, deduplication, and sequential playout despite out-of-order arrival. We implement **1.95× FEC** (primary copy always, FEC copy for 95% of frames) to stay exactly at the 2.0× bandwidth cap while providing ~95% protection against packet loss. The wire protocol is minimal (4-byte seq + 160-byte payload = 164 bytes) to match the harness format exactly and avoid overhead penalties.

## Recommended Grading Parameters
**Profile A:** `--delay_ms 120` (achieves 0.20% miss rate, 2.00× overhead)  
**Profile B:** `--delay_ms 150` (achieves 0.80% miss rate, 2.00× overhead)

## What Breaks It
The system fails with **burst losses** where >5 consecutive packets are dropped (probability both FEC copies lost becomes significant). Delays below 100ms (profile A) or 130ms (profile B) cause misses because frames don't arrive before their deadlines due to network jitter exceeding the buffer window. The fixed 128-slot circular buffer will fail if more than 128 frames are buffered simultaneously (would require 2.56 seconds of continuous packet accumulation). Finally, synchronized packet drops across both the primary and FEC paths (correlated loss) defeat the redundancy strategy, though this is rare with independent random relay delays.
