// SPDX-License-Identifier: BSD-3-Clause
//
// Copyright (c) 2026, the go-hocon/hocon authors

package hocon

import (
	"testing"
	"time"
)

func mustParse(t *testing.T, src string, opts ...Option) *Config {
	t.Helper()
	c, err := Parse(src, opts...)
	if err != nil {
		t.Fatalf("Parse(%q) error: %v", src, err)
	}
	return c
}

func TestBasicScalars(t *testing.T) {
	c := mustParse(t, `
		s = hello
		q = "quoted string"
		i = 42
		f = 3.14
		bt = true
		bf = false
		n = null
	`)
	if v, _ := c.GetString("s"); v != "hello" {
		t.Errorf("s = %q", v)
	}
	if v, _ := c.GetString("q"); v != "quoted string" {
		t.Errorf("q = %q", v)
	}
	if v, _ := c.GetInt("i"); v != 42 {
		t.Errorf("i = %d", v)
	}
	if v, _ := c.GetFloat("f"); v != 3.14 {
		t.Errorf("f = %v", v)
	}
	if v, _ := c.GetBool("bt"); v != true {
		t.Errorf("bt = %v", v)
	}
	if v, _ := c.GetBool("bf"); v != false {
		t.Errorf("bf = %v", v)
	}
	if v, _ := c.GetValue("n"); v.Type() != NullType {
		t.Errorf("n type = %v", v.Type())
	}
}

func TestSeparatorsAndCommas(t *testing.T) {
	c := mustParse(t, `{ "a": 1, b = 2
	 c : 3 }`)
	for k, want := range map[string]int64{"a": 1, "b": 2, "c": 3} {
		if v, _ := c.GetInt(k); v != want {
			t.Errorf("%s = %d want %d", k, v, want)
		}
	}
}

func TestComments(t *testing.T) {
	c := mustParse(t, `
		# hash comment
		a = 1 // slash comment
		// another
		b = 2 # trailing
	`)
	if v, _ := c.GetInt("a"); v != 1 {
		t.Errorf("a = %d", v)
	}
	if v, _ := c.GetInt("b"); v != 2 {
		t.Errorf("b = %d", v)
	}
}

func TestDottedKeys(t *testing.T) {
	c := mustParse(t, `a.b.c = 1`)
	if v, _ := c.GetInt("a.b.c"); v != 1 {
		t.Errorf("a.b.c = %d", v)
	}
	if !c.HasPath("a.b") {
		t.Error("a.b should exist")
	}
	sub, err := c.GetObject("a.b")
	if err != nil {
		t.Fatal(err)
	}
	if v, _ := sub.GetInt("c"); v != 1 {
		t.Errorf("sub c = %d", v)
	}
}

func TestQuotedDottedKey(t *testing.T) {
	// A quoted key with a dot is a single key, not a nested path.
	c := mustParse(t, `"a.b" = 1`)
	root := c.Root().Unwrap().(map[string]any)
	if root["a.b"] != float64(1) {
		t.Errorf(`root["a.b"] = %v`, root["a.b"])
	}
	if _, ok := root["a"]; ok {
		t.Error("should not have created nested key a")
	}
}

func TestObjectDeepMerge(t *testing.T) {
	c := mustParse(t, `
		a = { x = 1, y = 1 }
		a = { y = 2, z = 3 }
	`)
	if v, _ := c.GetInt("a.x"); v != 1 {
		t.Errorf("a.x = %d", v)
	}
	if v, _ := c.GetInt("a.y"); v != 2 {
		t.Errorf("a.y = %d", v)
	}
	if v, _ := c.GetInt("a.z"); v != 3 {
		t.Errorf("a.z = %d", v)
	}
}

func TestDottedKeysReuseChild(t *testing.T) {
	c := mustParse(t, "a.b = 1\na.c = 2")
	if v, _ := c.GetInt("a.b"); v != 1 {
		t.Errorf("a.b = %d", v)
	}
	if v, _ := c.GetInt("a.c"); v != 2 {
		t.Errorf("a.c = %d", v)
	}
}

func TestNestedObjectDeepMerge(t *testing.T) {
	c := mustParse(t, "a = { x = { p = 1 } }\na = { x = { q = 2 } }")
	if v, _ := c.GetInt("a.x.p"); v != 1 {
		t.Errorf("p = %d", v)
	}
	if v, _ := c.GetInt("a.x.q"); v != 2 {
		t.Errorf("q = %d", v)
	}
}

