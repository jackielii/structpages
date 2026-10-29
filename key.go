package structpages

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// keySep joins a component id and an instance key. Generated ids never
// contain it (see idSegment), so the first one in an id ends the
// component id.
const keySep = "--"

// Key generates the raw HTML id of one instance of a component rendered
// once per item, such as a table row: the component's id (as [ID]
// generates it), "--", and key with every byte outside [A-Za-z0-9-]
// written as _XX ("_" included). Use it where a component is rendered
// more than once on a page, so each instance has its own id.
//
// Like React's key, the key is the item's identity, unique among the
// component's instances. Unlike React's, it comes back: with
// HTMXRenderTarget or HTMXv4RenderTarget, a request targeting the
// instance Is the component, and [KeyOf] returns the key, so Props can
// load just that item:
//
//	// <tr id={ Page.Row |> key(row.ID.String()) }>
//	func (p Page) Props(t structpages.RenderTarget, s *Store) (Row, error) {
//	    if t.Is(p.Row) {
//	        return s.Row(structpages.KeyOf(t)) // check the caller may see it
//	    }
//	    …
//	}
//
// The key is client input, like a path parameter: Props must check the
// caller may see the instance it names. An empty key is an error; use ID
// for a component's single instance. v takes the forms ID takes except a
// plain string, which names no component.
func Key(ctx context.Context, v any, key string) (string, error) {
	if err := checkKeyArgs(v, key); err != nil {
		return "", err
	}
	id, err := ID(ctx, v)
	if err != nil {
		return "", err
	}
	return id + keySep + escapeKey(key), nil
}

// KeyTarget is [Key] as a CSS selector (with "#" prefix), for hx-target.
func KeyTarget(ctx context.Context, v any, key string) (string, error) {
	id, err := Key(ctx, v, key)
	if err != nil {
		return "", err
	}
	return "#" + id, nil
}

// checkKeyArgs rejects what Key can't make a keyed id of: a plain string,
// which ID passes through as a literal id rather than naming a component,
// and an empty key.
func checkKeyArgs(v any, key string) error {
	if s, ok := v.(string); ok {
		return fmt.Errorf("structpages: Key takes a component, not the string %q", s)
	}
	if key == "" {
		return errors.New("structpages: empty key - use ID for a component's single instance")
	}
	return nil
}

// KeyOf returns the key of the instance the request targets: the key a
// keyed id was generated with ([Key]), or "" when the target id isn't
// keyed. Read it once t.Is has named the component: an id of keyed form
// that names no component Is nothing, but still carries its key.
//
// The htmx selectors' targets carry it. A custom RenderTarget carries a
// key by having a Key() string method; one that wraps another target
// passes its key through with KeyOf:
//
//	func (t myTarget) Key() string { return structpages.KeyOf(t.inner) }
func KeyOf(t RenderTarget) string {
	// The built-in targets have no Key method, so Props, which takes the
	// target at its dynamic type, never hands one to a parameter asking
	// for Key() string.
	switch t := t.(type) {
	case *methodRenderTarget:
		return t.key
	case *htmxUnmatchedTarget:
		return t.key
	case keyedTarget:
		return t.Key()
	}
	return ""
}

// keyedTarget is a RenderTarget that carries the key of the instance it
// targets.
type keyedTarget interface {
	Key() string
}

// escapeKey keeps [A-Za-z0-9-] and writes any other byte as _XX in
// lowercase hex, so the id needs no CSS escaping and unescapeKey
// recovers the key.
func escapeKey(key string) string {
	const hex = "0123456789abcdef"
	var b strings.Builder
	b.Grow(len(key))
	for i := 0; i < len(key); i++ {
		c := key[i]
		if keyByte(c) {
			b.WriteByte(c)
			continue
		}
		b.WriteByte('_')
		b.WriteByte(hex[c>>4])
		b.WriteByte(hex[c&0xf])
	}
	return b.String()
}

// unescapeKey reverses escapeKey. It accepts only what escapeKey
// produces, so a key has exactly one id; ok is false otherwise, and for
// the empty string, which no key escapes to.
func unescapeKey(s string) (key string, ok bool) {
	if s == "" {
		return "", false
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case keyByte(c):
			b.WriteByte(c)
		case c == '_' && i+2 < len(s):
			hi, ok1 := lowerHex(s[i+1])
			lo, ok2 := lowerHex(s[i+2])
			if !ok1 || !ok2 || keyByte(hi<<4|lo) {
				return "", false
			}
			b.WriteByte(hi<<4 | lo)
			i += 2
		default:
			return "", false
		}
	}
	return b.String(), true
}

// keyByte reports whether escapeKey keeps c as is.
func keyByte(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-'
}

func lowerHex(c byte) (byte, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	}
	return 0, false
}

// splitKeyedID splits a keyed id into the component id and the key. ok
// is false when id isn't one Key could have generated.
func splitKeyedID(id string) (componentID, key string, ok bool) {
	componentID, esc, found := strings.Cut(id, keySep)
	if !found || componentID == "" {
		return "", "", false
	}
	key, ok = unescapeKey(esc)
	return componentID, key, ok
}
