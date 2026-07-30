package kv

import (
	"bytes"
	"encoding/hex"
	"math"
	"reflect"
	"testing"
)

// Golden encodings, derived by hand from kv.proto field numbers and the proto3
// wire format. These pin the bytes the C++ side reads, so a refactor cannot
// silently change the on-disk format.
func TestMarshalGolden(t *testing.T) {
	cases := []struct {
		name string
		in   Value
		want string
	}{
		// field 1, wire 2 -> tag 0x0A; len 5; "hello"
		{"string", String("hello"), "0a0568656c6c6f"},
		// oneof presence is explicit: an empty string still emits the field.
		{"empty string", String(""), "0a00"},
		// field 2, wire 0 -> tag 0x10; varint 300 = 0xAC 0x02
		{"int 300", Int(300), "10ac02"},
		// zero is still written because the field sits in a oneof.
		{"int 0", Int(0), "1000"},
		// negative int32 sign-extends to 64 bits: 10-byte varint.
		{"int -1", Int(-1), "10ffffffffffffffffff01"},
		// field 3, wire 1 -> tag 0x19; float64 1.5 little-endian
		{"double 1.5", Double(1.5), "19000000000000f83f"},
		// field 6, wire 0 -> tag 0x30
		{"bool true", Bool(true), "3001"},
		{"bool false", Bool(false), "3000"},
		// field 4 (array) wraps ArrayType{ repeated values = 1 }:
		//   Int(7)  -> 10 07                 (2 bytes)
		//   values  -> 0a 02 <2 bytes>       (4 bytes)
		//   outer   -> 22 04 <4 bytes>
		{"array of one int", Array(Int(7)), "22040a021007"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			want, err := hex.DecodeString(stripSpaces(c.want))
			if err != nil {
				t.Fatalf("bad golden hex: %v", err)
			}
			got := c.in.Marshal()
			if !bytes.Equal(got, want) {
				t.Errorf("got %x want %x", got, want)
			}
		})
	}
}

func stripSpaces(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] != ' ' {
			out = append(out, s[i])
		}
	}
	return string(out)
}

func TestMapGolden(t *testing.T) {
	// MapType{ items = [KeyValuePair{key:"a", value:int 1}] }
	// pair:  0a 01 61   12 02 10 01      -> 7 bytes
	// items: 0a 07 <pair>                -> 9 bytes
	// outer: 2a 09 <items>
	got := Map(map[string]Value{"a": Int(1)}).Marshal()
	want, _ := hex.DecodeString("2a090a070a016112021001")
	if !bytes.Equal(got, want) {
		t.Errorf("got %x want %x", got, want)
	}
}

func TestRoundTrip(t *testing.T) {
	cases := []Value{
		String("weapon"),
		String(""),
		Int(0),
		Int(-42),
		Int(math.MaxInt32),
		Int(math.MinInt32),
		Double(0),
		Double(-1.25),
		Bool(true),
		Bool(false),
		Array(),
		Array(Int(1), String("two"), Double(3.5), Bool(true)),
		Map(map[string]Value{}),
		Map(map[string]Value{
			"experience": Int(1200),
			"mastered":   Bool(true),
			"perks": Array(Map(map[string]Value{
				"index":        Int(3),
				"type":         Int(7),
				"value":        Double(12.5),
				"bestiaryName": String("dragon"),
			})),
		}),
	}

	for i, in := range cases {
		out, err := Unmarshal(in.Marshal(), 0)
		if err != nil {
			t.Fatalf("case %d: unmarshal: %v", i, err)
		}
		if !valuesEqual(in, out) {
			t.Errorf("case %d: round trip mismatch\n in: %#v\nout: %#v", i, in, out)
		}
	}
}

// valuesEqual compares ignoring Timestamp, which the SQL column supplies.
func valuesEqual(a, b Value) bool {
	if a.Kind != b.Kind {
		return false
	}
	switch a.Kind {
	case KindString:
		return a.Str == b.Str
	case KindInt:
		return a.Int == b.Int
	case KindDouble:
		return a.Double == b.Double
	case KindBool:
		return a.Bool == b.Bool
	case KindArray:
		if len(a.Array) != len(b.Array) {
			return false
		}
		for i := range a.Array {
			if !valuesEqual(a.Array[i], b.Array[i]) {
				return false
			}
		}
		return true
	case KindMap:
		if len(a.Map) != len(b.Map) {
			return false
		}
		for k, av := range a.Map {
			bv, ok := b.Map[k]
			if !ok || !valuesEqual(av, bv) {
				return false
			}
		}
		return true
	}
	return true
}

// An unset oneof must serialize to an empty message and decode back to a
// kind-less value, which is what C++ writes for a default ValueWrapper.
func TestEmptyValue(t *testing.T) {
	got := Value{}.Marshal()
	if len(got) != 0 {
		t.Fatalf("expected empty encoding, got %x", got)
	}
	back, err := Unmarshal(nil, 99)
	if err != nil {
		t.Fatalf("unmarshal empty: %v", err)
	}
	if back.Kind != 0 || back.Timestamp != 99 {
		t.Errorf("unexpected %#v", back)
	}
}

// Unknown fields must be skipped rather than aborting the parse, so a payload
// written by a newer C++ build still loads.
func TestUnknownFieldSkipped(t *testing.T) {
	// field 9 varint (tag 0x48) then a real int field.
	data, _ := hex.DecodeString("48071005")
	got, err := Unmarshal(data, 0)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Kind != KindInt || got.Int != 5 {
		t.Errorf("got %#v", got)
	}
}

func TestMapDeterministicOrder(t *testing.T) {
	v := Map(map[string]Value{"z": Int(1), "a": Int(2), "m": Int(3)})
	first := v.Marshal()
	for i := 0; i < 20; i++ {
		if !reflect.DeepEqual(v.Marshal(), first) {
			t.Fatal("map encoding is not deterministic across calls")
		}
	}
}

func TestBuildKeyMatchesCppScoping(t *testing.T) {
	// Player::kv() is scoped("player")->scoped(guid); weapon proficiency then
	// adds scoped("weapon-proficiency"), and the key is the weapon id.
	got := buildKey(buildKey(buildKey("player", "1234"), "weapon-proficiency"), "3264")
	want := "player.1234.weapon-proficiency.3264"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}
