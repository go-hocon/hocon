<!--
SPDX-License-Identifier: BSD-3-Clause
Copyright (c) 2026, the go-hocon/hocon authors
-->

# go-hocon

[![CI](https://github.com/go-hocon/hocon/actions/workflows/ci.yml/badge.svg)](https://github.com/go-hocon/hocon/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/go-hocon/hocon.svg)](https://pkg.go.dev/github.com/go-hocon/hocon)
[![Documentation](https://img.shields.io/badge/docs-mkdocs--material-4F46E5?style=flat-square)](https://go-hocon.github.io/docs/)
[![License: BSD-3-Clause](https://img.shields.io/badge/license-BSD--3--Clause-blue?style=flat-square)](LICENSE)
![Go 1.26.4+](https://img.shields.io/badge/go-1.26.4%2B-00ADD8?style=flat-square&logo=go&logoColor=white)
![Coverage 100%](https://img.shields.io/badge/coverage-100%25-1a7f37?style=flat-square)

**Pure-Go (`CGO_ENABLED=0`, stdlib-only) parser for HOCON** — the
Human-Optimized Config Object Notation used by Lightbend/Typesafe Config.
HOCON is a superset of JSON: unquoted keys/strings, `=` as an alias for `:`,
optional commas and root braces, `#`/`//` comments, dotted-path keys, deep
object merging of duplicate keys, array/value concatenation, `${path}` /
`${?path}` substitutions with environment fallback, `+=` self-append,
`include` directives (`"file"`, `file(...)`, `url(...)`, `classpath(...)`,
`required(...)`) and duration/size unit suffixes.

Validated differentially against the reference
[`puppetlabs/ruby-hocon`](https://github.com/puppetlabs/ruby-hocon) gem
(itself a port of Lightbend/Typesafe Config) across the spec's constructs.
Cross-compiles to the six 64-bit Go targets (amd64, arm64, riscv64, loong64,
ppc64le, s390x) and WebAssembly; 100% test coverage, including error
branches, enforced as a CI gate.

## Install

```sh
go get github.com/go-hocon/hocon
```

## Usage

```go
import "github.com/go-hocon/hocon"

cfg, err := hocon.Parse(`
  app {
    name = checkout
    workers = 4
    timeout = 30s
    max-size = 8MB
    listen = ${?PORT}
  }
`)
if err != nil { /* typed parse/resolve error */ }

name, _ := cfg.GetString("app.name")      // "checkout"
n, _ := cfg.GetInt("app.workers")         // 4
d, _ := cfg.GetDuration("app.timeout")    // 30 * time.Second
b, _ := cfg.GetBytes("app.max-size")      // 8000000 (SI "MB"; use "MiB" for 8388608)
fmt.Print(cfg.Render(hocon.RenderOptions{}))
```

`Parse` lexes, parses, merges and resolves an input string into a `*Config`.
Includes and environment lookups run through the injectable `WithIncludeResolver`
and `WithEnv` options — no filesystem or process environment touched in tests.
Typed accessors report typed errors: `GetString`, `GetInt`, `GetFloat`,
`GetBool`, `GetDuration`, `GetBytes`, `GetList`, `GetObject`, `GetOrElse`,
`HasPath`. A literal dot in a key (`"a.b" = 1`) is addressed with the
segmented accessors `GetValuePath`/`HasPathSegments`. `WithFallback` layers a
second config underneath; `Render` serialises back to HOCON or JSON.

See the [API docs](https://go-hocon.github.io/docs/api/) and
[`go doc github.com/go-hocon/hocon`](https://pkg.go.dev/github.com/go-hocon/hocon)
for the full surface, and [BENCHMARKS.md](BENCHMARKS.md) for measured figures
against the reference Ruby gem.

## License

BSD-3-Clause. See [LICENSE](LICENSE).
