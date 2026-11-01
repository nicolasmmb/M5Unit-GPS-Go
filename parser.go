// Package gps parses NMEA 0183 from the M5Stack Unit GPS v1.1
// (CASIC AT6668 + MAX2659).
//
// The parser uses fixed buffers and performs no heap allocation per sentence.
package gps

import (
	"math"
	"time"
)

// Fix is the latest position and status decoded from the module.
type Fix struct {
	// Valid is true while the receiver reports an active fix.
	Valid bool

	Latitude  float64 // degrees, positive north
	Longitude float64 // degrees, positive east
	Altitude  float64 // metres

	Satellites int
	Quality    int // GGA fix quality (0 = no fix)

	SpeedKnots float64
	Course     float64 // degrees

	// UTC is the time reported by RMC. Zero until a fix is received.
	UTC time.Time

	// Antenna is the state reported by the module's TXT sentence.
	Antenna string

	Sentences      uint64 // valid NMEA sentences parsed
	ChecksumErrors uint64 // sentences with a bad checksum
}

// Parser decodes NMEA sentences into a Fix. It is not safe for concurrent use;
// Device serializes access.
type Parser struct {
	fix Fix
}

// Fix returns a copy of the current state.
func (p *Parser) Fix() Fix { return p.fix }

// Feed parses one NMEA sentence (with or without a trailing CR/LF).
// It returns true when the sentence had a valid checksum and was recognized.
func (p *Parser) Feed(line []byte) bool {
	line = trim(line)
	if len(line) < 7 || line[0] != '$' {
		return false
	}
	star := indexByte(line, '*')
	if star < 0 || star+2 >= len(line) {
		return false
	}
	sum := 0
	for _, c := range line[1:star] {
		sum ^= int(c)
	}
	if hexVal(line[star+1])<<4|hexVal(line[star+2]) != sum {
		p.fix.ChecksumErrors++
		return false
	}

	p.fix.Sentences++

	f0 := field(line, 0)
	if len(f0) < 4 {
		return false
	}
	typ := f0[len(f0)-3:]
	switch {
	case typeIs(typ, "GGA"):
		p.decodeGGA(line)
	case typeIs(typ, "RMC"):
		p.decodeRMC(line)
	case typeIs(typ, "TXT"):
		p.decodeTXT(line)
	}
	return true
}

func (p *Parser) decodeGGA(line []byte) {
	lat, okLat := parseCoord(field(line, 2), fieldIs(field(line, 3), 'S'))
	lon, okLon := parseCoord(field(line, 4), fieldIs(field(line, 6), 'W'))
	if okLat {
		p.fix.Latitude = lat
	}
	if okLon {
		p.fix.Longitude = lon
	}
	if q, ok := parseInt(field(line, 6)); ok {
		p.fix.Quality = q
		if q > 0 {
			p.fix.Valid = true
		}
	}
	if sats, ok := parseInt(field(line, 7)); ok {
		p.fix.Satellites = sats
	}
	if alt, ok := parseFloat(field(line, 9)); ok {
		p.fix.Altitude = alt
	}
}

func (p *Parser) decodeRMC(line []byte) {
	status := field(line, 2)
	active := fieldIs(status, 'A')
	p.fix.Valid = active
	if !active {
		return
	}

	lat, okLat := parseCoord(field(line, 3), fieldIs(field(line, 4), 'S'))
	lon, okLon := parseCoord(field(line, 5), fieldIs(field(line, 6), 'W'))
	if okLat {
		p.fix.Latitude = lat
	}
	if okLon {
		p.fix.Longitude = lon
	}
	if speed, ok := parseFloat(field(line, 7)); ok {
		p.fix.SpeedKnots = speed
	}
	if course, ok := parseFloat(field(line, 8)); ok {
		p.fix.Course = course
	}

	h, m, s, ns, okClock := parseClock(field(line, 1))
	year, month, day, okDate := parseDate(field(line, 9))
	if okClock && okDate {
		p.fix.UTC = time.Date(year, time.Month(month), day, h, m, s, ns, time.UTC)
	}
}

func (p *Parser) decodeTXT(line []byte) {
	f := field(line, 4)
	if len(f) < 7 || !typeIs(f[:7], "ANTENNA") {
		return
	}
	state := f[7:]
	if i := indexByte(state, '*'); i >= 0 {
		state = state[:i]
	}
	for len(state) > 0 && state[0] == ' ' {
		state = state[1:]
	}
	p.fix.Antenna = string(state) // rare sentence: only allocation in the parser
}

