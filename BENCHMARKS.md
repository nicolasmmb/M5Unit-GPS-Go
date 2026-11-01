# Benchmarks

Everything here runs on the host (`go test`), because the parser and the
device pipeline are pure Go. The input is not synthetic: `testdata/cycle.nmea`
is **one second of NMEA captured from a real Unit GPS v1.1 indoors**
(GPS + GLONASS + BeiDou, 13 sentences, 637 bytes), which is the steady-state
load the library sees on the M5StickC.

## What is measured

| Benchmark | Realistic quantity |
|---|---|
| `BenchmarkFeed/GGA`, `RMC`, `GSV`, `TXT` | cost of each sentence type |
| `BenchmarkFeedCycle` | **one second of traffic** (all 13 sentences) |
| `BenchmarkDeviceCycle` | the same second through `Device` (mutex + parse + publish) |
| `BenchmarkFix` / `BenchmarkDeviceFix` | snapshot read (the screen does this at 5–10 Hz) |
| `TestNoAllocPerCycle` | contract: one full cycle touches **0 B** of heap |
| `TestAntennaAlloc` | repeated `TXT` (sent every second) does not allocate |
| `TestSetWorkModeNoAlloc` | the one-shot `PCAS04` command does not allocate |
| `TestFootprint` | `unsafe.Sizeof` budget for `Parser` and `Device` |

## How to run

```bash
make bench                 # all benchmarks, -count=8, with alloc stats
make alloc                 # allocation + footprint contracts
make footprint             # RAM sizes only
make bench-save NAME=before
# ... change the code ...
make bench-save NAME=after
make bench-compare FROM=before TO=after
make profile               # CPU + alloc profiles, pprof top
make escape                # compiler heap-escape report
```

`scripts/bench.sh` accepts `COUNT`, `BENCH` and `BENCHTIME`:

```bash
COUNT=20 BENCH='FeedCycle|DeviceCycle' make bench
```

## Results: v0.3.0 vs v0.3.1 (Apple M2 Pro, `-count=8`, `benchstat`)

| Metric | Before | After | Change |
|---|---:|---:|---:|
| `FeedCycle` B/op | 4 | **0** | −100% |
| `FeedCycle` allocs/op | 1 | **0** | −100% |
| `DeviceCycle` B/op | 4 | **0** | −100% |
| `DeviceCycle` allocs/op | 1 | **0** | −100% |
| `Feed/TXT` ns/op | 60.29 | **53.49** | −11.3% |
| `Feed/TXT` B/op | 4 | **0** | −100% |
| `DeviceCycle` ns/op | 1681 | **1659** | −1.3% |
| `Parser` size | 280 B | **160 B** | −42.9% |
| `Device` size | 480 B | **256 B** | −46.7% |

`Feed` for GGA/RMC/GSV was already allocation-free and did not regress
(benchstat: GGA ~0%, RMC +0.4%, GSV +0.2%; all within 1 ns).

| Benchmark | After (0 allocs) |
|---|---:|
| `Feed/GGA` | 272.8 ns/op |
| `Feed/RMC` | 316.0 ns/op |
| `Feed/GSV` | 207.9 ns/op |
| `Feed/TXT` | 53.49 ns/op |
| `FeedCycle` (13 sentences) | 1.573 µs |
| `DeviceCycle` | 1.659 µs |
| `Fix` | 2.48 ns/op |

### What that means on hardware

- The module sends ~13 sentences/s. Parsing one second costs **~1.6 µs on the
  host** and an estimated **~5 µs on the 240 MHz ESP32** — under 0.001% CPU.
- Before v0.3.1 the repeated `ANTENNA OPEN` TXT sentence allocated **1 object
  per second** (86,400/day). It now allocates only when the antenna state
  actually changes (normally once, at boot).
- `Device` no longer keeps a second copy of `Fix` (136 B) and no longer copies
  it for every sentence; `Fix()` copies it once per read under `RLock`.

### Why the numbers changed

1. **Antenna state**: `decodeTXT` compared the new value by converting it to a
   `string`, allocating even when unchanged. It now compares bytes against the
   existing string (`sameBytes`) and converts only on a real change.
2. **Device snapshot**: the reader published `d.fix = d.parser.fix` after every
   sentence. `Fix()` now returns `parser.fix` under the lock, so a second of
   traffic does 13 copies less.
3. **GSV slots**: `gsvState` used two `int`s per constellation (144 B). Counts
   fit in a byte, so six slots now take 18 B inside `Parser`.
4. **Commands**: `SetWorkMode` reuses a 24-byte buffer in `Device` instead of
   letting the array escape to the heap on every call.

Host numbers are Go on arm64; TinyGo/ESP32 is slower in absolute terms, but the
allocation counts and the structure sizes are what matter for GC pressure and
RAM, and those carry over directly.
