# M5Unit-GPS-Go

[![test](https://github.com/nicolasmmb/M5Unit-GPS-Go/actions/workflows/test.yml/badge.svg)](https://github.com/nicolasmmb/M5Unit-GPS-Go/actions/workflows/test.yml)

Pure-Go NMEA 0183 parser and reader for the **M5Stack Unit GPS v1.1**
(CASIC AT6668 + MAX2659). No C, C++, CGo or Arduino.

## Features

- GGA (quality, satellites, altitude), RMC (UTC, latitude/longitude, speed,
  course), GSV (satellites in view and best SNR) and TXT (antenna state).
- NMEA checksum validation and talker-independent parsing (`GP/GN/BD/GL/GA`).
- **Zero heap allocations per sentence**: fixed line buffer, in-place field
  scanning and hand-written number parsing.
- Background reader with a thread-safe `Fix()` snapshot.
- `$PCAS04` work-mode command (`SetWorkMode`), built without allocations.
- Host tests, allocation assertion and benchmarks.

## Hardware

| Parameter | Value |
|---|---|
| Module | M5Stack Unit GPS v1.1 (AT6668 + MAX2659) |
| Interface | UART 115200 8N1 |
| Wiring (M5StickC) | TX = G32, RX = G33, Grove |
| Older U032 (AT6558) | UART 9600 8N1 |

The Grove port is shared with other UART units; only one at a time.

## Install

```bash
go get github.com/nicolasmmb/M5Unit-GPS-Go@v0.1.0
```

## Quick start

```go
uart := machine.UART1
_ = uart.Configure(machine.UARTConfig{
    BaudRate: 115200,
    TX:       machine.GPIO32,
    RX:       machine.GPIO33,
})

dev := gps.New(uart)
dev.Start()
defer dev.Stop()
_ = dev.SetWorkMode(7)

for {
    f := dev.Fix()
    if f.Valid {
        println(f.Latitude, f.Longitude, f.Satellites, f.Altitude)
    }
    time.Sleep(time.Second)
}
```

## API

| Function | Description |
|---|---|
| `New(port io.ReadWriter)` | Device on any read/write stream |
| `Start()` / `Stop()` | Background NMEA reader |
| `Fix()` | Thread-safe snapshot of `Fix` |
| `SetWorkMode(mode)` | Send `$PCAS04,<mode>*CS` |
| `Parser.Feed(line)` | Parse one sentence (host friendly) |

`Fix` fields: `Valid`, `Latitude`, `Longitude`, `Altitude`, `Satellites`,
`Quality`, `SatsInView`, `BestSNR`, `SpeedKnots`, `Course`, `UTC`, `Antenna`,
`Sentences`, `ChecksumErrors`.

## Performance

`Feed` is allocation-free for GGA/RMC. The only allocation is the antenna
string, produced by the rare TXT sentence that changes the antenna state.

```bash
make bench
# BenchmarkFeedRMC-...  0 B/op  0 allocs/op
```

The test suite asserts `testing.AllocsPerRun` == 0 per sentence.

## References

- [M5Stack Unit GPS v1.1](https://docs.m5stack.com/en/unit/Unit-GPS%20v1.1)
- [M5Stack UIFlow driver](https://github.com/m5stack/uiflow-micropython/blob/master/m5stack/libs/driver/atgm336h.py)
- NMEA 0183 and CASIC `PCAS` commands

## License

MIT — see [LICENSE](LICENSE).
