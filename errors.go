// SPDX-License-Identifier: BSD-3-Clause
//
// Copyright (c) 2026, the go-hocon/hocon authors

package hocon

import (
	"errors"
	"fmt"
)

// ErrMissing is returned (wrapped in a [*PathError]) when a path is absent.
var ErrMissing = errors.New("path not found")

// ErrWrongType is returned (wrapped in a [*PathError]) when a value at a path
// exists but is not of the requested type.
var ErrWrongType = errors.New("wrong value type")

// ParseError describes a lexical or syntactic failure, with 1-based position.
type ParseError struct {
	Msg  string
	Line int
	Col  int
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("hocon: parse error at %d:%d: %s", e.Line, e.Col, e.Msg)
}

// ResolveError describes a failure while resolving substitutions (unresolved
// required substitution, or a substitution cycle).
type ResolveError struct {
	Msg string
}

func (e *ResolveError) Error() string { return "hocon: resolve error: " + e.Msg }

// PathError wraps [ErrMissing] or [ErrWrongType] with the offending path.
type PathError struct {
	Path string
	Err  error
}

func (e *PathError) Error() string {
	return fmt.Sprintf("hocon: %s: %v", e.Path, e.Err)
}

func (e *PathError) Unwrap() error { return e.Err }
