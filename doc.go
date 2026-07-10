// SPDX-License-Identifier: BSD-3-Clause
//
// Copyright (c) 2026, the go-hocon/hocon authors

// Package hocon is a pure-Go (no cgo, stdlib-only) parser for HOCON, the
// Human-Optimized Config Object Notation used by Typesafe Config. HOCON is a
// superset of JSON that adds unquoted keys and strings (which may contain
// internal whitespace, e.g. `a b c : 42`), `=` as an alias for `:`, optional
// commas and root braces, `#` and `//` comments, dotted-path keys, deep object
// merging of duplicate keys, array and value concatenation, `${path}` /
// `${?path}` substitutions with environment fallback, `+=` self-append,
// `include` directives (`"file"`, `file(...)`, `url(...)`, `classpath(...)` and
// `required(...)`) and duration / size unit suffixes. Whitespace between the
// simple values of a concatenation is preserved verbatim, and the spec's
// "forbidden characters" (plus, dollar, quote, braces, brackets, colon, equals,
// comma, hash, backtick, caret, question, bang, at, star, ampersand, backslash,
// and the two-character comment opener //) may neither appear in an unquoted
// string nor begin a value or key.
//
// The entry point is [Parse], which lexes, parses, merges and resolves an input
// string into a [*Config]. Substitutions are resolved after the whole tree is
// built. Includes and environment lookups are performed through injectable
// seams ([WithIncludeResolver], [WithEnv]) so callers — and tests — need never
// touch the filesystem or process environment. A plain include silently skips a
// resource that cannot be resolved (the resolver reports [ErrIncludeNotFound]
// or an fs.ErrNotExist-wrapped error); wrapping the target in `required(...)`
// turns a missing resource into a parse error. classpath resolution has no
// meaning outside a JVM, so it is delegated entirely to the resolver seam.
//
// Values are modelled by [ConfigValue] and classified by [ConfigValueType].
// Typed accessors on [Config] (GetString, GetInt, GetFloat, GetBool,
// GetDuration, GetBytes, GetList, GetObject, GetOrElse, HasPath) read dotted
// paths and report typed errors. Because a `.` in a dotted path always starts a
// new object level, a key that contains a literal dot (written quoted, e.g.
// `"a.b" = 1`) is addressed with the segmented accessors [Config.GetValuePath]
// and [Config.HasPathSegments], which take explicit path segments.
// [Config.Render] serialises a config back to HOCON or JSON.
package hocon
