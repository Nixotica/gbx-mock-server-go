package gbxmockserver

import (
	"bytes"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const xmlrpcDateTimeLayout = "2006-01-02T15:04:05"

type xmlValue struct {
	String   *string    `xml:"string"`
	Int      *string    `xml:"int"`
	I4       *string    `xml:"i4"`
	I8       *string    `xml:"i8"`
	Boolean  *string    `xml:"boolean"`
	Double   *string    `xml:"double"`
	Base64   *string    `xml:"base64"`
	DateTime *string    `xml:"dateTime.iso8601"`
	Nil      *struct{}  `xml:"nil"`
	Array    *xmlArray  `xml:"array"`
	Struct   *xmlStruct `xml:"struct"`
	CharData string     `xml:",chardata"`
}

type xmlArray struct {
	Data struct {
		Values []xmlValue `xml:"value"`
	} `xml:"data"`
}

type xmlStruct struct {
	Members []xmlMember `xml:"member"`
}

type xmlMember struct {
	Name  string   `xml:"name"`
	Value xmlValue `xml:"value"`
}

type xmlMethodCall struct {
	XMLName    xml.Name `xml:"methodCall"`
	MethodName string   `xml:"methodName"`
	Params     struct {
		Params []struct {
			Value xmlValue `xml:"value"`
		} `xml:"param"`
	} `xml:"params"`
}

// parseMethodCall decodes the XML payload of a <methodCall> frame. Params
// always returns a non-nil slice (possibly empty) so callers don't need a
// nil check.
func parseMethodCall(data []byte) (method string, params []Value, err error) {
	var mc xmlMethodCall
	if err = xml.Unmarshal(data, &mc); err != nil {
		return "", nil, fmt.Errorf("parse methodCall: %w", err)
	}
	out := make([]Value, 0, len(mc.Params.Params))
	for i, p := range mc.Params.Params {
		v, perr := valueFromXML(p.Value)
		if perr != nil {
			return "", nil, fmt.Errorf("parse param %d: %w", i, perr)
		}
		out = append(out, v)
	}
	return mc.MethodName, out, nil
}

func valueFromXML(xv xmlValue) (Value, error) {
	switch {
	case xv.Nil != nil:
		return NilValue(), nil
	case xv.String != nil:
		return StringValue(*xv.String), nil
	case xv.I4 != nil:
		return intFromText(*xv.I4)
	case xv.Int != nil:
		return intFromText(*xv.Int)
	case xv.I8 != nil:
		return intFromText(*xv.I8)
	case xv.Boolean != nil:
		s := strings.TrimSpace(*xv.Boolean)
		return BoolValue(s == "1" || strings.EqualFold(s, "true")), nil
	case xv.Double != nil:
		f, err := strconv.ParseFloat(strings.TrimSpace(*xv.Double), 64)
		if err != nil {
			return Value{}, fmt.Errorf("parse double %q: %w", *xv.Double, err)
		}
		return DoubleValue(f), nil
	case xv.Base64 != nil:
		raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(*xv.Base64))
		if err != nil {
			return Value{}, fmt.Errorf("parse base64: %w", err)
		}
		return Base64Value(raw), nil
	case xv.DateTime != nil:
		t, err := parseDateTime(*xv.DateTime)
		if err != nil {
			return Value{}, err
		}
		return DateTimeValue(t), nil
	case xv.Array != nil:
		items := make([]Value, len(xv.Array.Data.Values))
		for i, inner := range xv.Array.Data.Values {
			v, err := valueFromXML(inner)
			if err != nil {
				return Value{}, err
			}
			items[i] = v
		}
		return ArrayValue(items...), nil
	case xv.Struct != nil:
		out := make(map[string]Value, len(xv.Struct.Members))
		for _, m := range xv.Struct.Members {
			v, err := valueFromXML(m.Value)
			if err != nil {
				return Value{}, err
			}
			out[m.Name] = v
		}
		return StructValue(out), nil
	default:
		return StringValue(xv.CharData), nil
	}
}

func intFromText(s string) (Value, error) {
	i, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil {
		return Value{}, fmt.Errorf("parse int %q: %w", s, err)
	}
	return IntValue(i), nil
}

func parseDateTime(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	for _, layout := range []string{
		xmlrpcDateTimeLayout,
		time.RFC3339,
		"20060102T15:04:05",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("parse dateTime %q", s)
}

// encodeMethodResponse renders a successful <methodResponse> wrapping v.
func encodeMethodResponse(v Value) []byte {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<methodResponse><params><param>`)
	writeValue(&b, v)
	b.WriteString(`</param></params></methodResponse>`)
	return []byte(b.String())
}

// encodeFault renders a <methodResponse><fault>...</fault></methodResponse>
// with the given code and message.
func encodeFault(code int, message string) []byte {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<methodResponse><fault>`)
	writeValue(&b, StructValue(map[string]Value{
		"faultCode":   IntValue(int64(code)),
		"faultString": StringValue(message),
	}))
	b.WriteString(`</fault></methodResponse>`)
	return []byte(b.String())
}

// encodeMethodCall renders a <methodCall> frame, used for server-pushed
// callbacks. Params come through ValueOf so callers can pass mixed types.
func encodeMethodCall(method string, params []Value) []byte {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<methodCall><methodName>`)
	writeEscaped(&b, method)
	b.WriteString(`</methodName><params>`)
	for _, p := range params {
		b.WriteString(`<param>`)
		writeValue(&b, p)
		b.WriteString(`</param>`)
	}
	b.WriteString(`</params></methodCall>`)
	return []byte(b.String())
}

func writeValue(b *strings.Builder, v Value) {
	b.WriteString(`<value>`)
	switch v.Kind {
	case KindNil:
		b.WriteString(`<nil/>`)
	case KindString:
		b.WriteString(`<string>`)
		writeEscaped(b, v.str)
		b.WriteString(`</string>`)
	case KindInt:
		b.WriteString(`<i4>`)
		b.WriteString(strconv.FormatInt(v.i, 10))
		b.WriteString(`</i4>`)
	case KindDouble:
		b.WriteString(`<double>`)
		b.WriteString(strconv.FormatFloat(v.f, 'f', -1, 64))
		b.WriteString(`</double>`)
	case KindBool:
		if v.b {
			b.WriteString(`<boolean>1</boolean>`)
		} else {
			b.WriteString(`<boolean>0</boolean>`)
		}
	case KindDateTime:
		b.WriteString(`<dateTime.iso8601>`)
		b.WriteString(v.t.UTC().Format(xmlrpcDateTimeLayout))
		b.WriteString(`</dateTime.iso8601>`)
	case KindBase64:
		b.WriteString(`<base64>`)
		b.WriteString(base64.StdEncoding.EncodeToString(v.bin))
		b.WriteString(`</base64>`)
	case KindArray:
		b.WriteString(`<array><data>`)
		for _, item := range v.arr {
			writeValue(b, item)
		}
		b.WriteString(`</data></array>`)
	case KindStruct:
		b.WriteString(`<struct>`)
		for name, member := range v.obj {
			b.WriteString(`<member><name>`)
			writeEscaped(b, name)
			b.WriteString(`</name>`)
			writeValue(b, member)
			b.WriteString(`</member>`)
		}
		b.WriteString(`</struct>`)
	}
	b.WriteString(`</value>`)
}

func writeEscaped(b *strings.Builder, s string) {
	var buf bytes.Buffer
	_ = xml.EscapeText(&buf, []byte(s))
	b.Write(buf.Bytes())
}
