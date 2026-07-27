// Package netmsg implements the Tibia NetworkMessage buffer: little-endian
// primitives, u16-length-prefixed strings, and positions. Reader and Writer are
// split; the Writer reserves headroom so the transport layer can prepend the
// checksum and length headers without copying the body.
package netmsg

import (
	"encoding/binary"
	"math"
)

const (
	// Headroom reserved at the front of an outbound buffer for prepended
	// headers: 2 (outer length) + 4 (checksum) + 2 (legacy inner length) = 8.
	Headroom = 8
	// MaxSize matches NETWORKMESSAGE_MAXSIZE.
	MaxSize = 65500
)

// ---------------- Reader ----------------

// Reader consumes a received packet body.
type Reader struct {
	buf []byte
	pos int
}

// NewReader wraps a body slice (checksum/length already stripped).
func NewReader(buf []byte) *Reader { return &Reader{buf: buf} }

func (r *Reader) Remaining() int { return len(r.buf) - r.pos }
func (r *Reader) Pos() int       { return r.pos }
func (r *Reader) Buffer() []byte { return r.buf }

// Skip advances the read cursor by n bytes.
func (r *Reader) Skip(n int) { r.pos += n }

// GetByte returns the next byte (0 if exhausted).
func (r *Reader) GetByte() byte {
	if r.pos >= len(r.buf) {
		return 0
	}
	b := r.buf[r.pos]
	r.pos++
	return b
}

func (r *Reader) GetU16() uint16 {
	if r.pos+2 > len(r.buf) {
		r.pos = len(r.buf)
		return 0
	}
	v := binary.LittleEndian.Uint16(r.buf[r.pos:])
	r.pos += 2
	return v
}

func (r *Reader) GetU32() uint32 {
	if r.pos+4 > len(r.buf) {
		r.pos = len(r.buf)
		return 0
	}
	v := binary.LittleEndian.Uint32(r.buf[r.pos:])
	r.pos += 4
	return v
}

func (r *Reader) GetU64() uint64 {
	if r.pos+8 > len(r.buf) {
		r.pos = len(r.buf)
		return 0
	}
	v := binary.LittleEndian.Uint64(r.buf[r.pos:])
	r.pos += 8
	return v
}

// GetBytes returns a copy of the next n bytes.
func (r *Reader) GetBytes(n int) []byte {
	if n < 0 || r.pos+n > len(r.buf) {
		n = len(r.buf) - r.pos
		if n < 0 {
			n = 0
		}
	}
	out := make([]byte, n)
	copy(out, r.buf[r.pos:r.pos+n])
	r.pos += n
	return out
}

// GetString reads a u16-length-prefixed string.
func (r *Reader) GetString() string {
	n := int(r.GetU16())
	if n <= 0 || r.pos+n > len(r.buf) {
		return ""
	}
	s := string(r.buf[r.pos : r.pos+n])
	r.pos += n
	return s
}

// Position mirrors the client tile coordinate.
type Position struct {
	X uint16
	Y uint16
	Z uint8
}

func (r *Reader) GetPosition() Position {
	return Position{X: r.GetU16(), Y: r.GetU16(), Z: r.GetByte()}
}

// ---------------- Writer ----------------

// Writer builds an outbound packet body, growing forward from Headroom so the
// transport layer can prepend headers via Prepend*.
type Writer struct {
	buf   []byte
	start int // index of first body byte
	pos   int // write cursor (one past last body byte)
}

// NewWriter returns an empty outbound message.
func NewWriter() *Writer {
	return &Writer{buf: make([]byte, MaxSize), start: Headroom, pos: Headroom}
}

// Len is the current body length.
func (w *Writer) Len() int { return w.pos - w.start }

// Bytes returns the current wire bytes (headers + body).
func (w *Writer) Bytes() []byte { return w.buf[w.start:w.pos] }

func (w *Writer) AddByte(b byte) {
	w.buf[w.pos] = b
	w.pos++
}

func (w *Writer) AddU16(v uint16) {
	binary.LittleEndian.PutUint16(w.buf[w.pos:], v)
	w.pos += 2
}


// SetU16 overwrites a u16 at the given offset (for updating previously written counts).
func (w *Writer) SetU16(offset int, v uint16) {
	if offset+2 > len(w.buf) {
		return
	}
	binary.LittleEndian.PutUint16(w.buf[offset:], v)
}

// Pos returns the current absolute write position in the buffer.
func (w *Writer) Pos() int { return w.pos }
func (w *Writer) AddU32(v uint32) {
	binary.LittleEndian.PutUint32(w.buf[w.pos:], v)
	w.pos += 4
}

func (w *Writer) AddU64(v uint64) {
	binary.LittleEndian.PutUint64(w.buf[w.pos:], v)
	w.pos += 8
}

func (w *Writer) AddBytes(b []byte) {
	copy(w.buf[w.pos:], b)
	w.pos += len(b)
}

// AddString writes a u16-length-prefixed string.
func (w *Writer) AddString(s string) {
	w.AddU16(uint16(len(s)))
	w.AddBytes([]byte(s))
}

// AddPosition writes u16 x, u16 y, u8 z.
func (w *Writer) AddPosition(p Position) {
	w.AddU16(p.X)
	w.AddU16(p.Y)
	w.AddByte(p.Z)
}

// AddDouble mirrors NetworkMessage::addDouble (u8 precision + u32 scaled value).
func (w *Writer) AddDouble(value float64, precision uint8) {
	w.AddByte(precision)
	scale := 1.0
	for i := uint8(0); i < precision; i++ {
		scale *= 10
	}
	w.AddU32(uint32(value*scale) + 0x7FFFFFFF)
}

// AddRawDouble writes an IEEE 754 double-precision float as 8 raw bytes
// (little-endian), matching C++ msg.add<double>(value).
func (w *Writer) AddRawDouble(value float64) {
	bits := math.Float64bits(value)
	w.AddU64(bits)
}

// PrependByte writes a byte just before the current body start.
func (w *Writer) PrependByte(b byte) {
	w.start--
	w.buf[w.start] = b
}

// PrependU16 writes a little-endian u16 before the body.
func (w *Writer) PrependU16(v uint16) {
	w.start -= 2
	binary.LittleEndian.PutUint16(w.buf[w.start:], v)
}

// PrependU32 writes a little-endian u32 before the body.
func (w *Writer) PrependU32(v uint32) {
	w.start -= 4
	binary.LittleEndian.PutUint32(w.buf[w.start:], v)
}

// PadTo appends filler byte 0x33 until the body length is a multiple of mult.
func (w *Writer) PadTo(mult int) {
	for w.Len()%mult != 0 {
		w.AddByte(0x33)
	}
}
