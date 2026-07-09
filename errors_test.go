// SPDX-License-Identifier: BSD-3-Clause
//
// Copyright (c) 2026, the go-hocon/hocon authors

package hocon

import (
	"errors"
	"strings"
	"testing"
)

func wantErr(t *testing.T, src string, substr string, opts ...Option) {
	t.Helper()
	_, err := Parse(src, opts...)
	if err == nil {
		t.Fatalf("Parse(%q): expected error containing %q, got nil", src, substr)
	}
	if !strings.Contains(err.Error(), substr) {
		t.Fatalf("Parse(%q): error %q does not contain %q", src, err.Error(), substr)
	}
}

func TestLexErrors(t *testing.T) {
	wantErr(t, `a = "unterminated`, "unterminated string")
	wantErr(t, "a = \"nl\ninside\"", "unterminated string")
	wantErr(t, `a = "bad\x"`, "invalid escape")
	wantErr(t, `a = "trunc\u12"`, "truncated")
	wantErr(t, `a = "bad\uZZZZ"`, "invalid \\u")
	wantErr(t, `a = "end\`, "unterminated escape")
	wantErr(t, `a = """unterminated`, "unterminated triple")
	wantErr(t, `a = $x`, "expected '{' after '$'")
	wantErr(t, `a = $`, "expected '{' after '$'")
	wantErr(t, `a = ${unterminated`, "unterminated substitution")
	wantErr(t, "a = ${nl\n}", "unterminated substitution")
	wantErr(t, `a = ${}`, "empty substitution path")
	wantErr(t, `a = ${x.}`, "empty substitution segment")
}

func TestParseErrors(t *testing.T) {
	wantErr(t, `[1] extra`, "trailing content")
	wantErr(t, `{a = 1`, "unclosed object")
	wantErr(t, `}`, "unexpected '}'")
	wantErr(t, `:`, "expected a key")
	wantErr(t, `a`, "expected ':', '=', '+=' or '{'")
	wantErr(t, `a..b = 1`, "empty key segment")
	wantErr(t, `a =`, "expected a value")
	wantErr(t, `a = :`, "unexpected token in value")
	wantErr(t, `a = [1`, "unclosed array")
	wantErr(t, `[1`, "unclosed array")      // error from root array
	wantErr(t, `a = { b }`, "expected ':'") // error inside nested object (value)
	wantErr(t, `a { b }`, "expected ':'")   // error inside object with no separator
	wantErr(t, `a += :`, "unexpected token in value")
	wantErr(t, `a = [ : ]`, "unexpected token in value") // error inside array element
}

func TestResolveErrors(t *testing.T) {
	wantErr(t, "a = ${b}\nb = ${a}", "substitution cycle", WithEnv(noEnv))
	wantErr(t, `a = ${nope}`, "unresolved substitution", WithEnv(noEnv))
	wantErr(t, `a = ${a}`, "cycle", WithEnv(noEnv))
	wantErr(t, `a = [ ${nope} ]`, "unresolved", WithEnv(noEnv))     // error inside array element
	wantErr(t, `a = { b = ${nope} }`, "unresolved", WithEnv(noEnv)) // error inside object field
}

func TestGetterTypeErrors(t *testing.T) {
	c := mustParse(t, `
		s = str
		i = 1
		b = true
		arr = [1]
		obj = { x = 1 }
	`)
	if _, err := c.GetString("i"); !errors.Is(err, ErrWrongType) {
		t.Error("GetString wrong type")
	}
	if _, err := c.GetInt("s"); !errors.Is(err, ErrWrongType) {
		t.Error("GetInt wrong type")
	}
	if _, err := c.GetFloat("s"); !errors.Is(err, ErrWrongType) {
		t.Error("GetFloat wrong type")
	}
	if _, err := c.GetBool("s"); !errors.Is(err, ErrWrongType) {
		t.Error("GetBool wrong type")
	}
	if _, err := c.GetList("s"); !errors.Is(err, ErrWrongType) {
		t.Error("GetList wrong type")
	}
	if _, err := c.GetObject("s"); !errors.Is(err, ErrWrongType) {
		t.Error("GetObject wrong type")
	}
}

func TestGetterMissing(t *testing.T) {
	c := mustParse(t, `a = 1`)
	var pe *PathError
	_, err := c.GetValue("missing")
	if !errors.As(err, &pe) || !errors.Is(err, ErrMissing) {
		t.Errorf("want PathError/ErrMissing, got %v", err)
	}
	// intermediate non-object
	if _, err := c.GetValue("a.b"); !errors.Is(err, ErrMissing) {
		t.Errorf("want ErrMissing, got %v", err)
	}
	if c.HasPath("missing") {
		t.Error("HasPath should be false")
	}
	// error surfaces through typed getters
	if _, err := c.GetString("missing"); !errors.Is(err, ErrMissing) {
		t.Error("GetString missing")
	}
	if _, err := c.GetInt("missing"); !errors.Is(err, ErrMissing) {
		t.Error("GetInt missing")
	}
	if _, err := c.GetList("missing"); !errors.Is(err, ErrMissing) {
		t.Error("GetList missing")
	}
	if _, err := c.GetObject("missing"); !errors.Is(err, ErrMissing) {
		t.Error("GetObject missing")
	}
	if v := c.GetOrElse("missing", NewNull()); v.Type() != NullType {
		t.Error("GetOrElse default")
	}
}

func TestDurationBytesErrors(t *testing.T) {
	c := mustParse(t, `
		wrong = true
		bad = "10x"
		nonum = "abc"
		dot = "."
	`)
	if _, err := c.GetDuration("missing"); !errors.Is(err, ErrMissing) {
		t.Error("GetDuration missing")
	}
	if _, err := c.GetDuration("wrong"); !errors.Is(err, ErrWrongType) {
		t.Error("GetDuration wrong type")
	}
	if _, err := c.GetDuration("bad"); err == nil || !strings.Contains(err.Error(), "unknown duration unit") {
		t.Errorf("GetDuration bad unit: %v", err)
	}
	if _, err := c.GetDuration("nonum"); err == nil || !strings.Contains(err.Error(), "no numeric prefix") {
		t.Errorf("GetDuration nonum: %v", err)
	}
	if _, err := c.GetBytes("missing"); !errors.Is(err, ErrMissing) {
		t.Error("GetBytes missing")
	}
	if _, err := c.GetBytes("wrong"); !errors.Is(err, ErrWrongType) {
		t.Error("GetBytes wrong type")
	}
	if _, err := c.GetBytes("bad"); err == nil || !strings.Contains(err.Error(), "unknown size unit") {
		t.Errorf("GetBytes bad unit: %v", err)
	}
	if _, err := c.GetBytes("dot"); err == nil || !strings.Contains(err.Error(), "invalid number") {
		t.Errorf("GetBytes dot: %v", err)
	}
}

func TestErrorStrings(t *testing.T) {
	pe := &ParseError{Msg: "boom", Line: 2, Col: 3}
	if pe.Error() != "hocon: parse error at 2:3: boom" {
		t.Errorf("ParseError.Error = %q", pe.Error())
	}
	re := &ResolveError{Msg: "loop"}
	if re.Error() != "hocon: resolve error: loop" {
		t.Errorf("ResolveError.Error = %q", re.Error())
	}
	perr := &PathError{Path: "a.b", Err: ErrMissing}
	if !strings.Contains(perr.Error(), "a.b") {
		t.Errorf("PathError.Error = %q", perr.Error())
	}
	if !errors.Is(perr, ErrMissing) {
		t.Error("PathError.Unwrap")
	}
}
