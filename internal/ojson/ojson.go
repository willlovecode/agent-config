// Package ojson reads and writes JSON objects with their key order kept, so editing a
// user's settings file changes only the keys we touch.
package ojson

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// Object is a JSON object in key order. Values are *Object, []any, string, json.Number, bool or nil.
type Object struct {
	keys []string
	vals map[string]any
}

func New() *Object { return &Object{vals: map[string]any{}} }

func (o *Object) Get(k string) (any, bool) { v, ok := o.vals[k]; return v, ok }

func (o *Object) Set(k string, v any) {
	if _, ok := o.vals[k]; !ok {
		o.keys = append(o.keys, k)
	}
	o.vals[k] = v
}

// Child returns the object under k, creating it (or replacing a non-object) when needed.
func (o *Object) Child(k string) *Object {
	if c, ok := o.vals[k].(*Object); ok {
		return c
	}
	c := New()
	o.Set(k, c)
	return c
}

// List returns the array under k, or nil.
func (o *Object) List(k string) []any { l, _ := o.vals[k].([]any); return l }

func (o *Object) Keys() []string { return o.keys }

// Parse reads a JSON object; empty input (or only a BOM) gives an empty object.
func Parse(b []byte) (*Object, error) {
	b = bytes.TrimPrefix(b, []byte("\xef\xbb\xbf"))
	if len(bytes.TrimSpace(b)) == 0 {
		return New(), nil
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	v, err := decode(d)
	if err != nil {
		return nil, err
	}
	o, ok := v.(*Object)
	if !ok {
		return nil, fmt.Errorf("top level is not a JSON object")
	}
	return o, nil
}

func decode(d *json.Decoder) (any, error) {
	t, err := d.Token()
	if err != nil {
		return nil, err
	}
	switch t {
	case json.Delim('{'):
		o := New()
		for d.More() {
			kt, err := d.Token()
			if err != nil {
				return nil, err
			}
			v, err := decode(d)
			if err != nil {
				return nil, err
			}
			o.Set(kt.(string), v)
		}
		_, err := d.Token()
		return o, err
	case json.Delim('['):
		l := []any{}
		for d.More() {
			v, err := decode(d)
			if err != nil {
				return nil, err
			}
			l = append(l, v)
		}
		_, err := d.Token()
		return l, err
	}
	return t, nil
}

// Obj builds an object from key, value pairs in order; values go through FromValue.
func Obj(kv ...any) *Object {
	o := New()
	for i := 0; i+1 < len(kv); i += 2 {
		o.Set(kv[i].(string), FromValue(kv[i+1]))
	}
	return o
}

// FromValue converts plain Go values (maps sort by key) into the ojson form via a JSON round trip.
func FromValue(v any) any {
	if o, ok := v.(*Object); ok {
		return o
	}
	b, _ := json.Marshal(v)
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	out, _ := decode(d)
	return out
}

func (o *Object) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, k := range o.keys {
		if i > 0 {
			b.WriteByte(',')
		}
		kb, _ := marshal(k)
		vb, err := marshal(o.vals[k])
		if err != nil {
			return nil, err
		}
		b.Write(kb)
		b.WriteByte(':')
		b.Write(vb)
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}

func marshal(v any) ([]byte, error) {
	var b bytes.Buffer
	e := json.NewEncoder(&b)
	e.SetEscapeHTML(false)
	if err := e.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(b.Bytes(), "\n"), nil
}

// Format writes o with two-space indentation and a trailing newline.
func Format(o *Object) ([]byte, error) {
	raw, err := marshal(o)
	if err != nil {
		return nil, err
	}
	var b bytes.Buffer
	if err := json.Indent(&b, raw, "", "  "); err != nil {
		return nil, err
	}
	b.WriteByte('\n')
	return b.Bytes(), nil
}

// ContainsJSON reports whether v's compact JSON text contains the JSON fragment s.
func ContainsJSON(v any, s string) bool {
	b, _ := marshal(v)
	return strings.Contains(string(b), s)
}

// Mentions reports whether v contains the text s anywhere in its strings (or keys).
func Mentions(v any, s string) bool {
	b, _ := marshal(v)
	q, _ := marshal(s) // compare in JSON-escaped form
	return strings.Contains(string(b), string(q[1:len(q)-1]))
}
