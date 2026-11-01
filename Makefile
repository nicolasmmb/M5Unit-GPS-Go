TARGET ?= m5stick-c
TINYGO ?= tinygo

.PHONY: test bench bench-save bench-compare alloc footprint escape profile examples

COUNT ?= 8
BENCH ?= .
BENCHTIME ?= 1s

test:
	go test ./...

# Run every benchmark (real captured 1 Hz NMEA cycle) with alloc stats.
bench:
	COUNT=$(COUNT) BENCH=$(BENCH) BENCHTIME=$(BENCHTIME) ./scripts/bench.sh run

# Save a run: make bench-save NAME=before
bench-save:
	COUNT=$(COUNT) BENCH=$(BENCH) BENCHTIME=$(BENCHTIME) ./scripts/bench.sh save $(NAME)

# Compare two saved runs: make bench-compare FROM=before TO=after
bench-compare:
	./scripts/bench.sh compare $(FROM) $(TO)

# Allocation contract + RAM footprint tests.
alloc:
	./scripts/bench.sh alloc

footprint:
	go test -run TestFootprint -v

# Compiler heap-escape report.
escape:
	./scripts/bench.sh escape

profile:
	./scripts/bench.sh profile

examples:
	$(TINYGO) build -target=$(TARGET) -size short -o /tmp/m5unit-gps-read.bin ./examples/read

# Fail if the parser or device exceeds its RAM budget.
check: test alloc examples
