package gps

import (
	"io"
	"sync"
	"time"
)

// Device reads NMEA sentences from a port in the background.
//
// machine.UART satisfies io.ReadWriter. A Device is safe for concurrent use.
type Device struct {
	mu     sync.RWMutex
	port   io.ReadWriter
	parser Parser

	life    sync.Mutex
	stop    chan struct{}
	running bool

	// cmd is reused by SetWorkMode so the one-shot command does not allocate
	// a fresh buffer on the heap.
	cmdMu sync.Mutex
	cmd   [24]byte
}

// New returns a Device reading from port.
func New(port io.ReadWriter) *Device {
	return &Device{port: port}
}

// Start begins reading sentences in a goroutine.
func (d *Device) Start() {
	d.life.Lock()
	if d.running {
		d.life.Unlock()
		return
	}
	d.stop = make(chan struct{})
	d.running = true
	stop := d.stop
	d.life.Unlock()
	go d.loop(stop)
}

// Stop stops the reader goroutine.
func (d *Device) Stop() {
	d.life.Lock()
	if !d.running {
		d.life.Unlock()
		return
	}
	close(d.stop)
	d.running = false
	d.life.Unlock()
}

// Fix returns a copy of the latest decoded state.
func (d *Device) Fix() Fix {
	d.mu.RLock()
	f := d.parser.fix
	d.mu.RUnlock()
	return f
}

func (d *Device) loop(stop chan struct{}) {
	var line [128]byte
	n := 0
	var chunk [32]byte
	for {
		select {
		case <-stop:
			return
		default:
		}
		m, err := d.port.Read(chunk[:])
		if m > 0 {
			for _, b := range chunk[:m] {
				if b == '\n' {
					d.consume(line[:n])
					n = 0
				} else if n < len(line) {
					line[n] = b
					n++
				} else {
					n = 0
				}
			}
		}
		if m == 0 || err != nil {
			time.Sleep(10 * time.Millisecond)
		}
	}
}

// consume parses one sentence. Fix reads the parser state under the lock, so
// there is no need to copy the whole Fix for every sentence.
func (d *Device) consume(line []byte) {
	d.mu.Lock()
	d.parser.Feed(line)
	d.mu.Unlock()
}

// SetWorkMode sends the CASIC $PCAS04 command (for example mode 7).
func (d *Device) SetWorkMode(mode uint8) error {
	d.cmdMu.Lock()
	defer d.cmdMu.Unlock()
	b := &d.cmd
	n := copy(b[:], "$PCAS04,")
	n += appendUint(b[n:], int(mode))
	b[n] = '*'
	sum := 0
	for i := 1; i < n; i++ {
		sum ^= int(b[i])
	}
	b[n+1] = hexChar(sum >> 4)
	b[n+2] = hexChar(sum & 0x0F)
	b[n+3] = '\r'
	b[n+4] = '\n'
	_, err := d.port.Write(b[:n+5])
	return err
}

func appendUint(b []byte, n int) int {
	if n == 0 {
		b[0] = '0'
		return 1
	}
	var tmp [3]byte
	i := 0
	for n > 0 {
		tmp[i] = byte('0' + n%10)
		n /= 10
		i++
	}
	for j := 0; j < i; j++ {
		b[j] = tmp[i-1-j]
	}
	return i
}

func hexChar(v int) byte {
	if v < 10 {
		return byte('0' + v)
	}
	return byte('A' + v - 10)
}
