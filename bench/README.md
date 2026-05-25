# Benchmark baselines

Snapshots used to detect performance regressions across releases. Re-run on
the same machine when capturing a new baseline; per-machine absolute numbers
aren't directly comparable, but trends across the same machine are.

## Files

- `before-main.txt` — output of `BenchmarkURLGeneration*`, `BenchmarkIDGeneration*`
  on `main` (pre-v0.6.0). Captured immediately before the v0.6.0 ID/URLFor
  semantic changes for diffing with `after-v06.txt`.
- `after-v06.txt` — same benchmarks plus the new `BenchmarkURLGenerationV05`
  and `BenchmarkIDGenerationV06` suites, run on the v0.6.0 branch.

## How to re-capture

```sh
go test -run NoTest \
    -bench 'BenchmarkURLGeneration|BenchmarkURLGenerationStrict|BenchmarkURLGenerationV05|BenchmarkIDGeneration|BenchmarkIDGenerationV06' \
    -benchtime=2s -count=3 \
    > bench/$(git describe --always).txt 2>&1
```

## Comparing

```sh
go install golang.org/x/perf/cmd/benchstat@latest
benchstat bench/before-main.txt bench/after-v06.txt
```

## Profiles

`cpu.pprof` and `mem.pprof` (gitignored) can be regenerated with:

```sh
go test -run NoTest -bench BenchmarkIDGenerationV06 -benchtime=10s \
    -cpuprofile bench/cpu.pprof -memprofile bench/mem.pprof
go tool pprof -top -cum bench/cpu.pprof
go tool pprof -alloc_space -top -cum bench/mem.pprof
```
