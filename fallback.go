// SPDX-License-Identifier: BSD-3-Clause
//
// Copyright (c) 2026, the go-hocon/hocon authors

package hocon

// WithFallback returns a new [Config] in which values missing from c are taken
// from fallback. Two objects at the same path deep-merge with c winning;
// anything else in c overrides the fallback outright. Neither input is mutated.
func (c *Config) WithFallback(fallback *Config) *Config {
	return &Config{root: deepMergeValue(fallback.root, c.root)}
}

// deepMergeValue merges over onto base, over winning; objects merge recursively.
func deepMergeValue(base, over *ConfigValue) *ConfigValue {
	if base.typ != ObjectType || over.typ != ObjectType {
		return over
	}
	out := NewObject()
	for k, v := range base.obj {
		out.obj[k] = v
	}
	for k, v := range over.obj {
		if bv, ok := out.obj[k]; ok {
			out.obj[k] = deepMergeValue(bv, v)
		} else {
			out.obj[k] = v
		}
	}
	return out
}
