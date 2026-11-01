TARGET ?= m5stick-c
TINYGO ?= tinygo

.PHONY: test bench examples

test:
	go test ./...

bench:
	go test -run '^$$' -bench . -benchmem ./...

examples:
	$(TINYGO) build -target=$(TARGET) -size short -o /tmp/m5unit-gps-read.bin ./examples/read
