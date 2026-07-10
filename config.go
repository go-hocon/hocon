// SPDX-License-Identifier: BSD-3-Clause
//
// Copyright (c) 2026, the go-hocon/hocon authors

package hocon

import (
	"math"
	"strconv"
	"strings"
	"time"
)

// Config is a resolved HOCON document: a tree rooted at an object.
type Config struct {
	root *ConfigValue
}

// Root returns the root object value.
func (c *Config) Root() *ConfigValue { return c.root }

// splitPath breaks a dotted path into segments.
func splitPath(path string) []string { return strings.Split(path, ".") }

// GetValue returns the [ConfigValue] at a dotted path. Each `.` in path starts
// a new object level, so `GetValue("a.b")` descends into object `a` then key
// `b`. A key that itself contains a literal dot — written quoted in HOCON, e.g.
// `"a.b" = 1` — cannot be reached this way; use [Config.GetValuePath] with the
// key as a single explicit segment (`GetValuePath("a.b")`).
func (c *Config) GetValue(path string) (*ConfigValue, error) {
	return c.getSegments(splitPath(path), path)
}

// GetValuePath returns the [ConfigValue] at a path given as explicit segments,
// bypassing dotted-path parsing. This is how literal-dot keys are addressed:
// for `"a.b" = 1`, GetValuePath("a.b") returns 1, whereas GetValue("a.b") looks
// for a nested object `a` with key `b`. Passing several segments descends the
// tree segment by segment (GetValuePath("a", "b") == GetValue("a.b")).
func (c *Config) GetValuePath(segments ...string) (*ConfigValue, error) {
	return c.getSegments(segments, strings.Join(segments, "."))
}

// getSegments walks the object tree following the given segments verbatim.
func (c *Config) getSegments(segments []string, path string) (*ConfigValue, error) {
	cur := c.root
	for _, seg := range segments {
		if cur.typ != ObjectType {
			return nil, &PathError{Path: path, Err: ErrMissing}
		}
		v, ok := cur.obj[seg]
		if !ok {
			return nil, &PathError{Path: path, Err: ErrMissing}
		}
		cur = v
	}
	return cur, nil
}

// HasPath reports whether a value exists at the dotted path.
func (c *Config) HasPath(path string) bool {
	_, err := c.GetValue(path)
	return err == nil
}

// HasPathSegments reports whether a value exists at the given explicit path
// segments (the segmented analogue of [Config.HasPath]).
func (c *Config) HasPathSegments(segments ...string) bool {
	_, err := c.GetValuePath(segments...)
	return err == nil
}

func (c *Config) typed(path string, want ConfigValueType) (*ConfigValue, error) {
	v, err := c.GetValue(path)
	if err != nil {
		return nil, err
	}
	if v.typ != want {
		return nil, &PathError{Path: path, Err: ErrWrongType}
	}
	return v, nil
}

// GetString returns the string at path.
func (c *Config) GetString(path string) (string, error) {
	v, err := c.typed(path, StringType)
	if err != nil {
		return "", err
	}
	return v.str, nil
}

// GetBool returns the boolean at path.
func (c *Config) GetBool(path string) (bool, error) {
	v, err := c.typed(path, BooleanType)
	if err != nil {
		return false, err
	}
	return v.b, nil
}

// GetFloat returns the number at path as a float64.
func (c *Config) GetFloat(path string) (float64, error) {
	v, err := c.typed(path, NumberType)
	if err != nil {
		return 0, err
	}
	return v.num, nil
}

// GetInt returns the number at path truncated to an int64.
func (c *Config) GetInt(path string) (int64, error) {
	v, err := c.typed(path, NumberType)
	if err != nil {
		return 0, err
	}
	return int64(v.num), nil
}

// GetList returns the array elements at path.
func (c *Config) GetList(path string) ([]*ConfigValue, error) {
	v, err := c.typed(path, ArrayType)
	if err != nil {
		return nil, err
	}
	return v.arr, nil
}

// GetObject returns the object at path wrapped in its own [Config].
func (c *Config) GetObject(path string) (*Config, error) {
	v, err := c.typed(path, ObjectType)
	if err != nil {
		return nil, err
	}
	return &Config{root: v}, nil
}

// GetOrElse returns the value at path, or def when the path is absent.
func (c *Config) GetOrElse(path string, def *ConfigValue) *ConfigValue {
	v, err := c.GetValue(path)
	if err != nil {
		return def
	}
	return v
}

// GetDuration returns the duration at path. A bare number is milliseconds; a
// string may carry a unit suffix (`10s`, `5 min`, `100ms`, ...).
func (c *Config) GetDuration(path string) (time.Duration, error) {
	v, err := c.GetValue(path)
	if err != nil {
		return 0, err
	}
	switch v.typ {
	case NumberType:
		return time.Duration(v.num * float64(time.Millisecond)), nil
	case StringType:
		d, perr := parseDuration(v.str)
		if perr != nil {
			return 0, &PathError{Path: path, Err: perr}
		}
		return d, nil
	default:
		return 0, &PathError{Path: path, Err: ErrWrongType}
	}
}

