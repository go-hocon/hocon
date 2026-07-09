// SPDX-License-Identifier: BSD-3-Clause
//
// Copyright (c) 2026, the go-hocon/hocon authors

package hocon

import "strings"

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

// parseKey reads a (possibly dotted) key up to the separator.
func (p *parser) parseKey() ([]string, token, error) {
	first := p.tok()
	var segs []string
	cur := ""
	started := false
	for {
		t := p.tok()
		switch t.kind {
		case tUnquoted:
			started = true
			parts := strings.Split(t.text, ".")
			cur += parts[0]
			for _, extra := range parts[1:] {
				segs = append(segs, cur)
				cur = extra
			}
			p.advance()
		case tString:
			started = true
			cur += t.text
			p.advance()
		default:
			if !started {
				return nil, first, p.errAt(t, "expected a key")
			}
			segs = append(segs, cur)
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
	kind, name, err := p.parseIncludeTarget()
	if err != nil {
		return err
	}
	content, err := p.include(kind, name)
	if err != nil {
		return &ParseError{Msg: "include " + kind + "(" + name + "): " + err.Error()}
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
	o.mergeFrom(inc)
	return nil
}

// parseIncludeTarget reads `"file.conf"` (bare, treated as a file) or
// `kind("...")` where kind is one of file, url or classpath. Because the lexer
// keeps '(' and ')' inside unquoted runs, the qualifier arrives as `kind(` and
// the closing paren as a `)` token after the quoted string.
func (p *parser) parseIncludeTarget() (kind, name string, err error) {
	t := p.tok()
	if t.kind == tString {
		p.advance()
		return "file", t.text, nil
	}
	if t.kind != tUnquoted || !strings.HasSuffix(t.text, "(") {
		return "", "", p.errAt(t, "expected include target")
	}
	kind = strings.TrimSuffix(t.text, "(")
	switch kind {
	case "file", "url", "classpath":
	case "required":
		return "", "", p.errAt(t, "required(...) includes are not supported")
	default:
		return "", "", p.errAt(t, "unknown include qualifier "+kind)
	}
	p.advance()
	st := p.tok()
	if st.kind != tString {
		return "", "", p.errAt(st, "expected quoted string in "+kind+"(...)")
	}
	p.advance()
	ct := p.tok()
	if ct.kind != tUnquoted || !strings.HasPrefix(ct.text, ")") {
		return "", "", p.errAt(ct, "expected ')' after include target")
	}
	p.advance()
	return kind, st.text, nil
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
			pendingWS = " "
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
