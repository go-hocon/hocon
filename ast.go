// SPDX-License-Identifier: BSD-3-Clause
//
// Copyright (c) 2026, the go-hocon/hocon authors

package hocon

// elemKind classifies a piece of a value concatenation.
type elemKind int

const (
	elemLit   elemKind = iota // a scalar token (unquoted or quoted string)
	elemSubst                 // a ${path} / ${?path} substitution
	elemObj                   // an object literal
	elemArr                   // an array literal
)

// astElem is one piece of a value concatenation. wsBefore holds the whitespace
// that separated it from the previous element (used for string concatenation).
type astElem struct {
	kind     elemKind
	text     string   // literal text (unescaped for quoted, raw for unquoted)
	quoted   bool     // true when the literal came from quotes
	path     []string // substitution path
	optional bool     // ${?path}
	obj      *astObject
	arr      []*astValue
	wsBefore string
}

func (e *astElem) cloneElem() *astElem {
	c := *e
	if e.path != nil {
		c.path = append([]string(nil), e.path...)
	}
	return &c
}

// astValue is a (possibly single-element) value concatenation.
type astValue struct {
	elems []*astElem
}

// astObject is a merged object: fields keyed by name, keys preserving insertion
// order so rendering and resolution are deterministic.
type astObject struct {
	keys   []string
	fields map[string]*astValue
}

func newAstObject() *astObject {
	return &astObject{fields: map[string]*astValue{}}
}

func (o *astObject) put(key string, v *astValue) {
	if _, ok := o.fields[key]; !ok {
		o.keys = append(o.keys, key)
	}
	o.fields[key] = v
}

func pathEq(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func singleObj(v *astValue) bool {
	return len(v.elems) == 1 && v.elems[0].kind == elemObj
}

func objOf(v *astValue) *astObject { return v.elems[0].obj }

// lookup returns the merged value at a dotted path, or nil if absent or an
// intermediate segment is not an object.
func (o *astObject) lookup(path []string) *astValue {
	cur := o
	for i := 0; i < len(path)-1; i++ {
		v, ok := cur.fields[path[i]]
		if !ok || !singleObj(v) {
			return nil
		}
		cur = objOf(v)
	}
	return cur.fields[path[len(path)-1]]
}

// childObject returns the object stored at key, creating an empty one (or
// replacing a non-object) as needed.
func (o *astObject) childObject(key string) *astObject {
	if v, ok := o.fields[key]; ok && singleObj(v) {
		return objOf(v)
	}
	child := newAstObject()
	o.put(key, &astValue{elems: []*astElem{{kind: elemObj, obj: child}}})
	return child
}

// assign merges val into the object at the given path, first inlining any
// self-referential substitution (`${path}` pointing at the field being set,
// which powers `+=` and self-append) using the field's prior value.
func (o *astObject) assign(path []string, val *astValue) {
	prior := o.lookup(path)
	val = inlineSelf(val, path, prior)
	o.place(path, val)
}

func (o *astObject) place(path []string, val *astValue) {
	if len(path) == 1 {
		key := path[0]
		if ex, ok := o.fields[key]; ok && singleObj(ex) && singleObj(val) {
			objOf(ex).mergeFrom(objOf(val))
			return
		}
		o.put(key, val)
		return
	}
	o.childObject(path[0]).place(path[1:], val)
}

// mergeFrom deep-merges src into o (both objects), src winning.
func (o *astObject) mergeFrom(src *astObject) {
	for _, k := range src.keys {
		sv := src.fields[k]
		if dv, ok := o.fields[k]; ok && singleObj(dv) && singleObj(sv) {
			objOf(dv).mergeFrom(objOf(sv))
			continue
		}
		o.put(k, sv)
	}
}

// inlineSelf replaces every substitution in val that targets path with prior's
// elements (deep-copied). A missing optional self-reference is dropped; a
// missing required one is left in place to fail later at resolve time.
func inlineSelf(val *astValue, path []string, prior *astValue) *astValue {
	out := &astValue{}
	for _, e := range val.elems {
		if e.kind == elemSubst && pathEq(e.path, path) {
			switch {
			case prior != nil:
				for i, pe := range prior.elems {
					ce := pe.cloneElem()
					if i == 0 {
						ce.wsBefore = e.wsBefore
					}
					out.elems = append(out.elems, ce)
				}
			case e.optional:
				// drop: optional self-reference with no prior value
			default:
				out.elems = append(out.elems, e)
			}
			continue
		}
		out.elems = append(out.elems, e)
	}
	return out
}
