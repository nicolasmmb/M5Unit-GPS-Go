# M5Unit-GPS-Go

[![test](https://github.com/nicolasmmb/M5Unit-GPS-Go/actions/workflows/test.yml/badge.svg)](https://github.com/nicolasmmb/M5Unit-GPS-Go/actions/workflows/test.yml)

Pure-Go NMEA 0183 parser and reader for the **M5Stack Unit GPS v1.1**
(CASIC AT6668 + MAX2659). No C, C++, CGo or Arduino.

## Features

- GGA (quality, satellites, altitude), RMC (UTC, latitude/longitude, speed,
  course), GSV (satellites in view and best SNR, aggregated over GPS/GLONASS/BeiDou/Galileo/QZSS) and TXT (antenna state).
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

Zero heap allocations in steady state: `Feed` never touches the heap for
GGA/RMC/GSV/TXT, and the antenna string is only built when the state actually
changes (normally once, at boot).

Benchmarks use `testdata/cycle.nmea`, **one second of NMEA captured from real
hardware** (13 sentences, GPS + GLONASS + BeiDou):

```bash
make bench          # all benchmarks with alloc stats
make alloc          # asserts 0 allocs per cycle + RAM footprint
make bench-save NAME=before
make bench-save NAME=after && make bench-compare FROM=before TO=after
make profile        # CPU/alloc profiles (pprof)
```

| Benchmark | Result (Apple M2 Pro) |
|---|---:|
| `FeedCycle` (1 s of traffic) | **1.57 µs/op, 0 B, 0 allocs** |
| `DeviceCycle` (mutex + parse + publish) | **1.66 µs/op, 0 B, 0 allocs** |
| `Feed/GGA` / `RMC` / `GSV` / `TXT` | 273 / 316 / 208 / 53 ns |
| `Fix` | 2.5 ns/op |
| `Parser` / `Device` size | **160 B / 256 B** |

See [BENCHMARKS.md](BENCHMARKS.md) for the method and the before/after
(`benchstat`) of the v0.3.1 allocation work.

The test suite asserts `testing.AllocsPerRun` == 0 per sentence and per full
cycle, and fails if `Parser`/`Device` exceed their RAM budget.

## References

- [M5Stack Unit GPS v1.1](https://docs.m5stack.com/en/unit/Unit-GPS%20v1.1)
- [M5Stack UIFlow driver](https://github.com/m5stack/uiflow-micropython/blob/master/m5stack/libs/driver/atgm336h.py)
- NMEA 0183 and CASIC `PCAS` commands

## License

MIT — see [LICENSE](LICENSE).