func TestUnquotedSlashAndPlus(t *testing.T) {
	c := mustParse(t, "path = foo/bar/baz\nn = +5")
	if v, _ := c.GetString("path"); v != "foo/bar/baz" {
		t.Errorf("path = %q", v)
	}
	if v, _ := c.GetInt("n"); v != 5 {
		t.Errorf("n = %d", v)
	}
}

func TestUnicodeEscape(t *testing.T) {
	c := mustParse(t, `a = "\u0041\u00e9z"`)
	if v, _ := c.GetString("a"); v != "Aéz" {
		t.Errorf("a = %q", v)
	}
}

func TestObjectNoSeparator(t *testing.T) {
	c := mustParse(t, `a { b = 1 }`)
	if v, _ := c.GetInt("a.b"); v != 1 {
		t.Errorf("a.b = %d", v)
	}
}

func TestDottedOverridesScalarToObject(t *testing.T) {
	c := mustParse(t, "a = 1\na.b = 2")
	if v, _ := c.GetInt("a.b"); v != 2 {
		t.Errorf("a.b = %d", v)
	}
}

func TestArrayConcat(t *testing.T) {
	c := mustParse(t, `a = [1, 2] [3, 4]`)
	lst, err := c.GetList("a")
	if err != nil {
		t.Fatal(err)
	}
	if len(lst) != 4 {
		t.Fatalf("len = %d", len(lst))
	}
}

func TestArrayWithOptionalHole(t *testing.T) {
	c := mustParse(t, `a = [ ${?missing}, 1 ]`, WithEnv(noEnv))
	lst, _ := c.GetList("a")
	if len(lst) != 1 {
		t.Fatalf("len = %d", len(lst))
	}
}

func TestValueConcatString(t *testing.T) {
	c := mustParse(t, `a = the quick   brown fox`)
	if v, _ := c.GetString("a"); v != "the quick brown fox" {
		t.Errorf("a = %q", v)
	}
}

func TestObjectConcatMerge(t *testing.T) {
	c := mustParse(t, `a = { x = { p = 1 } } { x = { q = 2 } }`)
	if v, _ := c.GetInt("a.x.p"); v != 1 {
		t.Errorf("p = %d", v)
	}
	if v, _ := c.GetInt("a.x.q"); v != 2 {
		t.Errorf("q = %d", v)
	}
}

func TestObjectConcatReplace(t *testing.T) {
	c := mustParse(t, `a = { x = 1 } { x = 2 }`)
	if v, _ := c.GetInt("a.x"); v != 2 {
		t.Errorf("x = %d", v)
	}
}

func noEnv(string) (string, bool) { return "", false }

func TestSubstitutionInternal(t *testing.T) {
	c := mustParse(t, "a = 1\nb = ${a}", WithEnv(noEnv))
	if v, _ := c.GetInt("b"); v != 1 {
		t.Errorf("b = %d", v)
	}
}

func TestSubstitutionNested(t *testing.T) {
	c := mustParse(t, "a { b = 5 }\nc = ${a.b}", WithEnv(noEnv))
	if v, _ := c.GetInt("c"); v != 5 {
		t.Errorf("c = %d", v)
	}
}

func TestSubstitutionEnvFallback(t *testing.T) {
	env := func(name string) (string, bool) {
		if name == "MYVAR" {
			return "fromenv", true
		}
		return "", false
	}
	c := mustParse(t, `a = ${MYVAR}`, WithEnv(env))
	if v, _ := c.GetString("a"); v != "fromenv" {
		t.Errorf("a = %q", v)
	}
}

func TestSubstitutionOptionalMissing(t *testing.T) {
	c := mustParse(t, `a = ${?missing}`, WithEnv(noEnv))
	if c.HasPath("a") {
		t.Error("a should be omitted")
	}
}

func TestSubstitutionInStringConcat(t *testing.T) {
	c := mustParse(t, "n = 1\nb = true\nz = null\ns = pre ${n} ${b} ${z} post", WithEnv(noEnv))
	if v, _ := c.GetString("s"); v != "pre 1 true null post" {
		t.Errorf("s = %q", v)
	}
}

