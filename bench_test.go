// SPDX-License-Identifier: BSD-3-Clause
//
// Copyright (c) 2026, the go-hocon/hocon authors

package hocon

import "testing"

// benchConfig is a representative HOCON document exercising the full feature
// surface: nested objects, dotted keys, arrays, substitutions, self-reference,
// value/object/array concatenation, unit suffixes and comments.
const benchConfig = `
# application configuration
app {
  name = my service
  version = "1.4.2"
  tags = [web, api, "edge node"]
}
defaults = { timeout = 30s, retries = 3, pool { size = 8, idle = 2 } }
service = ${defaults} {
  name = ${app.name}
  endpoint = "https://"${app.name}".example.com"
  timeout = 45s
  pool { size = 16 }
}
limits {
  max-bytes = 256MiB
  window = 5 min
}
hosts = [ 10.0.0.1, 10.0.0.2 ]
hosts += 10.0.0.3
paths = [ /bin ]
paths = ${paths} [ /usr/bin, /usr/local/bin ]
feature.flags.beta = true // trailing comment
feature.flags.gamma = false
message = the quick brown fox jumped over ${app.name}
`

func BenchmarkParse(b *testing.B) {
	env := func(string) (string, bool) { return "", false }
	b.ReportAllocs()
	for b.Loop() {
		if _, err := Parse(benchConfig, WithEnv(env)); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseSmall(b *testing.B) {
	const src = "a = 1\nb = ${a}\nc = { x = 2, y = [1, 2, 3] }"
	env := func(string) (string, bool) { return "", false }
	b.ReportAllocs()
	for b.Loop() {
		if _, err := Parse(src, WithEnv(env)); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRender(b *testing.B) {
	env := func(string) (string, bool) { return "", false }
	c, err := Parse(benchConfig, WithEnv(env))
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for b.Loop() {
		_ = c.Render(RenderOptions{})
	}
}
