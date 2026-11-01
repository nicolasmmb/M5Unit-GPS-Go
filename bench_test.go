package gps

import (
	"os"
	"strings"
	"testing"
	"unsafe"
)

// cycleLines returns one second of NMEA captured from the Unit GPS v1.1
// (GPS + GLONASS + BeiDou, 1 Hz). Benchmarks use it to measure realistic use
// instead of a single hand-written sentence.
func cycleLines(tb testing.TB) [][]byte {
	tb.Helper()
	raw, err := os.ReadFile("testdata/cycle.nmea")
	if err != nil {
		tb.Fatal(err)
	}
	var lines [][]byte
	for _, l := range strings.Split(string(raw), "\n") {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		lines = append(lines, []byte(l))
	}
	if len(lines) == 0 {
		tb.Fatal("empty NMEA cycle")
	}
	return lines
}

func sentenceOf(tb testing.TB, kind string) []byte {
	tb.Helper()
	for _, l := range cycleLines(tb) {
		if string(l[3:6]) == kind {
			return l
		}
	}
	tb.Fatalf("no %s sentence in the cycle", kind)
	return nil
}

func feedBench(b *testing.B, lines [][]byte) {
	var p Parser
	p.Feed(lines[0]) // warm up
	b.ReportAllocs()
	b.SetBytes(int64(totalBytes(lines)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, l := range lines {
			p.Feed(l)
		}
	}
}

func totalBytes(lines [][]byte) int {
	n := 0
	for _, l := range lines {
		n += len(l)
	}
	return n
}

func BenchmarkFeed(b *testing.B) {
	for _, kind := range []string{"GGA", "RMC", "GSV", "TXT"} {
		b.Run(kind, func(b *testing.B) {
			feedBench(b, [][]byte{sentenceOf(b, kind)})
		})
	}
}

// BenchmarkFeedCycle parses one full 1 Hz cycle, the realistic steady state.
func BenchmarkFeedCycle(b *testing.B) {
	feedBench(b, cycleLines(b))
}

func BenchmarkFix(b *testing.B) {
	var p Parser
	for _, l := range cycleLines(b) {
		p.Feed(l)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if !p.Fix().Valid {
			b.Fatal("lost fix")
		}
	}
}

// BenchmarkDeviceCycle measures the device pipeline (mutex + parse + snapshot)
// for one second of NMEA.
func BenchmarkDeviceCycle(b *testing.B) {
	lines := cycleLines(b)
	d := New(&discardWriter{})
	b.ReportAllocs()
	b.SetBytes(int64(totalBytes(lines)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, l := range lines {
			d.consume(l)
		}
	}
}

func BenchmarkDeviceFix(b *testing.B) {
	d := New(&discardWriter{})
	for _, l := range cycleLines(b) {
		d.consume(l)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if !d.Fix().Valid {
			b.Fatal("lost fix")
		}
	}
}

// TestNoAllocPerCycle is the realistic allocation contract: one second of
// NMEA (GGA, RMC, GSV x4, TXT, ...) must not touch the heap.
func TestNoAllocPerCycle(t *testing.T) {
	lines := cycleLines(t)
	var p Parser
	for _, l := range lines {
		p.Feed(l)
	}
	allocs := testing.AllocsPerRun(100, func() {
		for _, l := range lines {
			p.Feed(l)
		}
	})
	if allocs != 0 {
		t.Fatalf("Feed allocates %v per cycle, want 0", allocs)
	}
}

// TestAntennaAlloc checks that the repeated TXT sentence (sent every second by
// the module) does not allocate while the antenna state is unchanged.
func TestAntennaAlloc(t *testing.T) {
	txt := sentenceOf(t, "TXT")
	var p Parser
	p.Feed(txt)
	allocs := testing.AllocsPerRun(100, func() { p.Feed(txt) })
	if allocs != 0 {
		t.Fatalf("unchanged TXT allocates %v, want 0", allocs)
	}

	// A real change may allocate once, but only once.
	changed := withChecksum("GPTXT,01,01,01,ANTENNA OK")
	p.Feed(changed)
	allocs = testing.AllocsPerRun(100, func() { p.Feed(changed) })
	if allocs != 0 {
		t.Fatalf("stable TXT after change allocates %v, want 0", allocs)
	}
}

func TestSetWorkModeNoAlloc(t *testing.T) {
	d := New(&discardWriter{})
	allocs := testing.AllocsPerRun(100, func() { _ = d.SetWorkMode(7) })
	if allocs != 0 {
		t.Fatalf("SetWorkMode allocates %v, want 0", allocs)
	}
}

// TestFootprint documents the RAM cost of the parser and device. The values
// are for the host (64-bit); TinyGo on ESP32 is 32-bit and slightly smaller.
func TestFootprint(t *testing.T) {
	t.Logf("Fix=%d B  Parser=%d B  Device=%d B",
		unsafe.Sizeof(Fix{}), unsafe.Sizeof(Parser{}), unsafe.Sizeof(Device{}))
	if n := unsafe.Sizeof(Parser{}); n > 256 {
		t.Fatalf("Parser is %d B, budget 256 B", n)
	}
	if n := unsafe.Sizeof(Device{}); n > 384 {
		t.Fatalf("Device is %d B, budget 384 B", n)
	}
}

type discardWriter struct{}

func (discardWriter) Write(p []byte) (int, error) { return len(p), nil }
func (discardWriter) Read(p []byte) (int, error)  { return 0, nil }
