// Package kv implements the key/value store the C++ server keeps in the
// `kv_store` table, including the protobuf encoding of its values.
//
// The wire format is src/protobuf/kv.proto:
//
//	message ValueWrapper {
//	  oneof value {
//	    string    str_value    = 1;
//	    int32     int_value    = 2;
//	    double    double_value = 3;
//	    ArrayType array_value  = 4;
//	    MapType   map_value    = 5;
//	    bool      bool_value   = 6;
//	  }
//	}
//	message ArrayType    { repeated ValueWrapper values = 1; }
//	message KeyValuePair { string key = 1; ValueWrapper value = 2; }
//	message MapType      { repeated KeyValuePair items = 1; }
//
// Encoding is hand-rolled rather than generated so the package carries no
// protobuf runtime dependency, matching how internal/appproto handles
// appearances.proto.
package kv

import (
	"encoding/binary"
	"fmt"
	"math"
	"sort"
)

// Kind tags which arm of the ValueWrapper oneof a Value carries.
type Kind uint8

const (
	KindString Kind = iota + 1
	KindInt
	KindDouble
	KindArray
	KindMap
	KindBool
)

// Field numbers from kv.proto.
const (
	fieldStr    = 1
	fieldInt    = 2
	fieldDouble = 3
	fieldArray  = 4
	fieldMap    = 5
	fieldBool   = 6

	fieldArrayValues = 1
	fieldPairKey     = 1
	fieldPairValue   = 2
	fieldMapItems    = 1
)

// Wire types.
const (
	wireVarint = 0
	wireFixed64 = 1
	wireBytes   = 2
)

// Value mirrors C++ ValueWrapper: exactly one arm is meaningful, selected by Kind.
//
// Int is int32 on purpose — C++ aliases IntType to `int` and the proto field is
// int32, so a wider Go type would silently accept values the C++ side truncates.
type Value struct {
	Kind   Kind
	Str    string
	Int    int32
	Double float64
	Bool   bool
	Array  []Value
	Map    map[string]Value

	// Deleted marks a tombstone. C++ does not serialize this flag; a deleted value
	// makes the SQL layer DELETE the row instead of upserting it.
	Deleted bool

	// Timestamp comes from the kv_store column, not from the proto payload.
	Timestamp uint64
}

// Constructors.
func String(s string) Value        { return Value{Kind: KindString, Str: s} }
func Int(i int32) Value            { return Value{Kind: KindInt, Int: i} }
func Double(f float64) Value       { return Value{Kind: KindDouble, Double: f} }
func Bool(b bool) Value            { return Value{Kind: KindBool, Bool: b} }
func Array(vs ...Value) Value      { return Value{Kind: KindArray, Array: vs} }
func Map(m map[string]Value) Value { return Value{Kind: KindMap, Map: m} }

// Deleted returns the tombstone value, mirroring ValueWrapper::deleted().
func DeletedValue() Value { return Value{Deleted: true} }

// GetInt returns the int arm, or 0. Mirrors get<IntType>() on a missing key.
func (v Value) GetInt() int32 {
	if v.Kind == KindInt {
		return v.Int
	}
	return 0
}

// GetDouble returns the double arm, or 0.
func (v Value) GetDouble() float64 {
	if v.Kind == KindDouble {
		return v.Double
	}
	return 0
}

// GetString returns the string arm, or "".
func (v Value) GetString() string {
	if v.Kind == KindString {
		return v.Str
	}
	return ""
}

// GetBool returns the bool arm, or false.
func (v Value) GetBool() bool {
	if v.Kind == KindBool {
		return v.Bool
	}
	return false
}

// MapValue looks up key in the map arm.
func (v Value) MapValue(key string) (Value, bool) {
	if v.Kind != KindMap || v.Map == nil {
		return Value{}, false
	}
	got, ok := v.Map[key]
	return got, ok
}

// ---- encoding ----

func appendTag(b []byte, field int, wire byte) []byte {
	return binary.AppendUvarint(b, uint64(field)<<3|uint64(wire))
}

