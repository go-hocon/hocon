// SPDX-License-Identifier: BSD-3-Clause
//
// Copyright (c) 2026, the go-hocon/hocon authors

package hocon

import (
	"errors"
	"io/fs"
	"strings"
)

// parser turns a token stream into a merged [astObject].
type parser struct {
	toks    []token
	pos     int
	include IncludeResolver
}

func (p *parser) tok() token { return p.toks[p.pos] }

func (p *parser) advance() token {
	t := p.toks[p.pos]
	if p.pos < len(p.toks)-1 {
		p.pos++
	}
	return t
}

func (p *parser) errAt(t token, msg string) error {
	return &ParseError{Msg: msg, Line: t.line, Col: t.col}
}

// skipWS consumes whitespace tokens only.
func (p *parser) skipWS() {
	for p.tok().kind == tWS {
		p.advance()
	}
}

// skipSeparators consumes whitespace, newlines and commas.
func (p *parser) skipSeparators() {
	for {
		switch p.tok().kind {
		case tWS, tNL, tComma:
			p.advance()
		default:
			return
		}
	}
}

// parseRoot parses a document. The root may be a braced object, a bare object
// (implicit braces) or a top-level array.
func (p *parser) parseRoot() (*astObject, error) {
	p.skipSeparators()
	root := newAstObject()
	if p.tok().kind == tLBrack {
		arr, err := p.parseArray()
		if err != nil {
			return nil, err
		}
		root.put("", &astValue{elems: []*astElem{{kind: elemArr, arr: arr}}})
	} else {
		braced := false
		if p.tok().kind == tLBrace {
			braced = true
			p.advance()
		}
		if err := p.parseFields(root, braced); err != nil {
			return nil, err
		}
	}
	p.skipSeparators()
	if p.tok().kind != tEOF {
		return nil, p.errAt(p.tok(), "trailing content at top level")
	}
	return root, nil
}

// parseObject parses a `{ ... }` object; the opening brace is already consumed.
func (p *parser) parseObject() (*astObject, error) {
	o := newAstObject()
	if err := p.parseFields(o, true); err != nil {
		return nil, err
	}
	return o, nil
}

func (p *parser) parseFields(o *astObject, braced bool) error {
	for {
		p.skipSeparators()
		switch p.tok().kind {
		case tEOF:
			if braced {
				return p.errAt(p.tok(), "unclosed object")
			}
			return nil
		case tRBrace:
			if !braced {
				return p.errAt(p.tok(), "unexpected '}'")
			}
			p.advance()
			return nil
		}
		if err := p.parseField(o); err != nil {
			return err
		}
	}
}

// parseField parses a single `include`, or a `key (:|=|+=|{) value`.
func (p *parser) parseField(o *astObject) error {
	if p.tok().kind == tUnquoted && p.tok().text == "include" {
		// `include` is only a directive when followed by whitespace + target.
		save := p.pos
		p.advance()
		if p.tok().kind == tWS {
			return p.parseInclude(o)
		}
		p.pos = save
	}
	path, keyTok, err := p.parseKey()
	if err != nil {
		return err
	}
	p.skipWS()
	switch p.tok().kind {
	case tLBrace:
		p.advance()
		obj, err := p.parseObject()
		if err != nil {
			return err
		}
		o.assign(path, &astValue{elems: []*astElem{{kind: elemObj, obj: obj}}})
		return nil
	case tColon, tEquals:
		p.advance()
		val, err := p.parseValue()
		if err != nil {
			return err
		}
		o.assign(path, val)
		return nil
	case tPlusEq:
		p.advance()
		val, err := p.parseValue()
		if err != nil {
			return err
		}
		appended := &astValue{elems: []*astElem{
			{kind: elemSubst, path: path, optional: true},
			{kind: elemArr, arr: []*astValue{val}},
		}}
		o.assign(path, appended)
		return nil
	default:
		return p.errAt(keyTok, "expected ':', '=', '+=' or '{' after key")
	}
}

