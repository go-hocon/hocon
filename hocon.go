// SPDX-License-Identifier: BSD-3-Clause
//
// Copyright (c) 2026, the go-hocon/hocon authors

package hocon

import "os"

// IncludeResolver resolves an `include` directive to the text of the included
// document. kind is one of "file", "url" or "classpath"; name is the target.
// It is an injectable seam so callers (and tests) can supply content without
// touching the filesystem.
type IncludeResolver func(kind, name string) (string, error)

// EnvLookup resolves a substitution name against the environment when it is not
// found in the config, mirroring os.LookupEnv. It is an injectable seam.
type EnvLookup func(name string) (string, bool)

// defaultInclude reads "file" includes from disk and rejects other kinds.
func defaultInclude(kind, name string) (string, error) {
	if kind != "file" {
		return "", &ParseError{Msg: kind + "(...) includes require a custom resolver"}
	}
	b, err := os.ReadFile(name)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

type options struct {
	include IncludeResolver
	env     EnvLookup
}

// Option customises [Parse].
type Option func(*options)

// WithIncludeResolver overrides how `include` directives are resolved.
func WithIncludeResolver(r IncludeResolver) Option {
	return func(o *options) { o.include = r }
}

// WithEnv overrides environment-variable fallback for substitutions.
func WithEnv(e EnvLookup) Option {
	return func(o *options) { o.env = e }
}

// Parse lexes, parses, merges and resolves a HOCON document into a [*Config].
func Parse(src string, opts ...Option) (*Config, error) {
	o := &options{include: defaultInclude, env: os.LookupEnv}
	for _, opt := range opts {
		opt(o)
	}
	toks, err := lex(src)
	if err != nil {
		return nil, err
	}
	p := &parser{toks: toks, include: o.include}
	root, err := p.parseRoot()
	if err != nil {
		return nil, err
	}
	r := &resolver{root: root, env: o.env, visiting: map[string]bool{}}
	cv, err := r.resolveObject(root)
	if err != nil {
		return nil, err
	}
	return &Config{root: cv}, nil
}