// appendInt32 encodes a proto3 int32. Negative values are sign-extended to 64
// bits, producing a 10-byte varint — the same as the reference implementation.
func appendInt32(b []byte, v int32) []byte {
	return binary.AppendUvarint(b, uint64(int64(v)))
}

func appendBytesField(b []byte, field int, payload []byte) []byte {
	b = appendTag(b, field, wireBytes)
	b = binary.AppendUvarint(b, uint64(len(payload)))
	return append(b, payload...)
}

// Marshal encodes v as a ValueWrapper message.
//
// Note that proto3 still emits a field whose value is the zero value when that
// field is inside a oneof, because oneof presence is explicit. So Int(0) encodes
// as `10 00`, not as an empty message — matching set_int_value(0) in C++.
func (v Value) Marshal() []byte {
	return v.appendTo(nil)
}

func (v Value) appendTo(b []byte) []byte {
	switch v.Kind {
	case KindString:
		b = appendBytesField(b, fieldStr, []byte(v.Str))
	case KindInt:
		b = appendTag(b, fieldInt, wireVarint)
		b = appendInt32(b, v.Int)
	case KindDouble:
		b = appendTag(b, fieldDouble, wireFixed64)
		b = binary.LittleEndian.AppendUint64(b, math.Float64bits(v.Double))
	case KindBool:
		b = appendTag(b, fieldBool, wireVarint)
		if v.Bool {
			b = append(b, 1)
		} else {
			b = append(b, 0)
		}
	case KindArray:
		var inner []byte
		for _, elem := range v.Array {
			inner = appendBytesField(inner, fieldArrayValues, elem.Marshal())
		}
		b = appendBytesField(b, fieldArray, inner)
	case KindMap:
		var inner []byte
		// C++ iterates a flat_hash_map, so its key order is unspecified. Sorting
		// keys here makes the output deterministic; both sides decode either order.
		keys := make([]string, 0, len(v.Map))
		for k := range v.Map {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			var pair []byte
			pair = appendBytesField(pair, fieldPairKey, []byte(k))
			pair = appendBytesField(pair, fieldPairValue, v.Map[k].Marshal())
			inner = appendBytesField(inner, fieldMapItems, pair)
		}
		b = appendBytesField(b, fieldMap, inner)
	default:
		// An unset oneof serializes to an empty message, which is what C++ writes
		// for a default-constructed ValueWrapper.
	}
	return b
}

// ---- decoding ----

type reader struct {
	buf []byte
	pos int
}

func (r *reader) done() bool { return r.pos >= len(r.buf) }

func (r *reader) uvarint() (uint64, error) {
	v, n := binary.Uvarint(r.buf[r.pos:])
	if n <= 0 {
		return 0, fmt.Errorf("kv: malformed varint at %d", r.pos)
	}
	r.pos += n
	return v, nil
}

func (r *reader) bytes() ([]byte, error) {
	n, err := r.uvarint()
	if err != nil {
		return nil, err
	}
	if uint64(r.pos)+n > uint64(len(r.buf)) {
		return nil, fmt.Errorf("kv: length %d overruns buffer at %d", n, r.pos)
	}
	out := r.buf[r.pos : r.pos+int(n)]
	r.pos += int(n)
	return out, nil
}

func (r *reader) fixed64() (uint64, error) {
	if r.pos+8 > len(r.buf) {
		return 0, fmt.Errorf("kv: truncated fixed64 at %d", r.pos)
	}
	v := binary.LittleEndian.Uint64(r.buf[r.pos:])
	r.pos += 8
	return v, nil
}

// skip advances past a field of the given wire type, so an unknown field does not
// abort the parse.
func (r *reader) skip(wire byte) error {
	switch wire {
	case wireVarint:
		_, err := r.uvarint()
		return err
	case wireFixed64:
		_, err := r.fixed64()
		return err
	case wireBytes:
		_, err := r.bytes()
		return err
	case 5: // fixed32
		if r.pos+4 > len(r.buf) {
			return fmt.Errorf("kv: truncated fixed32 at %d", r.pos)
		}
		r.pos += 4
		return nil
	default:
		return fmt.Errorf("kv: unsupported wire type %d", wire)
	}
}

