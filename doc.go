// SPDX-License-Identifier: BSD-3-Clause
//
// Copyright (c) 2026, the go-hocon/hocon authors

// Package hocon is a pure-Go (no cgo, stdlib-only) parser for HOCON, the
// Human-Optimized Config Object Notation used by Typesafe Config. HOCON is a
// superset of JSON that adds unquoted keys and strings, `=` as an alias for
// `:`, optional commas and root braces, `#` and `//` comments, dotted-path
// keys, deep object merging of duplicate keys, array and value concatenation,
// `${path}` / `${?path}` substitutions with environment fallback, `+=`
// self-append, `include` directives and duration / size unit suffixes.
//
// The entry point is [Parse], which lexes, parses, merges and resolves an input
// string into a [*Config]. Substitutions are resolved after the whole tree is
// built. Includes and environment lookups are performed through injectable
// seams ([WithIncludeResolver], [WithEnv]) so callers — and tests — need never
// touch the filesystem or process environment.
//
// Values are modelled by [ConfigValue] and classified by [ConfigValueType].
// Typed accessors on [Config] (GetString, GetInt, GetFloat, GetBool,
// GetDuration, GetBytes, GetList, GetObject, GetOrElse, HasPath) read dotted
// paths and report typed errors. [Config.Render] serialises a config back to
// HOCON or JSON.
package hocon
