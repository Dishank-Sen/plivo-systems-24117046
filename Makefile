.PHONY: all clean test

all: sender receiver

sender: main.go protocol.go jitter_buffer.go sender.go receiver.go
	go build -o sender .

receiver: main.go protocol.go jitter_buffer.go sender.go receiver.go
	go build -o receiver .

clean:
	rm -f sender receiver

test:
	@echo "Building sender and receiver..."
	@$(MAKE) all
	@echo "Build complete. To test, run:"
	@echo "  cd /home/dishank/Documents/plivo/systems_handout"
	@echo "  python3 run.py --profile profiles/A.json --delay_ms 100 \\"
	@echo "    --sender_cmd /mnt/data/plivo-sde/sender \\"
	@echo "    --receiver_cmd /mnt/data/plivo-sde/receiver"
