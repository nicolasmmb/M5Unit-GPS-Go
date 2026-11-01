//go:build esp32

// Example: read the M5Stack Unit GPS v1.1 over UART and print the fix.
package main

import (
	"time"

	"machine"

	gps "github.com/nicolasmmb/M5Unit-GPS-Go"
)

func main() {
	time.Sleep(2 * time.Second)

	uart := machine.UART1
	_ = uart.Configure(machine.UARTConfig{
		BaudRate: 115200, // GPS v1.1 (AT6668)
		TX:       machine.GPIO32,
		RX:       machine.GPIO33,
	})

	dev := gps.New(uart)
	dev.Start()
	_ = dev.SetWorkMode(7)
	println("gps: waiting for NMEA at 115200")

	for {
		f := dev.Fix()
		println("sats", f.Satellites, "valid", f.Valid,
			"lat", f.Latitude, "lon", f.Longitude, "alt", f.Altitude,
			"utc", f.UTC.Format("15:04:05"), "antenna", f.Antenna,
			"sentences", f.Sentences, "errors", f.ChecksumErrors)
		time.Sleep(time.Second)
	}
}
