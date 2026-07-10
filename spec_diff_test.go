// SPDX-License-Identifier: BSD-3-Clause
//
// Copyright (c) 2026, the go-hocon/hocon authors

package hocon

import (
	"encoding/json"
	"reflect"
	"testing"
)

// jsonEq reports whether the Go config's unwrapped tree equals the reference
// JSON produced by the ruby `hocon` gem (rendered with concise options). Both
// sides are normalised through encoding/json so numbers compare as float64.
func jsonEq(t *testing.T, c *Config, wantJSON string) bool {
	t.Helper()
	gotBytes, err := json.Marshal(c.Root().Unwrap())
	if err != nil {
		t.Fatalf("marshal go result: %v", err)
	}
	var got, want any
	if err := json.Unmarshal(gotBytes, &got); err != nil {
		t.Fatalf("unmarshal go result: %v", err)
	}
	if err := json.Unmarshal([]byte(wantJSON), &want); err != nil {
		t.Fatalf("unmarshal reference %q: %v", wantJSON, err)
	}
	return reflect.DeepEqual(got, want)
}

// TestDifferentialAgainstRubyHocon is a table of inputs paired with the exact
// JSON the reference ruby `hocon` gem produces (captured from
// Hocon::ConfigFactory.parse_string(src).resolve.root.render(concise)). It
// pins go-hocon's parse+resolve to the reference across the HOCON spec's
// constructs: unquoted keys/values with internal whitespace (the bug this
// change fixes), value/array/object concatenation, substitutions, self-
// reference, `+=`, dotted and literal-dot keys, comments and null handling.
func TestDifferentialAgainstRubyHocon(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string // reference JSON from ruby hocon
	}{
		{"unquoted_key_spaces", "valid hocon: string value", `{"valid hocon":"string value"}`},
		{"key_double_space", "a b  c : 42", `{"a b  c":42}`},
		{"val_multi_space", "a = the quick   brown fox", `{"a":"the quick   brown fox"}`},
		{"basic_scalars", "s=hello\nq=\"q s\"\ni=42\nf=3.14\nbt=true\nbf=false\nn=null",
			`{"bf":false,"bt":true,"f":3.14,"i":42,"n":null,"q":"q s","s":"hello"}`},
		{"dotted", "a.b.c = 1", `{"a":{"b":{"c":1}}}`},
		{"quoted_dot_key", `"a.b" = 1`, `{"a.b":1}`},
		{"deep_merge", "a={x=1,y=1}\na={y=2,z=3}", `{"a":{"x":1,"y":2,"z":3}}`},
		{"arr_concat", "a=[1,2] [3,4]", `{"a":[1,2,3,4]}`},
		{"obj_concat", "a={x={p=1}} {x={q=2}}", `{"a":{"x":{"p":1,"q":2}}}`},
		{"obj_concat_replace", "a={x=1} {x=2}", `{"a":{"x":2}}`},
		{"selfref", "x=1\na=${x}\na=${a}", `{"a":1,"x":1}`},
		{"selfref_arr", "a=[1,2]\na=${a} [3,4]", `{"a":[1,2,3,4]}`},
		{"pluseq_fresh", "a += 1", `{"a":[1]}`},
		{"pluseq_append", "a=[1,2]\na += 3", `{"a":[1,2,3]}`},
		{"subst_concat", "n=1\nb=true\nz=null\ns=pre ${n} ${b} ${z} post",
			`{"b":true,"n":1,"s":"pre 1 true null post","z":null}`},
		{"subst_nested", "a{b=5}\nc=${a.b}", `{"a":{"b":5},"c":5}`},
		{"inherit", "g={cs=6}\ne=${g} {name=east}", `{"e":{"cs":6,"name":"east"},"g":{"cs":6}}`},
		{"path_add", "p=[bin]\np=${p} [usr]", `{"p":["bin","usr"]}`},
		{"arr_no_comma", "a=[1 2 3 4]", `{"a":["1 2 3 4"]}`},
		{"arr_of_arrs", "a=[ [1,2] [3,4] ]", `{"a":[[1,2,3,4]]}`},
		{"truekey", "true : 42", `{"true":42}`},
		{"numkey", "3.14 : 42", `{"3":{"14":42}}`},
		{"foo_include_key", "{ foo include : 42 }", `{"foo include":42}`},
		{"obj_no_sep", "a { b = 1 }", `{"a":{"b":1}}`},
		{"nullconcat", "a = x null y", `{"a":"x null y"}`},
		{"trailing_comma", "a=[1,2,3,]", `{"a":[1,2,3]}`},
		{"nested_ws_key", "{ foo  bar : baz }", `{"foo  bar":"baz"}`},
		{"unquoted_slash", "path = foo/bar/baz", `{"path":"foo/bar/baz"}`},
		{"neg_num", "n = -5", `{"n":-5}`},
		{"empty_obj_arr", "a={}\nb=[]", `{"a":{},"b":[]}`},
		{"concat_true_str", "a = true foo", `{"a":"true foo"}`},
		{"comments", "# c\na = 1 // t\n// x\nb = 2 # z", `{"a":1,"b":2}`},
		{"parens_unquoted", "a = foo(bar)", `{"a":"foo(bar)"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, err := Parse(tc.src, WithEnv(noEnv))
			if err != nil {
				t.Fatalf("Parse(%q) error: %v", tc.src, err)
			}
			if !jsonEq(t, c, tc.want) {
				got, _ := json.Marshal(c.Root().Unwrap())
				t.Errorf("Parse(%q):\n got  %s\n want %s", tc.src, got, tc.want)
			}
		})
	}
}

// TestDifferentialErrors pins the inputs that the reference ruby `hocon` gem
// rejects: mixing objects/arrays into a string concatenation, and the spec's
// "forbidden characters" (which may neither appear in unquoted strings nor
// begin a value or key).
func TestDifferentialErrors(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{"str_obj_concat", "a : abc { x : y }"},
		{"arr_obj_concat", "a : [1] { x : y }"},
		{"str_arr_concat", "a : abc [1, 2]"},
		{"null_obj_concat", "a : null { x : y }"},
		{"lone_plus", "n = +5"},
		{"plus_in_word", "a = foo+bar"},
		{"bang", "a = foo!bar"},
		{"star", "a = foo*bar"},
		{"caret", "a = a^b"},
		{"at", "a = a@b"},
		{"amp", "a = a&b"},
		{"question", "a = a?b"},
		{"backtick", "a = a`b"},
		{"backslash", `a = a\b`},
		{"lead_reserved", "a = !foo"},
		{"reserved_key", "!foo = 1"},
		{"reserved_in_array", "a = [ !x ]"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Parse(tc.src, WithEnv(noEnv)); err == nil {
				t.Errorf("Parse(%q) should have failed", tc.src)
			}
		})
	}
}