// GetBytes returns a size in bytes at path. A bare number is a byte count; a
// string may carry a unit suffix (`10MB`, `1GiB`, `512 bytes`, ...).
func (c *Config) GetBytes(path string) (int64, error) {
	v, err := c.GetValue(path)
	if err != nil {
		return 0, err
	}
	switch v.typ {
	case NumberType:
		return int64(v.num), nil
	case StringType:
		n, perr := parseBytes(v.str)
		if perr != nil {
			return 0, &PathError{Path: path, Err: perr}
		}
		return n, nil
	default:
		return 0, &PathError{Path: path, Err: ErrWrongType}
	}
}

// Render serialises the config using the given options.
func (c *Config) Render(opts RenderOptions) string { return c.root.render(opts) }

var durationUnits = map[string]time.Duration{
	"":             time.Millisecond,
	"ns":           time.Nanosecond,
	"nano":         time.Nanosecond,
	"nanos":        time.Nanosecond,
	"nanosecond":   time.Nanosecond,
	"nanoseconds":  time.Nanosecond,
	"us":           time.Microsecond,
	"micro":        time.Microsecond,
	"micros":       time.Microsecond,
	"microsecond":  time.Microsecond,
	"microseconds": time.Microsecond,
	"ms":           time.Millisecond,
	"milli":        time.Millisecond,
	"millis":       time.Millisecond,
	"millisecond":  time.Millisecond,
	"milliseconds": time.Millisecond,
	"s":            time.Second,
	"sec":          time.Second,
	"secs":         time.Second,
	"second":       time.Second,
	"seconds":      time.Second,
	"m":            time.Minute,
	"min":          time.Minute,
	"mins":         time.Minute,
	"minute":       time.Minute,
	"minutes":      time.Minute,
	"h":            time.Hour,
	"hour":         time.Hour,
	"hours":        time.Hour,
	"d":            24 * time.Hour,
	"day":          24 * time.Hour,
	"days":         24 * time.Hour,
}

var byteUnits = map[string]int64{
	"":      1,
	"b":     1,
	"byte":  1,
	"bytes": 1,
	"kb":    1e3, "kilobyte": 1e3, "kilobytes": 1e3,
	"mb": 1e6, "megabyte": 1e6, "megabytes": 1e6,
	"gb": 1e9, "gigabyte": 1e9, "gigabytes": 1e9,
	"tb": 1e12, "terabyte": 1e12, "terabytes": 1e12,
	"pb": 1e15, "petabyte": 1e15, "petabytes": 1e15,
	"k": 1 << 10, "ki": 1 << 10, "kib": 1 << 10, "kibibyte": 1 << 10, "kibibytes": 1 << 10,
	"m": 1 << 20, "mi": 1 << 20, "mib": 1 << 20, "mebibyte": 1 << 20, "mebibytes": 1 << 20,
	"g": 1 << 30, "gi": 1 << 30, "gib": 1 << 30, "gibibyte": 1 << 30, "gibibytes": 1 << 30,
	"t": 1 << 40, "ti": 1 << 40, "tib": 1 << 40, "tebibyte": 1 << 40, "tebibytes": 1 << 40,
	"p": 1 << 50, "pi": 1 << 50, "pib": 1 << 50, "pebibyte": 1 << 50, "pebibytes": 1 << 50,
}

// splitNumUnit separates the leading numeric literal from a trailing unit.
func splitNumUnit(s string) (num float64, unit string, err error) {
	s = strings.TrimSpace(s)
	i := 0
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		i++
	}
	start := i
	seenDot := false
	for i < len(s) {
		c := s[i]
		if c >= '0' && c <= '9' {
			i++
			continue
		}
		if c == '.' && !seenDot {
			seenDot = true
			i++
			continue
		}
		break
	}
	if i == start {
		return 0, "", &ParseError{Msg: "no numeric prefix in unit value " + strconv.Quote(s)}
	}
	num, err = strconv.ParseFloat(s[:i], 64)
	if err != nil {
		return 0, "", &ParseError{Msg: "invalid number in unit value " + strconv.Quote(s)}
	}
	return num, strings.ToLower(strings.TrimSpace(s[i:])), nil
}

func parseDuration(s string) (time.Duration, error) {
	num, unit, err := splitNumUnit(s)
	if err != nil {
		return 0, err
	}
	mult, ok := durationUnits[unit]
	if !ok {
		return 0, &ParseError{Msg: "unknown duration unit " + strconv.Quote(unit)}
	}
	return time.Duration(num * float64(mult)), nil
}

func parseBytes(s string) (int64, error) {
	num, unit, err := splitNumUnit(s)
	if err != nil {
		return 0, err
	}
	mult, ok := byteUnits[unit]
	if !ok {
		return 0, &ParseError{Msg: "unknown size unit " + strconv.Quote(unit)}
	}
	return int64(math.Round(num * float64(mult))), nil
}
