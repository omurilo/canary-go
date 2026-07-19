package propstream

import (
	"bytes"
	"testing"
)

func TestPropStreamRoundTrip(t *testing.T) {
	writer := NewPropWriteStream()

	writer.WriteUint8(42)
	writer.WriteUint16(1337)
	writer.WriteUint32(0xDEADBEEF)
	writer.WriteUint64(0xCAFEBABE12345678)
	writer.WriteString("hello canary")

	stream := writer.GetStream()

	reader := NewPropStream(stream)

	u8, err := reader.ReadUint8()
	if err != nil || u8 != 42 {
		t.Errorf("expected 42, got %v, err: %v", u8, err)
	}

	u16, err := reader.ReadUint16()
	if err != nil || u16 != 1337 {
		t.Errorf("expected 1337, got %v, err: %v", u16, err)
	}

	u32, err := reader.ReadUint32()
	if err != nil || u32 != 0xDEADBEEF {
		t.Errorf("expected 0xDEADBEEF, got %v, err: %v", u32, err)
	}

	u64, err := reader.ReadUint64()
	if err != nil || u64 != 0xCAFEBABE12345678 {
		t.Errorf("expected 0xCAFEBABE12345678, got %v, err: %v", u64, err)
	}

	str, err := reader.ReadString()
	if err != nil || str != "hello canary" {
		t.Errorf("expected 'hello canary', got %v, err: %v", str, err)
	}

	if reader.Size() != 0 {
		t.Errorf("expected empty reader, got size %d", reader.Size())
	}
}

func TestPropStreamOverflow(t *testing.T) {
	reader := NewPropStream([]byte{1})
	_, err := reader.ReadUint16()
	if err != ErrReadOverflow {
		t.Errorf("expected ErrReadOverflow, got %v", err)
	}
}

func TestPropStreamClear(t *testing.T) {
	writer := NewPropWriteStream()
	writer.WriteUint8(1)
	writer.Clear()
	if len(writer.GetStream()) != 0 {
		t.Errorf("expected empty stream after Clear, got %d", len(writer.GetStream()))
	}
}

func TestPropStreamSkip(t *testing.T) {
	writer := NewPropWriteStream()
	writer.WriteUint32(100)
	writer.WriteUint32(200)

	reader := NewPropStream(writer.GetStream())
	if err := reader.Skip(4); err != nil {
		t.Errorf("expected nil error on skip, got %v", err)
	}

	u32, err := reader.ReadUint32()
	if err != nil || u32 != 200 {
		t.Errorf("expected 200, got %v, err: %v", u32, err)
	}
}

func TestPropStreamLargeString(t *testing.T) {
	writer := NewPropWriteStream()
	largeStr := string(bytes.Repeat([]byte("A"), 0x10000))
	writer.WriteString(largeStr)

	reader := NewPropStream(writer.GetStream())
	strLen, _ := reader.ReadUint16()
	if strLen != 0 {
		t.Errorf("expected 0 length for string exceeding 0xFFFF, got %d", strLen)
	}
}
