// SPDX-License-Identifier: BSD-3-Clause
//
// Copyright (c) 2026, the go-hocon/hocon authors

package hocon

import (
	"strconv"
	"strings"
)

type tokKind int

const (
	tEOF tokKind = iota
	tLBrace
	tRBrace
	tLBrack
	tRBrack
	tColon
	tEquals
	tPlusEq
	tComma
	tNL
	tWS
	tString
	tSubst
	tUnquoted
	tReserved // a character reserved by the spec that may not start a value/key
)

type token struct {
	kind     tokKind
	text     string
	path     []string
	optional bool
	line     int
	col      int
}

type lexer struct {
	src  string
	pos  int
	line int
	col  int
}

func newLexer(src string) *lexer { return &lexer{src: src, line: 1, col: 1} }

func (l *lexer) eof() bool { return l.pos >= len(l.src) }

func (l *lexer) cur() byte { return l.src[l.pos] }

func (l *lexer) peek() byte {
	if l.pos+1 < len(l.src) {
		return l.src[l.pos+1]
	}
	return 0
}

// advance consumes one byte, updating the line/column counters.
func (l *lexer) advance() byte {
	c := l.src[l.pos]
	l.pos++
	if c == '\n' {
		l.line++
		l.col = 1
	} else {
		l.col++
	}
	return c
}

// isReserved reports whether c is a HOCON "forbidden character" that carries no
// structural meaning of its own: it may neither appear inside an unquoted string
// nor begin a value or key. Per the spec these are reserved for future use.
func isReserved(c byte) bool {
	switch c {
	case '`', '^', '?', '!', '@', '*', '&', '\\':
		return true
	default:
		return false
	}
}

func isUnquotedStop(c, next byte) bool {
	switch c {
	case ' ', '\t', '\r', '\n', '$', '"', '{', '}', '[', ']', ':', '=', ',', '+', '#',
		'`', '^', '?', '!', '@', '*', '&', '\\':
		return true
	case '/':
		return next == '/'
	default:
		return false
	}
}

// lex tokenises the whole input.
func lex(src string) ([]token, error) {
	l := newLexer(src)
	var toks []token
	for {
		tok, err := l.next()
		if err != nil {
			return nil, err
		}
		toks = append(toks, tok)
		if tok.kind == tEOF {
			return toks, nil
		}
	}
}

func (l *lexer) next() (token, error) {
	for {
		if l.eof() {
			return token{kind: tEOF, line: l.line, col: l.col}, nil
		}
		line, col := l.line, l.col
		c := l.cur()
		switch {
		case c == ' ' || c == '\t' || c == '\r':
			var ws strings.Builder
			ws.WriteByte(l.advance())
			for !l.eof() && (l.cur() == ' ' || l.cur() == '\t' || l.cur() == '\r') {
				ws.WriteByte(l.advance())
			}
			// The HOCON spec requires whitespace *between* simple values in a
			// value (or path) concatenation to be preserved verbatim, so the
			// token carries the exact run rather than a single space.
			return token{kind: tWS, text: ws.String(), line: line, col: col}, nil
		case c == '\n':
			l.advance()
			return token{kind: tNL, line: line, col: col}, nil
		case c == '#':
			l.skipLineComment()
			continue
		case c == '/' && l.peek() == '/':
			l.skipLineComment()
			continue
		case c == '{':
			l.advance()
			return token{kind: tLBrace, line: line, col: col}, nil
		case c == '}':
			l.advance()
			return token{kind: tRBrace, line: line, col: col}, nil
		case c == '[':
			l.advance()
			return token{kind: tLBrack, line: line, col: col}, nil
		case c == ']':
			l.advance()
			return token{kind: tRBrack, line: line, col: col}, nil
		case c == ':':
			l.advance()
			return token{kind: tColon, line: line, col: col}, nil
		case c == ',':
			l.advance()
			return token{kind: tComma, line: line, col: col}, nil
		case c == '=':
			l.advance()
			return token{kind: tEquals, line: line, col: col}, nil
		case c == '+':
			if l.peek() == '=' {
				l.advance()
				l.advance()
				return token{kind: tPlusEq, line: line, col: col}, nil
			}
			// A lone '+' is a forbidden character, not the start of a value.
			l.advance()
			return token{kind: tReserved, text: "+", line: line, col: col}, nil
		case c == '"':
			return l.lexString()
		case c == '$':
			return l.lexSubst()
		case isReserved(c):
			l.advance()
			return token{kind: tReserved, text: string(c), line: line, col: col}, nil
		default:
			return l.lexUnquoted()
		}
	}
}

func (l *lexer) skipLineComment() {
	for !l.eof() && l.cur() != '\n' {
		l.advance()
	}
}