// TestRegressionUnquotedSpaces is the dedicated regression test for the bug this
// change fixes: unquoted keys and values containing internal whitespace were
// wrongly rejected. Each case here previously failed to parse or lost spaces.
func TestRegressionUnquotedSpaces(t *testing.T) {
	c := mustParse(t, "valid hocon: string value", WithEnv(noEnv))
	if v, _ := c.GetString("valid hocon"); v != "string value" {
		t.Errorf("key 'valid hocon' = %q, want %q", v, "string value")
	}
	// Internal whitespace is preserved verbatim in both keys and values.
	c2 := mustParse(t, "a  b   c :  x   y ", WithEnv(noEnv))
	cv, err := c2.GetValuePath("a  b   c")
	if err != nil {
		t.Fatalf("GetValuePath: %v", err)
	}
	if cv.str != "x   y" {
		t.Errorf("value = %q, want %q", cv.str, "x   y")
	}
	// Tabs between values are preserved too.
	c3 := mustParse(t, "k = a\tb", WithEnv(noEnv))
	if v, _ := c3.GetString("k"); v != "a\tb" {
		t.Errorf("tab value = %q, want %q", v, "a\tb")
	}
}

// TestLiteralDotKeyAccess covers the segmented accessor for keys that contain a
// literal dot (written quoted), which the dotted getters cannot reach.
func TestLiteralDotKeyAccess(t *testing.T) {
	c := mustParse(t, `"a.b" = 1`)
	// The dotted getter looks for nested a -> b and fails.
	if _, err := c.GetValue("a.b"); err == nil {
		t.Error("GetValue(\"a.b\") should not find the literal-dot key")
	}
	// The segmented accessor addresses the literal key.
	v, err := c.GetValuePath("a.b")
	if err != nil {
		t.Fatalf("GetValuePath: %v", err)
	}
	if v.Type() != NumberType || v.num != 1 {
		t.Errorf("GetValuePath value = %v", v.Unwrap())
	}
	if !c.HasPathSegments("a.b") {
		t.Error("HasPathSegments(\"a.b\") should be true")
	}
	if c.HasPathSegments("a", "b") {
		t.Error("HasPathSegments(\"a\",\"b\") should be false")
	}
	// Multi-segment GetValuePath descends like a dotted path.
	c2 := mustParse(t, "a.b.c = 7")
	got, err := c2.GetValuePath("a", "b", "c")
	if err != nil {
		t.Fatalf("GetValuePath segments: %v", err)
	}
	if got.num != 7 {
		t.Errorf("GetValuePath(a,b,c) = %v", got.Unwrap())
	}
	// Missing intermediate through a non-object.
	if _, err := c2.GetValuePath("a", "b", "c", "d"); err == nil {
		t.Error("expected missing for path through scalar")
	}
}
