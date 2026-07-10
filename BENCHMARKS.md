<!--
SPDX-License-Identifier: BSD-3-Clause
Copyright (c) 2026, the go-hocon/hocon authors
-->

# Benchmarks

`go-hocon` is benchmarked against its behavioural reference, the Ruby
[`hocon`](https://github.com/puppetlabs/ruby-hocon) gem (itself a faithful port
of Lightbend/Typesafe Config). The standing rule for the org is that a pure-Go
implementation must be **at least as fast as the reference**; the numbers below
show go-hocon parsing and resolving the same document roughly **70× faster**.

## Methodology

Both implementations parse **and fully resolve** (`.resolve`) an identical,
representative HOCON document that exercises the whole feature surface: nested
objects, dotted keys, arrays, `${...}` substitutions, self-referential
substitutions, value/object/array concatenation, `+=`, unit suffixes and
comments. The document is the `benchConfig` constant in
[`bench_test.go`](bench_test.go) (the Ruby side reads the byte-identical
`benchcfg.conf`).

- **Go**: `go test -run '^$' -bench . -benchmem`, using Go's `testing.B.Loop`
  harness (auto-tuned iteration count), `go1.26.4`.
- **Ruby**: 100 warm-up iterations, then 5 000 timed iterations via
  `Benchmark.realtime`, reported as microseconds per operation. Ruby 4.0.5.
- Same machine, single run each; wall-clock, no I/O (parse from an in-memory
  string, environment/include seams stubbed).

Reproduce:

```sh
# Go
go test -run '^$' -bench . -benchmem ./...

# Ruby (in a ruby-hocon checkout, benchcfg.conf = the same document)
ruby -Ilib bench.rb
```

## Results

Machine: Apple M4 Max, macOS 26.5.1, Go 1.26.4, Ruby 4.0.5.

### go-hocon (`go test -bench`)

| Benchmark          |    ns/op |     B/op | allocs/op |
|--------------------|---------:|---------:|----------:|
| `BenchmarkParse`   |  ~22 300 |  ~72 900 |      ~738 |
| `BenchmarkParseSmall` | ~3 960 |  ~15 200 |      ~130 |
| `BenchmarkRender`  |   ~3 230 |   ~2 400 |       ~20 |

### Reference: ruby `hocon` gem, same document

| Operation                    |    per op |
|------------------------------|----------:|
| `parse_string(...).resolve`  |  ~1 608 µs |

### Comparison (full parse + resolve of the representative document)

| Implementation | per op    | relative |
|----------------|-----------|----------|
| **go-hocon**   | ~22.3 µs  | **1.0×** (baseline) |
| ruby `hocon`   | ~1 608 µs | ~72× slower |

go-hocon meets and greatly exceeds the "at least as fast as the reference"
bar. Absolute numbers vary with hardware and Go/Ruby versions; the
order-of-magnitude gap is the stable, portable takeaway.