// --- helpers ---

func trim(line []byte) []byte {
	for len(line) > 0 {
		c := line[len(line)-1]
		if c == '\r' || c == '\n' || c == ' ' {
			line = line[:len(line)-1]
			continue
		}
		break
	}
	for len(line) > 0 && line[0] == ' ' {
		line = line[1:]
	}
	return line
}

func indexByte(b []byte, c byte) int {
	for i, v := range b {
		if v == c {
			return i
		}
	}
	return -1
}

func typeIs(b []byte, s string) bool {
	if len(b) != len(s) {
		return false
	}
	for i := range b {
		if b[i] != s[i] {
			return false
		}
	}
	return true
}

func fieldIs(b []byte, c byte) bool {
	return len(b) == 1 && b[0] == c
}

// field returns the idx-th comma-separated field without allocating.
func field(line []byte, idx int) []byte {
	start, n := 0, 0
	for i := 0; i <= len(line); i++ {
		if i == len(line) || line[i] == ',' {
			if n == idx {
				return line[start:i]
			}
			n++
			start = i + 1
		}
	}
	return nil
}

func hexVal(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	}
	return -1
}

func parseFloat(b []byte) (float64, bool) {
	if len(b) == 0 {
		return 0, false
	}
	neg := false
	i := 0
	if b[0] == '-' {
		neg = true
		i++
	} else if b[0] == '+' {
		i++
	}
	var v float64
	digits := 0
	for ; i < len(b) && b[i] >= '0' && b[i] <= '9'; i++ {
		v = v*10 + float64(b[i]-'0')
		digits++
	}
	if i < len(b) && b[i] == '.' {
		i++
		scale := 0.1
		for ; i < len(b) && b[i] >= '0' && b[i] <= '9'; i++ {
			v += float64(b[i]-'0') * scale
			scale *= 0.1
			digits++
		}
	}
	if digits == 0 || i != len(b) {
		return 0, false
	}
	if neg {
		v = -v
	}
	return v, true
}

func parseInt(b []byte) (int, bool) {
	if len(b) == 0 {
		return 0, false
	}
	v := 0
	for _, c := range b {
		if c < '0' || c > '9' {
			return 0, false
		}
		v = v*10 + int(c-'0')
	}
	return v, true
}

func parseCoord(b []byte, southWest bool) (float64, bool) {
	v, ok := parseFloat(b)
	if !ok {
		return 0, false
	}
	// NMEA uses ddmm.mmmm / dddmm.mmmm: the last two digits before the
	// decimal point are always minutes.
	deg := math.Floor(v / 100)
	dec := deg + (v-deg*100)/60
	if southWest {
		dec = -dec
	}
	return dec, true
}

func parseClock(b []byte) (h, m, s, ns int, ok bool) {
	if len(b) < 6 {
		return 0, 0, 0, 0, false
	}
	for i := 0; i < 6; i++ {
		if b[i] < '0' || b[i] > '9' {
			return 0, 0, 0, 0, false
		}
	}
	h = int(b[0]-'0')*10 + int(b[1]-'0')
	m = int(b[2]-'0')*10 + int(b[3]-'0')
	s = int(b[4]-'0')*10 + int(b[5]-'0')
	if h > 23 || m > 59 || s > 60 {
		return 0, 0, 0, 0, false
	}
	if len(b) > 6 && b[6] == '.' {
		scale := 100000000 // 0.1s
		for i := 7; i < len(b) && i < 10; i++ {
			if b[i] < '0' || b[i] > '9' {
				return 0, 0, 0, 0, false
			}
			ns += int(b[i]-'0') * scale
			scale /= 10
		}
	}
	return h, m, s, ns, true
}

func parseDate(b []byte) (year, month, day int, ok bool) {
	if len(b) < 6 {
		return 0, 0, 0, false
	}
	for i := 0; i < 6; i++ {
		if b[i] < '0' || b[i] > '9' {
			return 0, 0, 0, false
		}
	}
	day = int(b[0]-'0')*10 + int(b[1]-'0')
	month = int(b[2]-'0')*10 + int(b[3]-'0')
	yy := int(b[4]-'0')*10 + int(b[5]-'0')
	year = 2000 + yy
	if yy >= 80 {
		year = 1900 + yy
	}
	if month < 1 || month > 12 || day < 1 || day > 31 {
		return 0, 0, 0, false
	}
	return year, month, day, true
}
