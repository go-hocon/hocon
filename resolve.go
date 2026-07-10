// SPDX-License-Identifier: BSD-3-Clause
//
// Copyright (c) 2026, the go-hocon/hocon authors

package hocon

import (
	"strconv"
	"strings"
)

// resolver resolves an [astObject] into a [ConfigValue] tree, expanding
// substitutions against the root and the environment seam.
type resolver struct {
	root     *astObject
	env      EnvLookup
	visiting map[string]bool
}

func (r *resolver) resolveObject(o *astObject) (*ConfigValue, error) {
	out := NewObject()
	for _, k := range o.keys {
		cv, err := r.resolveValue(o.fields[k])
		if err != nil {
			return nil, err
		}
		if cv == nil {
			continue // optional substitution resolved to nothing
		}
		out.obj[k] = cv
	}
	return out, nil
}

type resolvedElem struct {
	cv     *ConfigValue
	ws     string
	unqLit bool
	raw    string
}

func (r *resolver) resolveValue(v *astValue) (*ConfigValue, error) {
	var parts []resolvedElem
	for _, e := range v.elems {
		cv, unq, raw, err := r.resolveElem(e)
		if err != nil {
			return nil, err
		}
		if cv == nil {
			continue // optional substitution that resolved to nothing
		}
		parts = append(parts, resolvedElem{cv: cv, ws: e.wsBefore, unqLit: unq, raw: raw})
	}
	switch len(parts) {
	case 0:
		return nil, nil
	case 1:
		if parts[0].unqLit {
			return interpretScalar(parts[0].raw), nil
		}
		return parts[0].cv, nil
	default:
		return combine(parts)
	}
}

func (r *resolver) resolveElem(e *astElem) (cv *ConfigValue, unqLit bool, raw string, err error) {
	switch e.kind {
	case elemLit:
		if e.quoted {
			return NewString(e.text), false, "", nil
		}
		return NewString(e.text), true, e.text, nil
	case elemObj:
		o, err := r.resolveObject(e.obj)
		return o, false, "", err
	case elemArr:
		out := NewArray()
		for _, av := range e.arr {
			ev, err := r.resolveValue(av)
			if err != nil {
				return nil, false, "", err
			}
			if ev == nil {
				continue
			}
			out.arr = append(out.arr, ev)
		}
		return out, false, "", nil
	default: // elemSubst
		sv, err := r.resolveSubst(e.path, e.optional)
		return sv, false, "", err
	}
}

func (r *resolver) resolveSubst(path []string, optional bool) (*ConfigValue, error) {
	key := strings.Join(path, ".")
	if r.visiting[key] {
		return nil, &ResolveError{Msg: "substitution cycle at ${" + key + "}"}
	}
	if av := r.root.lookup(path); av != nil {
		r.visiting[key] = true
		cv, err := r.resolveValue(av)
		delete(r.visiting, key)
		if err != nil {
			return nil, err
		}
		return cv, nil
	}
	if r.env != nil {
		if val, ok := r.env(key); ok {
			return NewString(val), nil
		}
	}
	if optional {
		return nil, nil
	}
	return nil, &ResolveError{Msg: "unresolved substitution ${" + key + "}"}
}

// interpretScalar turns a single unquoted token into a typed value.
func interpretScalar(raw string) *ConfigValue {
	switch raw {
	case "true":
		return NewBool(true)
	case "false":
		return NewBool(false)
	case "null":
		return NewNull()
	}
	if n, err := strconv.ParseFloat(raw, 64); err == nil {
		return NewNumber(n)
	}
	return NewString(raw)
}

// combine folds multiple concatenation parts. Per the HOCON spec, objects
// concatenate with objects (deep merge) and arrays with arrays (concatenation),
// but mixing an object or array with anything else is an error; otherwise the
// parts are simple values that string-concatenate, preserving the whitespace
// between them verbatim.
func combine(parts []resolvedElem) (*ConfigValue, error) {
	anyObj, anyArr, anyScalar := false, false, false
	for _, p := range parts {
		switch p.cv.typ {
		case ObjectType:
			anyObj = true
		case ArrayType:
			anyArr = true
		default:
			anyScalar = true
		}
	}
	switch {
	case anyObj && !anyArr && !anyScalar:
		out := NewObject()
		for _, p := range parts {
			mergeInto(out, p.cv)
		}
		return out, nil
	case anyArr && !anyObj && !anyScalar:
		out := NewArray()
		for _, p := range parts {
			out.arr = append(out.arr, p.cv.arr...)
		}
		return out, nil
	case anyObj || anyArr:
		return nil, &ResolveError{Msg: "cannot concatenate an object or list with a non-object-or-list value"}
	default:
		var b strings.Builder
		for i, p := range parts {
			if i > 0 {
				b.WriteString(p.ws)
			}
			b.WriteString(p.cv.scalarString())
		}
		return NewString(b.String()), nil
	}
}