func (l *lexer) lexUnquoted() (token, error) {
	line, col := l.line, l.col
	var b strings.Builder
	// The first byte always belongs to the token: next() dispatches here only
	// at a genuine token start (which for '+' is itself a stop character).
	b.WriteByte(l.advance())
	for !l.eof() {
		c := l.cur()
		if isUnquotedStop(c, l.peek()) {
			break
		}
		b.WriteByte(l.advance())
	}
	return token{kind: tUnquoted, text: b.String(), line: line, col: col}, nil
}

func (l *lexer) lexString() (token, error) {
	line, col := l.line, l.col
	// Triple-quoted multi-line string.
	if l.peek() == '"' && l.pos+2 < len(l.src) && l.src[l.pos+2] == '"' {
		return l.lexTripleString(line, col)
	}
	l.advance() // opening quote
	var b strings.Builder
	for {
		if l.eof() {
			return token{}, &ParseError{Msg: "unterminated string", Line: line, Col: col}
		}
		c := l.cur()
		if c == '\n' {
			return token{}, &ParseError{Msg: "unterminated string", Line: line, Col: col}
		}
		if c == '"' {
			l.advance()
			return token{kind: tString, text: b.String(), line: line, col: col}, nil
		}
		if c == '\\' {
			if err := l.lexEscape(&b, line, col); err != nil {
				return token{}, err
			}
			continue
		}
		b.WriteByte(l.advance())
	}
}

func (l *lexer) lexEscape(b *strings.Builder, line, col int) error {
	l.advance() // backslash
	if l.eof() {
		return &ParseError{Msg: "unterminated escape", Line: line, Col: col}
	}
	c := l.advance()
	switch c {
	case '"':
		b.WriteByte('"')
	case '\\':
		b.WriteByte('\\')
	case '/':
		b.WriteByte('/')
	case 'n':
		b.WriteByte('\n')
	case 't':
		b.WriteByte('\t')
	case 'r':
		b.WriteByte('\r')
	case 'b':
		b.WriteByte('\b')
	case 'f':
		b.WriteByte('\f')
	case 'u':
		return l.lexUnicodeEscape(b, line, col)
	default:
		return &ParseError{Msg: "invalid escape \\" + string(c), Line: line, Col: col}
	}
	return nil
}

func (l *lexer) lexUnicodeEscape(b *strings.Builder, line, col int) error {
	if l.pos+4 > len(l.src) {
		return &ParseError{Msg: "truncated \\u escape", Line: line, Col: col}
	}
	hex := l.src[l.pos : l.pos+4]
	n, err := strconv.ParseUint(hex, 16, 32)
	if err != nil {
		return &ParseError{Msg: "invalid \\u escape", Line: line, Col: col}
	}
	for range 4 {
		l.advance()
	}
	b.WriteRune(rune(n))
	return nil
}

func (l *lexer) lexTripleString(line, col int) (token, error) {
	l.advance()
	l.advance()
	l.advance() // three opening quotes
	start := l.pos
	for {
		if l.eof() {
			return token{}, &ParseError{Msg: "unterminated triple-quoted string", Line: line, Col: col}
		}
		if l.cur() == '"' && l.peek() == '"' && l.pos+2 < len(l.src) && l.src[l.pos+2] == '"' {
			text := l.src[start:l.pos]
			l.advance()
			l.advance()
			l.advance()
			return token{kind: tString, text: text, line: line, col: col}, nil
		}
		l.advance()
	}
}

func (l *lexer) lexSubst() (token, error) {
	line, col := l.line, l.col
	l.advance() // $
	if l.eof() || l.cur() != '{' {
		return token{}, &ParseError{Msg: "expected '{' after '$'", Line: line, Col: col}
	}
	l.advance() // {
	optional := false
	if !l.eof() && l.cur() == '?' {
		optional = true
		l.advance()
	}
	var b strings.Builder
	for {
		if l.eof() {
			return token{}, &ParseError{Msg: "unterminated substitution", Line: line, Col: col}
		}
		c := l.cur()
		if c == '}' {
			l.advance()
			break
		}
		if c == '\n' {
			return token{}, &ParseError{Msg: "unterminated substitution", Line: line, Col: col}
		}
		b.WriteByte(l.advance())
	}
	raw := strings.TrimSpace(b.String())
	if raw == "" {
		return token{}, &ParseError{Msg: "empty substitution path", Line: line, Col: col}
	}
	segs := strings.Split(raw, ".")
	for _, s := range segs {
		if s == "" {
			return token{}, &ParseError{Msg: "empty substitution segment", Line: line, Col: col}
		}
	}
	return token{kind: tSubst, path: segs, optional: optional, line: line, col: col}, nil
}