// parseKey reads a (possibly dotted) key up to the separator. A key is a path
// expression that works like a value concatenation: unquoted and quoted string
// tokens joined with their intervening whitespace. Whitespace *inside* a path
// element is preserved verbatim (so `valid hocon` and `a b  c` are single keys);
// leading and trailing whitespace, and whitespace surrounding a `.` separator,
// is discarded. Dots that appear unquoted split the key into path segments.
func (p *parser) parseKey() ([]string, token, error) {
	first := p.tok()
	var segs []string
	var cur strings.Builder
	segStarted := false // the current segment has at least one non-ws char
	started := false    // at least one key token has been seen
	pendingWS := ""     // whitespace awaiting a following same-segment token

	flush := func() {
		segs = append(segs, cur.String())
		cur.Reset()
		segStarted = false
		pendingWS = ""
	}
	appendPart := func(part string) {
		if segStarted && pendingWS != "" {
			cur.WriteString(pendingWS)
		}
		cur.WriteString(part)
		segStarted = true
		pendingWS = ""
	}

	for {
		t := p.tok()
		switch t.kind {
		case tUnquoted:
			started = true
			parts := strings.Split(t.text, ".")
			for i, part := range parts {
				if i > 0 {
					flush() // a dot ends the current segment (trailing ws dropped)
				}
				if part == "" {
					continue
				}
				appendPart(part)
			}
			p.advance()
		case tString:
			started = true
			appendPart(t.text)
			p.advance()
		case tWS:
			pendingWS = t.text
			p.advance()
		default:
			if !started {
				return nil, first, p.errAt(t, "expected a key")
			}
			flush()
			for _, s := range segs {
				if s == "" {
					return nil, first, p.errAt(first, "empty key segment")
				}
			}
			return segs, first, nil
		}
	}
}

func (p *parser) parseInclude(o *astObject) error {
	p.skipWS()
	directiveTok := p.tok()
	kind, name, required, err := p.parseIncludeTarget()
	if err != nil {
		return err
	}
	content, err := p.include(kind, name)
	if err != nil {
		// A missing resource is silently ignored for a plain include; a
		// required() include instead fails. Any other resolver error is fatal
		// regardless of required(). "Missing" is signalled by fs.ErrNotExist
		// (the default file resolver) or the exported ErrIncludeNotFound seam.
		missing := errors.Is(err, fs.ErrNotExist) || errors.Is(err, ErrIncludeNotFound)
		switch {
		case missing && required:
			return p.errAt(directiveTok, "required include "+kind+"("+name+") could not be resolved: "+err.Error())
		case missing:
			return nil
		default:
			return &ParseError{Msg: "include " + kind + "(" + name + "): " + err.Error(), Line: directiveTok.line, Col: directiveTok.col}
		}
	}
	toks, err := lex(content)
	if err != nil {
		return err
	}
	sub := &parser{toks: toks, include: p.include}
	inc, err := sub.parseRoot()
	if err != nil {
		return err
	}
	// An included document must be an object, not an array (a top-level array
	// is stored under the empty key by parseRoot).
	if _, isArray := inc.fields[""]; isArray {
		return p.errAt(directiveTok, "included resource "+kind+"("+name+") must contain an object, not an array")
	}
	o.mergeFrom(inc)
	return nil
}

