package gps

import (
	"testing"
	"time"
)

func withChecksum(body string) []byte {
	sum := 0
	for i := 0; i < len(body); i++ {
		sum ^= int(body[i])
	}
	line := make([]byte, 0, len(body)+5)
	line = append(line, '$')
	line = append(line, body...)
	line = append(line, '*', hexChar(sum>>4), hexChar(sum&0x0F), '\r', '\n')
	return line
}

func TestRMC(t *testing.T) {
	line := withChecksum("GNRMC,123519,A,4807.038,N,01131.000,E,022.4,084.4,230394,003.1,W")
	var p Parser
	if !p.Feed(line) {
		t.Fatal("RMC not parsed")
	}
	f := p.Fix()
	if !f.Valid {
		t.Fatal("Valid = false")
	}
	if f.Latitude < 48.11 || f.Latitude > 48.12 {
		t.Fatalf("Latitude = %v", f.Latitude)
	}
	if f.Longitude < 11.51 || f.Longitude > 11.52 {
		t.Fatalf("Longitude = %v", f.Longitude)
	}
	if f.SpeedKnots != 22.4 || f.Course != 84.4 {
		t.Fatalf("speed/course = %v/%v", f.SpeedKnots, f.Course)
	}
	want := time.Date(1994, 3, 23, 12, 35, 19, 0, time.UTC)
	if !f.UTC.Equal(want) {
		t.Fatalf("UTC = %v, want %v", f.UTC, want)
	}
}

func TestGGA(t *testing.T) {
	line := withChecksum("GPGGA,123519,4807.038,N,01131.000,E,1,08,0.9,545.4,M,46.9,M,,")
	var p Parser
	if !p.Feed(line) {
		t.Fatal("GGA not parsed")
	}
	f := p.Fix()
	if f.Quality != 1 || f.Satellites != 8 || f.Altitude != 545.4 {
		t.Fatalf("unexpected GGA: %+v", f)
	}
	if !f.Valid {
		t.Fatal("Valid = false")
	}
}

func TestGSV(t *testing.T) {
	line := withChecksum("GPGSV,3,1,11,05,25,047,22,13,20,123,23,15,15,089,20,12,10,045,18")
	var p Parser
	if !p.Feed(line) {
		t.Fatal("GSV not parsed")
	}
	f := p.Fix()
	if f.SatsInView != 11 || f.BestSNR != 23 {
		t.Fatalf("GSV = view %d snr %d, want 11/23", f.SatsInView, f.BestSNR)
	}
	// A second cycle starts at message 1 and resets the best SNR.
	line2 := withChecksum("GPGSV,3,1,11,05,25,047,10")
	p.Feed(line2)
	if p.Fix().BestSNR != 10 {
		t.Fatalf("BestSNR = %d, want 10", p.Fix().BestSNR)
	}
}

func TestGSVMultiTalker(t *testing.T) {
	// GP (7), GL (2) and BD (1) are aggregated into one in-view count; the
	// best SNR is the maximum across constellations.
	lines := [][]byte{
		withChecksum("GPGSV,2,1,07,08,46,268,19,10,26,133,11,16,02,342,20,27,49,329,12,1"),
		withChecksum("GPGSV,2,2,07,28,33,053,25"),
		withChecksum("GLGSV,1,1,02,81,28,042,23,82,71,087,29"),
		withChecksum("BDGSV,1,1,01,20,,,27"),
	}
	var p Parser
	for _, l := range lines {
		if !p.Feed(l) {
			t.Fatalf("not parsed: %s", l)
		}
	}
	f := p.Fix()
	if f.SatsInView != 10 {
		t.Fatalf("SatsInView = %d, want 10", f.SatsInView)
	}
	if f.BestSNR != 29 {
		t.Fatalf("BestSNR = %d, want 29", f.BestSNR)
	}
	// A new GP cycle resets only GP: 4 + GL 2 + BD 1.
	p.Feed(withChecksum("GPGSV,1,1,04,08,46,268,15"))
	if f := p.Fix(); f.SatsInView != 7 || f.BestSNR != 29 {
		t.Fatalf("after GP reset = view %d snr %d, want 7/29", f.SatsInView, f.BestSNR)
	}
}

func TestTXT(t *testing.T) {
	line := withChecksum("GNTXT,01,01,02,ANTENNA OK")
	var p Parser
	if !p.Feed(line) {
		t.Fatal("TXT not parsed")
	}
	if got := p.Fix().Antenna; got != "OK" {
		t.Fatalf("Antenna = %q, want OK", got)
	}
}

func TestBadChecksum(t *testing.T) {
	line := withChecksum("GNRMC,123519,A,4807.038,N,01131.000,E,022.4,084.4,230394,003.1,W")
	line[len(line)-3] = '0'
	var p Parser
	if p.Feed(line) {
		t.Fatal("bad checksum accepted")
	}
	if p.Fix().ChecksumErrors != 1 {
		t.Fatalf("ChecksumErrors = %d", p.Fix().ChecksumErrors)
	}
}

func TestNoFix(t *testing.T) {
	line := withChecksum("GNRMC,123519,V,4807.038,N,01131.000,E,022.4,084.4,230394,003.1,W")
	var p Parser
	p.Feed(line)
	if p.Fix().Valid {
		t.Fatal("Valid = true for status V")
	}
}

func TestSetWorkMode(t *testing.T) {
	fw := &fakeWriter{}
	d := New(fw)
	if err := d.SetWorkMode(7); err != nil {
		t.Fatal(err)
	}
	want := "$PCAS04,7*1E\r\n"
	if string(fw.buf) != want {
		t.Fatalf("SetWorkMode = %q, want %q", fw.buf, want)
	}
}

func TestNoAllocPerSentence(t *testing.T) {
	line := withChecksum("GNRMC,123519,A,4807.038,N,01131.000,E,022.4,084.4,230394,003.1,W")
	var p Parser
	p.Feed(line)
	gsv := withChecksum("GPGSV,3,1,11,05,25,047,22,13,20,123,23,15,15,089,20,12,10,045,18")
	allocs := testing.AllocsPerRun(1000, func() {
		p.Feed(line)
		p.Feed(gsv)
	})
	if allocs != 0 {
		t.Fatalf("Feed allocates %v per sentence, want 0", allocs)
	}
}

func BenchmarkFeedRMC(b *testing.B) {
	line := withChecksum("GNRMC,123519,A,4807.038,N,01131.000,E,022.4,084.4,230394,003.1,W")
	var p Parser
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Feed(line)
	}
}

type fakeWriter struct{ buf []byte }

func (f *fakeWriter) Write(p []byte) (int, error) {
	f.buf = append(f.buf, p...)
	return len(p), nil
}
func (f *fakeWriter) Read(p []byte) (int, error) { return 0, nil }
