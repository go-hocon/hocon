// SPDX-License-Identifier: BSD-3-Clause
//
// Copyright (c) 2026, the go-hocon/hocon authors

package hocon

import "testing"

func TestWithFallback(t *testing.T) {
	base := mustParse(t, `
		a = 1
		b = { x = 1, y = 1 }
		onlybase = 9
	`)
	over := mustParse(t, `
		a = 2
		b = { y = 2, z = 3 }
		onlyover = 8
	`)
	m := over.WithFallback(base)
	if v, _ := m.GetInt("a"); v != 2 {
		t.Errorf("a = %d (override should win)", v)
	}
	if v, _ := m.GetInt("b.x"); v != 1 {
		t.Errorf("b.x = %d (from fallback)", v)
	}
	if v, _ := m.GetInt("b.y"); v != 2 {
		t.Errorf("b.y = %d (override)", v)
	}
	if v, _ := m.GetInt("b.z"); v != 3 {
		t.Errorf("b.z = %d (override only)", v)
	}
	if v, _ := m.GetInt("onlybase"); v != 9 {
		t.Errorf("onlybase = %d", v)
	}
	if v, _ := m.GetInt("onlyover"); v != 8 {
		t.Errorf("onlyover = %d", v)
	}
	// Inputs are unmodified.
	if v, _ := base.GetInt("a"); v != 1 {
		t.Errorf("base mutated: a = %d", v)
	}
}

func TestWithFallbackScalarOverObject(t *testing.T) {
	// A non-object override replaces the fallback object entirely.
	base := mustParse(t, `a = { x = 1 }`)
	over := mustParse(t, `a = 5`)
	m := over.WithFallback(base)
	if v, _ := m.GetInt("a"); v != 5 {
		t.Errorf("a = %d", v)
	}
}

func TestNewObjectOfAndAccessors(t *testing.T) {
	obj := NewObjectOf(map[string]*ConfigValue{
		"k": NewNumber(1),
		"s": NewString("v"),
	})
	if obj.Type() != ObjectType {
		t.Fatal("want object")
	}
	f := obj.Fields()
	if len(f) != 2 || f["s"].Type() != StringType {
		t.Errorf("Fields = %v", f)
	}
	arr := NewArray(NewNumber(1), NewNumber(2))
	if els := arr.Elements(); len(els) != 2 {
		t.Errorf("Elements len = %d", len(els))
	}
	// Non-container accessors return nil.
	if NewNull().Fields() != nil {
		t.Error("Fields on null should be nil")
	}
	if NewNull().Elements() != nil {
		t.Error("Elements on null should be nil")
	}
}
