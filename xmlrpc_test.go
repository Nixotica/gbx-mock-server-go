package gbxmockserver

import (
	"reflect"
	"testing"
	"time"
)

// TestValueRoundTrip encodes each Value kind, parses the resulting XML back
// through the methodCall path, and asserts the parsed Value equals the
// original. Catches encoder/decoder asymmetry and the historical
// zero-int-drop bug.
func TestValueRoundTrip(t *testing.T) {
	cases := []struct {
		name string
		v    Value
		eq   func(t *testing.T, got Value)
	}{
		{
			name: "string",
			v:    StringValue("hello & <world>"),
			eq: func(t *testing.T, got Value) {
				if s, _ := got.AsString(); s != "hello & <world>" {
					t.Fatalf("string round-trip got %q", s)
				}
			},
		},
		{
			name: "int-positive",
			v:    IntValue(42),
			eq:   intEq(t, 42),
		},
		{
			name: "int-zero",
			v:    IntValue(0),
			eq:   intEq(t, 0),
		},
		{
			name: "int-negative",
			v:    IntValue(-7),
			eq:   intEq(t, -7),
		},
		{
			name: "bool-true",
			v:    BoolValue(true),
			eq: func(t *testing.T, got Value) {
				if b, _ := got.AsBool(); !b {
					t.Fatal("bool round-trip lost true")
				}
			},
		},
		{
			name: "double",
			v:    DoubleValue(3.14159),
			eq: func(t *testing.T, got Value) {
				if f, _ := got.AsDouble(); f != 3.14159 {
					t.Fatalf("double round-trip got %v", f)
				}
			},
		},
		{
			name: "base64",
			v:    Base64Value([]byte{0x00, 0x01, 0xff, 0x7f}),
			eq: func(t *testing.T, got Value) {
				b, ok := got.AsBase64()
				if !ok || !reflect.DeepEqual(b, []byte{0x00, 0x01, 0xff, 0x7f}) {
					t.Fatalf("base64 round-trip got %#v", b)
				}
			},
		},
		{
			name: "datetime",
			v:    DateTimeValue(time.Date(2026, 5, 16, 12, 30, 45, 0, time.UTC)),
			eq: func(t *testing.T, got Value) {
				ts, _ := got.AsDateTime()
				want := time.Date(2026, 5, 16, 12, 30, 45, 0, time.UTC)
				if !ts.Equal(want) {
					t.Fatalf("datetime round-trip got %v want %v", ts, want)
				}
			},
		},
		{
			name: "nil",
			v:    NilValue(),
			eq: func(t *testing.T, got Value) {
				if !got.IsNil() {
					t.Fatalf("nil round-trip got kind=%v", got.Kind)
				}
			},
		},
		{
			name: "array-empty",
			v:    ArrayValue(),
			eq: func(t *testing.T, got Value) {
				a, ok := got.AsArray()
				if !ok || len(a) != 0 {
					t.Fatalf("empty array got %#v ok=%v", a, ok)
				}
			},
		},
		{
			name: "array-mixed",
			v:    ArrayValue(StringValue("a"), IntValue(2), BoolValue(false)),
			eq: func(t *testing.T, got Value) {
				a, _ := got.AsArray()
				if len(a) != 3 {
					t.Fatalf("array len got %d", len(a))
				}
				if s, _ := a[0].AsString(); s != "a" {
					t.Fatalf("array[0] %v", a[0])
				}
				if i, _ := a[1].AsInt(); i != 2 {
					t.Fatalf("array[1] %v", a[1])
				}
				if b, _ := a[2].AsBool(); b {
					t.Fatalf("array[2] %v", a[2])
				}
			},
		},
		{
			name: "struct-nested",
			v: StructValue(map[string]Value{
				"player": StructValue(map[string]Value{
					"login": StringValue("nadeo"),
					"score": IntValue(1234),
				}),
				"checkpoints": ArrayValue(IntValue(100), IntValue(200), IntValue(0)),
			}),
			eq: func(t *testing.T, got Value) {
				s, _ := got.AsStruct()
				inner, _ := s["player"].AsStruct()
				if login, _ := inner["login"].AsString(); login != "nadeo" {
					t.Fatalf("player.login got %v", inner["login"])
				}
				if sc, _ := inner["score"].AsInt(); sc != 1234 {
					t.Fatalf("player.score got %v", inner["score"])
				}
				cps, _ := s["checkpoints"].AsArray()
				if len(cps) != 3 {
					t.Fatalf("checkpoints len got %d", len(cps))
				}
				if z, _ := cps[2].AsInt(); z != 0 {
					t.Fatalf("checkpoints[2] zero-int dropped: %v", cps[2])
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := encodeMethodCall("X", []Value{tc.v})
			method, params, err := parseMethodCall(body)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if method != "X" || len(params) != 1 {
				t.Fatalf("method=%q params=%d", method, len(params))
			}
			tc.eq(t, params[0])
		})
	}
}

func intEq(t *testing.T, want int64) func(*testing.T, Value) {
	t.Helper()
	return func(t *testing.T, got Value) {
		t.Helper()
		i, ok := got.AsInt()
		if !ok || i != want {
			t.Fatalf("int round-trip got %v ok=%v want %d", i, ok, want)
		}
	}
}

// TestValueOfStruct walks a struct via the `xmlrpc:` tag and checks that the
// resulting Value matches what GbxRemoteGo's event-args layouts use on the
// wire.
func TestValueOfStruct(t *testing.T) {
	type WayPoint struct {
		AccountId string `xmlrpc:"AccountId"`
		RaceTime  int    `xmlrpc:"RaceTime"`
		IsEndRace bool   `xmlrpc:"IsEndRace"`
		Skip      string `xmlrpc:"-"`
		Untagged  string
	}

	v := ValueOf(WayPoint{
		AccountId: "abc",
		RaceTime:  45000,
		IsEndRace: true,
		Skip:      "ignored",
		Untagged:  "kept",
	})

	s, ok := v.AsStruct()
	if !ok {
		t.Fatalf("expected struct kind, got %v", v.Kind)
	}
	if _, present := s["Skip"]; present {
		t.Errorf("xmlrpc:\"-\" field should be skipped")
	}
	if got, _ := s["AccountId"].AsString(); got != "abc" {
		t.Errorf("AccountId got %q", got)
	}
	if got, _ := s["RaceTime"].AsInt(); got != 45000 {
		t.Errorf("RaceTime got %d", got)
	}
	if got, _ := s["IsEndRace"].AsBool(); !got {
		t.Errorf("IsEndRace got false")
	}
	if got, _ := s["Untagged"].AsString(); got != "kept" {
		t.Errorf("untagged field got %q", got)
	}
}

// TestFaultEncoding verifies the fault frame matches the XML-RPC standard
// layout (faultCode int, faultString string).
func TestFaultEncoding(t *testing.T) {
	body := encodeFault(-503, "boom")
	if !contains(body, "<fault>") || !contains(body, "<name>faultCode</name>") ||
		!contains(body, "<name>faultString</name>") || !contains(body, "boom") ||
		!contains(body, "-503") {
		t.Fatalf("fault body missing parts: %s", body)
	}
}

func contains(b []byte, sub string) bool {
	return indexOf(b, sub) >= 0
}

func indexOf(b []byte, sub string) int {
	s := string(b)
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
