// SPDX-License-Identifier: BSD-3-Clause
//
// Copyright (c) 2026, the go-hocon/hocon authors

package hocon

import (
	"sort"
	"strconv"
	"strings"
)

// ConfigValueType classifies a [ConfigValue].
type ConfigValueType int

// The value types HOCON can hold. They mirror JSON plus HOCON's distinction
// between the six is otherwise identical to JSON.
const (
	NullType ConfigValueType = iota
	BooleanType
	NumberType
	StringType
	ArrayType
	ObjectType
)

// String names the type (useful in error messages and tests).
func (t ConfigValueType) String() string {
	switch t {
	case NullType:
		return "null"
	case BooleanType:
		return "boolean"
	case NumberType:
		return "number"
	case StringType:
		return "string"
	case ArrayType:
		return "array"
	default:
		return "object"
	}
}

// ConfigValue is a single resolved HOCON value.
type ConfigValue struct {
	typ ConfigValueType
	num float64
	str string
	b   bool
	arr []*ConfigValue
	obj map[string]*ConfigValue
}

// NewNull returns a null value.
func NewNull() *ConfigValue { return &ConfigValue{typ: NullType} }

// NewBool returns a boolean value.
func NewBool(b bool) *ConfigValue { return &ConfigValue{typ: BooleanType, b: b} }

// NewNumber returns a numeric value.
func NewNumber(n float64) *ConfigValue { return &ConfigValue{typ: NumberType, num: n} }

// NewString returns a string value.
func NewString(s string) *ConfigValue { return &ConfigValue{typ: StringType, str: s} }

// NewArray returns an array value over the given elements.
func NewArray(elems ...*ConfigValue) *ConfigValue {
	return &ConfigValue{typ: ArrayType, arr: elems}
}

// NewObject returns an (empty) object value.
func NewObject() *ConfigValue {
	return &ConfigValue{typ: ObjectType, obj: map[string]*ConfigValue{}}
}

// NewObjectOf returns an object value populated from entries (copied).
func NewObjectOf(entries map[string]*ConfigValue) *ConfigValue {
	o := NewObject()
	for k, v := range entries {
		o.obj[k] = v
	}
	return o
}

// Fields returns a shallow copy of an object value's entries. It returns nil
// for non-object values.
func (v *ConfigValue) Fields() map[string]*ConfigValue {
	if v.typ != ObjectType {
		return nil
	}
	out := make(map[string]*ConfigValue, len(v.obj))
	for k, e := range v.obj {
		out[k] = e
	}
	return out
}

// Elements returns an array value's elements, or nil for non-array values.
func (v *ConfigValue) Elements() []*ConfigValue {
	if v.typ != ArrayType {
		return nil
	}
	return v.arr
}

// Type reports the value's type.
func (v *ConfigValue) Type() ConfigValueType { return v.typ }

// Unwrap returns the value as a native Go value: nil, bool, float64, string,
// []any or map[string]any.
func (v *ConfigValue) Unwrap() any {
	switch v.typ {
	case NullType:
		return nil
	case BooleanType:
		return v.b
	case NumberType:
		return v.num
	case StringType:
		return v.str
	case ArrayType:
		out := make([]any, len(v.arr))
		for i, e := range v.arr {
			out[i] = e.Unwrap()
		}
		return out
	default:
		out := make(map[string]any, len(v.obj))
		for k, e := range v.obj {
			out[k] = e.Unwrap()
		}
		return out
	}
}

// mergeInto deep-merges src into dst (both objects), src winning on conflicts;
// two objects at the same key merge recursively, otherwise src replaces dst.
func mergeInto(dst, src *ConfigValue) {
	for k, sv := range src.obj {
		if dv, ok := dst.obj[k]; ok && dv.typ == ObjectType && sv.typ == ObjectType {
			mergeInto(dv, sv)
			continue
		}
		dst.obj[k] = sv
	}
}

// numberString renders a float without a trailing ".0" for integral values.
func numberString(n float64) string {
	if n == float64(int64(n)) {
		return strconv.FormatInt(int64(n), 10)
	}
	return strconv.FormatFloat(n, 'g', -1, 64)
}

// String renders the value's scalar content as it would appear concatenated
// into a larger string (numbers and booleans stringified, null as "null").
func (v *ConfigValue) scalarString() string {
	switch v.typ {
	case NullType:
		return "null"
	case BooleanType:
		return strconv.FormatBool(v.b)
	case NumberType:
		return numberString(v.num)
	default:
		return v.str
	}
}

// RenderOptions controls serialisation.
type RenderOptions struct {
	// JSON emits strict JSON (quoted keys, colons, commas) instead of HOCON.
	JSON bool
	// Indent is the per-level indentation string; empty means two spaces.
	Indent string
}

// Render serialises the whole config using the given options.
func (v *ConfigValue) render(opts RenderOptions) string {
	indent := opts.Indent
	if indent == "" {
		indent = "  "
	}
	var b strings.Builder
	renderValue(&b, v, opts, indent, 0)
	return b.String()
}

func pad(b *strings.Builder, indent string, depth int) {
	for range depth {
		b.WriteString(indent)
	}
}

func renderValue(b *strings.Builder, v *ConfigValue, opts RenderOptions, indent string, depth int) {
	switch v.typ {
	case NullType:
		b.WriteString("null")
	case BooleanType:
		b.WriteString(strconv.FormatBool(v.b))
	case NumberType:
		b.WriteString(numberString(v.num))
	case StringType:
		b.WriteString(strconv.Quote(v.str))
	case ArrayType:
		renderArray(b, v, opts, indent, depth)
	default:
		renderObject(b, v, opts, indent, depth)
	}
}

func renderArray(b *strings.Builder, v *ConfigValue, opts RenderOptions, indent string, depth int) {
	if len(v.arr) == 0 {
		b.WriteString("[]")
		return
	}
	b.WriteString("[\n")
	for i, e := range v.arr {
		pad(b, indent, depth+1)
		renderValue(b, e, opts, indent, depth+1)
		if opts.JSON && i < len(v.arr)-1 {
			b.WriteByte(',')
		}
		b.WriteByte('\n')
	}
	pad(b, indent, depth)
	b.WriteByte(']')
}

func renderObject(b *strings.Builder, v *ConfigValue, opts RenderOptions, indent string, depth int) {
	if len(v.obj) == 0 {
		b.WriteString("{}")
		return
	}
	keys := make([]string, 0, len(v.obj))
	for k := range v.obj {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	b.WriteString("{\n")
	for i, k := range keys {
		pad(b, indent, depth+1)
		b.WriteString(strconv.Quote(k))
		if opts.JSON {
			b.WriteString(": ")
		} else {
			b.WriteString(" = ")
		}
		renderValue(b, v.obj[k], opts, indent, depth+1)
		if opts.JSON && i < len(keys)-1 {
			b.WriteByte(',')
		}
		b.WriteByte('\n')
	}
	pad(b, indent, depth)
	b.WriteByte('}')
}