// parseIncludeTarget reads an include argument: a bare `"file.conf"` (treated
// as a file heuristic), or a qualifier form `file("...")`, `url("...")`,
// `classpath("...")`, or any of those wrapped in `required(...)` (which makes a
// missing resource an error rather than a silent skip). Because the lexer keeps
// '(' and ')' inside unquoted runs (they are legal unquoted-string characters),
// the qualifier prefix arrives as one or more `qualifier(` tokens — packed
// together (`required(file(`) or separated by whitespace (`required( file(`) —
// and the closing parens arrive as a run of `)` after the quoted string.
func (p *parser) parseIncludeTarget() (kind, name string, required bool, err error) {
	t := p.tok()
	if t.kind == tString { // bare quoted string: heuristic file include
		p.advance()
		return "file", t.text, false, nil
	}
	if t.kind != tUnquoted || !strings.HasSuffix(t.text, "(") {
		return "", "", false, p.errAt(t, "expected include target")
	}
	// Collect the opener qualifiers, which may span several whitespace-separated
	// unquoted tokens; each token is a run of `qualifier(` fragments.
	kind = "file" // heuristic default when only required(...) wraps a string
	opens := 0
	for {
		cur := p.tok()
		if cur.kind != tUnquoted || !strings.HasSuffix(cur.text, "(") {
			break
		}
		for _, q := range strings.Split(strings.TrimSuffix(cur.text, "("), "(") {
			switch q {
			case "required":
				if opens != 0 {
					return "", "", false, p.errAt(cur, "required(...) must be the outermost include qualifier")
				}
				required = true
			case "file", "url", "classpath":
				kind = q
			default:
				return "", "", false, p.errAt(cur, "unknown include qualifier "+q)
			}
			opens++
		}
		p.advance()
		p.skipWS()
	}
	st := p.tok()
	if st.kind != tString {
		return "", "", false, p.errAt(st, "expected quoted string in include qualifier")
	}
	p.advance()
	// Consume exactly `opens` closing parens, which may be spread across tokens
	// and separated by whitespace, e.g. `) )` or a single `))`.
	closes := 0
	for closes < opens {
		p.skipWS()
		ct := p.tok()
		if ct.kind != tUnquoted {
			return "", "", false, p.errAt(ct, "expected ')' after include target")
		}
		for _, ch := range ct.text {
			if ch != ')' {
				return "", "", false, p.errAt(ct, "expected ')' after include target")
			}
			closes++
		}
		p.advance()
		if closes > opens {
			return "", "", false, p.errAt(ct, "unbalanced ')' in include target")
		}
	}
	return kind, st.text, required, nil
}

// parseValue parses a value concatenation up to a field terminator.
func (p *parser) parseValue() (*astValue, error) {
	// leading whitespace after the separator is not significant
	p.skipWS()
	v := &astValue{}
	pendingWS := ""
	for {
		t := p.tok()
		switch t.kind {
		case tNL, tComma, tRBrace, tRBrack, tEOF:
			if len(v.elems) == 0 {
				return nil, p.errAt(t, "expected a value")
			}
			return v, nil
		case tWS:
			pendingWS = t.text // preserved verbatim for string concatenation
			p.advance()
			continue
		case tLBrace:
			p.advance()
			obj, err := p.parseObject()
			if err != nil {
				return nil, err
			}
			v.elems = append(v.elems, &astElem{kind: elemObj, obj: obj, wsBefore: pendingWS})
		case tLBrack:
			arr, err := p.parseArray()
			if err != nil {
				return nil, err
			}
			v.elems = append(v.elems, &astElem{kind: elemArr, arr: arr, wsBefore: pendingWS})
		case tString:
			p.advance()
			v.elems = append(v.elems, &astElem{kind: elemLit, text: t.text, quoted: true, wsBefore: pendingWS})
		case tUnquoted:
			p.advance()
			v.elems = append(v.elems, &astElem{kind: elemLit, text: t.text, wsBefore: pendingWS})
		case tSubst:
			p.advance()
			v.elems = append(v.elems, &astElem{kind: elemSubst, path: t.path, optional: t.optional, wsBefore: pendingWS})
		default:
			return nil, p.errAt(t, "unexpected token in value")
		}
		pendingWS = ""
	}
}

// parseArray parses `[ ... ]`; the opening bracket is consumed here.
func (p *parser) parseArray() ([]*astValue, error) {
	p.advance() // [
	var out []*astValue
	for {
		p.skipSeparators()
		switch p.tok().kind {
		case tRBrack:
			p.advance()
			return out, nil
		case tEOF:
			return nil, p.errAt(p.tok(), "unclosed array")
		}
		val, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		out = append(out, val)
	}
}
