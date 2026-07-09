// SPDX-License-Identifier: BSD-3-Clause
//
// Copyright (c) 2026, the go-hocon/hocon authors

package hocon

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderRoundTrip(t *testing.T) {
	src := `
		name = app
		nested = { a = 1, b = [1, 2], empty = {}, emptyarr = [] }
		flt = 1.5
		on = true
		nothing = null
	`
	c := mustParse(t, src)
	// HOCON render then re-parse.
	hocon := c.Render(RenderOptions{})
	if !strings.Contains(hocon, "=") {
		t.Errorf("HOCON render lacks '=':\n%s", hocon)
	}
	c2 := mustParse(t, hocon)
	if v, _ := c2.GetString("name"); v != "app" {
		t.Errorf("round-trip name = %q", v)
	}
	if v, _ := c2.GetInt("nested.a"); v != 1 {
		t.Errorf("round-trip nested.a = %d", v)
	}
	// JSON render is valid HOCON too (colons + commas).
	j := c.Render(RenderOptions{JSON: true, Indent: "    "})
	if !strings.Contains(j, ":") || !strings.Contains(j, ",") {
		t.Errorf("JSON render:\n%s", j)
	}
	c3 := mustParse(t, j)
	if v, _ := c3.GetFloat("flt"); v != 1.5 {
		t.Errorf("json round-trip flt = %v", v)
	}
	if v, _ := c3.GetBool("on"); v != true {
		t.Errorf("json round-trip on = %v", v)
	}
}

func TestRenderTypes(t *testing.T) {
	c := mustParse(t, `a = "x"`)
	got := c.Render(RenderOptions{})
	if !strings.Contains(got, `"a"`) {
		t.Errorf("render = %q", got)
	}
}

func TestConfigValueTypeString(t *testing.T) {
	cases := map[ConfigValueType]string{
		NullType:    "null",
		BooleanType: "boolean",
		NumberType:  "number",
		StringType:  "string",
		ArrayType:   "array",
		ObjectType:  "object",
	}
	for typ, want := range cases {
		if typ.String() != want {
			t.Errorf("%d.String() = %q want %q", typ, typ.String(), want)
		}
	}
}

func TestIncludeSeam(t *testing.T) {
	resolver := func(kind, name string) (string, error) {
		switch {
		case kind == "file" && name == "base.conf":
			return "base = 1\nshared = frombase", nil
		case kind == "url" && name == "http://x/y.conf":
			return "urlval = 2", nil
		case kind == "classpath" && name == "app.conf":
			return "cpval = 3", nil
		case kind == "file" && name == "bare.conf":
			return "bareval = 4", nil
		}
		return "", errors.New("not found")
	}
	c := mustParse(t, `
		include file("base.conf")
		include url("http://x/y.conf")
		include classpath("app.conf")
		include "bare.conf"
		shared = override
	`, WithIncludeResolver(resolver), WithEnv(noEnv))
	if v, _ := c.GetInt("base"); v != 1 {
		t.Errorf("base = %d", v)
	}
	if v, _ := c.GetInt("urlval"); v != 2 {
		t.Errorf("urlval = %d", v)
	}
	if v, _ := c.GetInt("cpval"); v != 3 {
		t.Errorf("cpval = %d", v)
	}
	if v, _ := c.GetInt("bareval"); v != 4 {
		t.Errorf("bareval = %d", v)
	}
	if v, _ := c.GetString("shared"); v != "override" {
		t.Errorf("shared = %q (merge order)", v)
	}
}

func TestIncludeErrors(t *testing.T) {
	failing := func(kind, name string) (string, error) {
		return "", errors.New("nope")
	}
	wantErr(t, `include file("x.conf")`, "nope", WithIncludeResolver(failing))
	// lexing error inside included content
	badLex := func(kind, name string) (string, error) { return `a = "unterminated`, nil }
	wantErr(t, `include file("x.conf")`, "unterminated string", WithIncludeResolver(badLex))
	// parse error inside included content
	badParse := func(kind, name string) (string, error) { return `}`, nil }
	wantErr(t, `include file("x.conf")`, "unexpected '}'", WithIncludeResolver(badParse))
	// malformed directives
	wantErr(t, `include foo`, "expected include target")
	wantErr(t, `include {`, "expected include target")
	wantErr(t, `include required("x")`, "required(...) includes are not supported")
	wantErr(t, `include bogus(`, "unknown include qualifier")
	wantErr(t, `include file(`, "expected quoted string")
	wantErr(t, "include file(\"x\"", "expected ')'")
}

func TestIncludeReservedAsKey(t *testing.T) {
	// `include` not followed by whitespace is an ordinary key.
	c := mustParse(t, `include:1`)
	if v, _ := c.GetInt("include"); v != 1 {
		t.Errorf("include = %d", v)
	}
}

func TestDefaultIncludeFromDisk(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "inc.conf")
	if err := os.WriteFile(path, []byte("disk = 7"), 0o644); err != nil {
		t.Fatal(err)
	}
	c := mustParse(t, `include file("`+path+`")`, WithEnv(noEnv))
	if v, _ := c.GetInt("disk"); v != 7 {
		t.Errorf("disk = %d", v)
	}
	// missing file
	wantErr(t, `include file("`+filepath.Join(dir, "nope.conf")+`")`, "no such file")
	// unsupported kind for default resolver
	wantErr(t, `include url("http://x")`, "require a custom resolver")
}

func TestDefaultEnv(t *testing.T) {
	t.Setenv("HOCON_TEST_VAR", "envvalue")
	c := mustParse(t, `a = ${HOCON_TEST_VAR}`)
	if v, _ := c.GetString("a"); v != "envvalue" {
		t.Errorf("a = %q", v)
	}
}

func TestCloneElemPath(t *testing.T) {
	// self-reference where prior value is itself a substitution exercises
	// cloneElem's path copy.
	c := mustParse(t, "x = 9\na = ${x}\na = ${a}", WithEnv(noEnv))
	if v, _ := c.GetInt("a"); v != 9 {
		t.Errorf("a = %d", v)
	}
}

func TestNewConstructors(t *testing.T) {
	if NewNull().Type() != NullType {
		t.Error("NewNull")
	}
	if !NewBool(true).b {
		t.Error("NewBool")
	}
	if NewNumber(2).num != 2 {
		t.Error("NewNumber")
	}
	if NewString("s").str != "s" {
		t.Error("NewString")
	}
	if NewArray(NewNumber(1)).Type() != ArrayType {
		t.Error("NewArray")
	}
	if NewObject().Type() != ObjectType {
		t.Error("NewObject")
	}
}
