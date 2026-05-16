package gbxmockserver

import (
	"reflect"
	"strings"
	"time"
)

// ValueKind enumerates the XML-RPC value tags supported by the mock.
type ValueKind int

const (
	KindNil ValueKind = iota
	KindString
	KindInt
	KindDouble
	KindBool
	KindDateTime
	KindBase64
	KindArray
	KindStruct
)

func (k ValueKind) String() string {
	switch k {
	case KindNil:
		return "nil"
	case KindString:
		return "string"
	case KindInt:
		return "int"
	case KindDouble:
		return "double"
	case KindBool:
		return "bool"
	case KindDateTime:
		return "dateTime"
	case KindBase64:
		return "base64"
	case KindArray:
		return "array"
	case KindStruct:
		return "struct"
	}
	return "unknown"
}

// Value is a typed XML-RPC value. It is the common currency between the
// codec, the response store, and recorded MethodCall params.
type Value struct {
	Kind ValueKind
	str  string
	i    int64
	f    float64
	b    bool
	t    time.Time
	bin  []byte
	arr  []Value
	obj  map[string]Value
}

func NilValue() Value                 { return Value{Kind: KindNil} }
func StringValue(s string) Value      { return Value{Kind: KindString, str: s} }
func IntValue(i int64) Value          { return Value{Kind: KindInt, i: i} }
func DoubleValue(f float64) Value     { return Value{Kind: KindDouble, f: f} }
func BoolValue(b bool) Value          { return Value{Kind: KindBool, b: b} }
func DateTimeValue(t time.Time) Value { return Value{Kind: KindDateTime, t: t} }
func Base64Value(b []byte) Value      { return Value{Kind: KindBase64, bin: b} }
func ArrayValue(items ...Value) Value { return Value{Kind: KindArray, arr: items} }
func StructValue(m map[string]Value) Value {
	return Value{Kind: KindStruct, obj: m}
}

func (v Value) AsString() (string, bool)           { return v.str, v.Kind == KindString }
func (v Value) AsInt() (int64, bool)               { return v.i, v.Kind == KindInt }
func (v Value) AsDouble() (float64, bool)          { return v.f, v.Kind == KindDouble }
func (v Value) AsBool() (bool, bool)               { return v.b, v.Kind == KindBool }
func (v Value) AsDateTime() (time.Time, bool)      { return v.t, v.Kind == KindDateTime }
func (v Value) AsBase64() ([]byte, bool)           { return v.bin, v.Kind == KindBase64 }
func (v Value) AsArray() ([]Value, bool)           { return v.arr, v.Kind == KindArray }
func (v Value) AsStruct() (map[string]Value, bool) { return v.obj, v.Kind == KindStruct }

// IsNil reports whether v is the explicit Nil kind.
func (v Value) IsNil() bool { return v.Kind == KindNil }

// ValueOf converts an arbitrary Go value into a Value. It handles the common
// scalar types, []byte (→ base64), time.Time (→ dateTime), slices/arrays
// (→ array), maps with string keys (→ struct), and structs walked by their
// `xmlrpc:"FieldName"` tag (with the field's Go name as fallback). A nil
// pointer or untyped nil maps to KindNil.
//
// Passing a Value back returns it unchanged so callers can mix raw values
// with typed ones in `args ...any` lists.
func ValueOf(v any) Value {
	if v == nil {
		return NilValue()
	}
	if val, ok := v.(Value); ok {
		return val
	}
	if t, ok := v.(time.Time); ok {
		return DateTimeValue(t)
	}
	if b, ok := v.([]byte); ok {
		return Base64Value(b)
	}

	rv := reflect.ValueOf(v)
	return valueFromReflect(rv)
}

func valueFromReflect(rv reflect.Value) Value {
	if !rv.IsValid() {
		return NilValue()
	}

	switch rv.Kind() {
	case reflect.Ptr, reflect.Interface:
		if rv.IsNil() {
			return NilValue()
		}
		return valueFromReflect(rv.Elem())

	case reflect.Bool:
		return BoolValue(rv.Bool())

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return IntValue(rv.Int())

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		u := rv.Uint()
		if u > 1<<63-1 {
			u = 1<<63 - 1
		}
		return IntValue(int64(u))

	case reflect.Float32, reflect.Float64:
		return DoubleValue(rv.Float())

	case reflect.String:
		return StringValue(rv.String())

	case reflect.Slice:
		if rv.Type().Elem().Kind() == reflect.Uint8 {
			return Base64Value(rv.Bytes())
		}
		fallthrough
	case reflect.Array:
		items := make([]Value, rv.Len())
		for i := 0; i < rv.Len(); i++ {
			items[i] = valueFromReflect(rv.Index(i))
		}
		return ArrayValue(items...)

	case reflect.Map:
		if rv.Type().Key().Kind() != reflect.String {
			return NilValue()
		}
		out := make(map[string]Value, rv.Len())
		iter := rv.MapRange()
		for iter.Next() {
			out[iter.Key().String()] = valueFromReflect(iter.Value())
		}
		return StructValue(out)

	case reflect.Struct:
		if t, ok := rv.Interface().(time.Time); ok {
			return DateTimeValue(t)
		}
		out := make(map[string]Value)
		t := rv.Type()
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			if !f.IsExported() {
				continue
			}
			name, omitempty, skip := parseXmlrpcTag(f)
			if skip {
				continue
			}
			fv := rv.Field(i)
			if omitempty && fv.IsZero() {
				continue
			}
			out[name] = valueFromReflect(fv)
		}
		return StructValue(out)
	}

	return NilValue()
}

func parseXmlrpcTag(f reflect.StructField) (name string, omitempty bool, skip bool) {
	tag := f.Tag.Get("xmlrpc")
	if tag == "-" {
		return "", false, true
	}
	if tag == "" {
		return f.Name, false, false
	}
	parts := strings.Split(tag, ",")
	name = parts[0]
	if name == "" {
		name = f.Name
	}
	for _, p := range parts[1:] {
		if p == "omitempty" {
			omitempty = true
		}
	}
	return name, omitempty, false
}