func TestSelfReferenceSet(t *testing.T) {
	c := mustParse(t, "x = 1\na = ${x}\na = ${a}", WithEnv(noEnv))
	if v, _ := c.GetInt("a"); v != 1 {
		t.Errorf("a = %d", v)
	}
}

func TestPlusEqualsFresh(t *testing.T) {
	c := mustParse(t, `a += 1`, WithEnv(noEnv))
	lst, _ := c.GetList("a")
	if len(lst) != 1 {
		t.Fatalf("len = %d", len(lst))
	}
}

func TestPlusEqualsAppend(t *testing.T) {
	c := mustParse(t, "a = [1, 2]\na += 3", WithEnv(noEnv))
	lst, _ := c.GetList("a")
	if len(lst) != 3 {
		t.Fatalf("len = %d", len(lst))
	}
}

func TestUnits(t *testing.T) {
	c := mustParse(t, `
		d1 = 10s
		d2 = "5 min"
		d3 = 100
		d4 = 250ms
		d5 = "-2 hours"
		b1 = 10MB
		b2 = 1GiB
		b3 = "512 bytes"
		b4 = 2048
		b5 = 1.5kb
	`)
	cases := []struct {
		path string
		want time.Duration
	}{
		{"d1", 10 * time.Second},
		{"d2", 5 * time.Minute},
		{"d3", 100 * time.Millisecond},
		{"d4", 250 * time.Millisecond},
		{"d5", -2 * time.Hour},
	}
	for _, tc := range cases {
		got, err := c.GetDuration(tc.path)
		if err != nil || got != tc.want {
			t.Errorf("GetDuration(%s) = %v, %v; want %v", tc.path, got, err, tc.want)
		}
	}
	bcases := []struct {
		path string
		want int64
	}{
		{"b1", 10_000_000},
		{"b2", 1 << 30},
		{"b3", 512},
		{"b4", 2048},
		{"b5", 1500},
	}
	for _, tc := range bcases {
		got, err := c.GetBytes(tc.path)
		if err != nil || got != tc.want {
			t.Errorf("GetBytes(%s) = %v, %v; want %v", tc.path, got, err, tc.want)
		}
	}
}

func TestTripleQuoted(t *testing.T) {
	c := mustParse(t, `a = """line1
line2 "with quotes" """`)
	v, _ := c.GetString("a")
	want := "line1\nline2 \"with quotes\" "
	if v != want {
		t.Errorf("a = %q want %q", v, want)
	}
}

func TestEscapes(t *testing.T) {
	c := mustParse(t, `a = "tab\tnl\nquote\"back\\slash\/ret\rbell\bff\fuA"`)
	v, _ := c.GetString("a")
	want := "tab\tnl\nquote\"back\\slash/ret\rbell\bff\fuA"
	if v != want {
		t.Errorf("a = %q want %q", v, want)
	}
}

func TestTopLevelArray(t *testing.T) {
	c := mustParse(t, `[1, 2, 3]`)
	lst, err := c.GetList("")
	if err != nil {
		t.Fatal(err)
	}
	if len(lst) != 3 {
		t.Fatalf("len = %d", len(lst))
	}
}

func TestGetOrElse(t *testing.T) {
	c := mustParse(t, `a = 1`)
	if v := c.GetOrElse("a", NewString("def")); v.Type() != NumberType {
		t.Error("want existing value")
	}
	if v := c.GetOrElse("missing", NewString("def")); v.str != "def" {
		t.Error("want default")
	}
}

func TestUnwrap(t *testing.T) {
	c := mustParse(t, `
		s = hi
		i = 2
		b = true
		n = null
		arr = [1, "x"]
		obj = { k = 1 }
	`)
	root := c.Root().Unwrap().(map[string]any)
	if root["s"] != "hi" {
		t.Errorf("s = %v", root["s"])
	}
	if root["b"] != true {
		t.Errorf("b = %v", root["b"])
	}
	if root["n"] != nil {
		t.Errorf("n = %v", root["n"])
	}
	arr := root["arr"].([]any)
	if len(arr) != 2 || arr[0] != float64(1) || arr[1] != "x" {
		t.Errorf("arr = %v", arr)
	}
	obj := root["obj"].(map[string]any)
	if obj["k"] != float64(1) {
		t.Errorf("obj = %v", obj)
	}
	_ = i0(c)
}

func i0(c *Config) int64 { v, _ := c.GetInt("i"); return v }