// Unmarshal decodes a ValueWrapper message. timestamp comes from the kv_store
// column and is propagated into nested values, as fromProto does.
func Unmarshal(data []byte, timestamp uint64) (Value, error) {
	v, err := unmarshalValue(data)
	if err != nil {
		return Value{}, err
	}
	v.Timestamp = timestamp
	stampNested(&v, timestamp)
	return v, nil
}

func stampNested(v *Value, ts uint64) {
	v.Timestamp = ts
	for i := range v.Array {
		stampNested(&v.Array[i], ts)
	}
	for k, elem := range v.Map {
		stampNested(&elem, ts)
		v.Map[k] = elem
	}
}

func unmarshalValue(data []byte) (Value, error) {
	var out Value
	r := &reader{buf: data}
	for !r.done() {
		key, err := r.uvarint()
		if err != nil {
			return out, err
		}
		field := int(key >> 3)
		wire := byte(key & 0x7)

		switch field {
		case fieldStr:
			raw, err := r.bytes()
			if err != nil {
				return out, err
			}
			out = String(string(raw))
		case fieldInt:
			raw, err := r.uvarint()
			if err != nil {
				return out, err
			}
			out = Int(int32(int64(raw)))
		case fieldDouble:
			raw, err := r.fixed64()
			if err != nil {
				return out, err
			}
			out = Double(math.Float64frombits(raw))
		case fieldBool:
			raw, err := r.uvarint()
			if err != nil {
				return out, err
			}
			out = Bool(raw != 0)
		case fieldArray:
			raw, err := r.bytes()
			if err != nil {
				return out, err
			}
			arr, err := unmarshalArray(raw)
			if err != nil {
				return out, err
			}
			out = Value{Kind: KindArray, Array: arr}
		case fieldMap:
			raw, err := r.bytes()
			if err != nil {
				return out, err
			}
			m, err := unmarshalMap(raw)
			if err != nil {
				return out, err
			}
			out = Value{Kind: KindMap, Map: m}
		default:
			if err := r.skip(wire); err != nil {
				return out, err
			}
		}
	}
	return out, nil
}

func unmarshalArray(data []byte) ([]Value, error) {
	var out []Value
	r := &reader{buf: data}
	for !r.done() {
		key, err := r.uvarint()
		if err != nil {
			return nil, err
		}
		if int(key>>3) != fieldArrayValues {
			if err := r.skip(byte(key & 0x7)); err != nil {
				return nil, err
			}
			continue
		}
		raw, err := r.bytes()
		if err != nil {
			return nil, err
		}
		elem, err := unmarshalValue(raw)
		if err != nil {
			return nil, err
		}
		out = append(out, elem)
	}
	return out, nil
}

func unmarshalMap(data []byte) (map[string]Value, error) {
	out := make(map[string]Value)
	r := &reader{buf: data}
	for !r.done() {
		key, err := r.uvarint()
		if err != nil {
			return nil, err
		}
		if int(key>>3) != fieldMapItems {
			if err := r.skip(byte(key & 0x7)); err != nil {
				return nil, err
			}
			continue
		}
		raw, err := r.bytes()
		if err != nil {
			return nil, err
		}
		pairKey, pairVal, err := unmarshalPair(raw)
		if err != nil {
			return nil, err
		}
		out[pairKey] = pairVal
	}
	return out, nil
}

func unmarshalPair(data []byte) (string, Value, error) {
	var (
		key string
		val Value
	)
	r := &reader{buf: data}
	for !r.done() {
		tag, err := r.uvarint()
		if err != nil {
			return "", val, err
		}
		switch int(tag >> 3) {
		case fieldPairKey:
			raw, err := r.bytes()
			if err != nil {
				return "", val, err
			}
			key = string(raw)
		case fieldPairValue:
			raw, err := r.bytes()
			if err != nil {
				return "", val, err
			}
			val, err = unmarshalValue(raw)
			if err != nil {
				return "", val, err
			}
		default:
			if err := r.skip(byte(tag & 0x7)); err != nil {
				return "", val, err
			}
		}
	}
	return key, val, nil
}
