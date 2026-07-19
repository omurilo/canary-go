package propstream

import (
	"encoding/binary"
	"errors"
)

var ErrReadOverflow = errors.New("propstream: read overflow")

type PropStream struct {
	buf []byte
	pos int
}

func NewPropStream(buf []byte) *PropStream {
	return &PropStream{buf: buf}
}

func (p *PropStream) Size() int {
	return len(p.buf) - p.pos
}

func (p *PropStream) ReadUint8() (uint8, error) {
	if p.Size() < 1 {
		return 0, ErrReadOverflow
	}
	val := p.buf[p.pos]
	p.pos++
	return val, nil
}

func (p *PropStream) ReadUint16() (uint16, error) {
	if p.Size() < 2 {
		return 0, ErrReadOverflow
	}
	val := binary.LittleEndian.Uint16(p.buf[p.pos:])
	p.pos += 2
	return val, nil
}

func (p *PropStream) ReadUint32() (uint32, error) {
	if p.Size() < 4 {
		return 0, ErrReadOverflow
	}
	val := binary.LittleEndian.Uint32(p.buf[p.pos:])
	p.pos += 4
	return val, nil
}

func (p *PropStream) ReadUint64() (uint64, error) {
	if p.Size() < 8 {
		return 0, ErrReadOverflow
	}
	val := binary.LittleEndian.Uint64(p.buf[p.pos:])
	p.pos += 8
	return val, nil
}

func (p *PropStream) ReadString() (string, error) {
	strLen, err := p.ReadUint16()
	if err != nil {
		return "", err
	}
	if p.Size() < int(strLen) {
		return "", ErrReadOverflow
	}
	val := string(p.buf[p.pos : p.pos+int(strLen)])
	p.pos += int(strLen)
	return val, nil
}

func (p *PropStream) Skip(n int) error {
	if p.Size() < n {
		return ErrReadOverflow
	}
	p.pos += n
	return nil
}

type PropWriteStream struct {
	buf []byte
}

func NewPropWriteStream() *PropWriteStream {
	return &PropWriteStream{}
}

func (p *PropWriteStream) GetStream() []byte {
	return p.buf
}

func (p *PropWriteStream) Clear() {
	p.buf = p.buf[:0]
}

func (p *PropWriteStream) WriteUint8(val uint8) {
	p.buf = append(p.buf, val)
}

func (p *PropWriteStream) WriteUint16(val uint16) {
	b := make([]byte, 2)
	binary.LittleEndian.PutUint16(b, val)
	p.buf = append(p.buf, b...)
}

func (p *PropWriteStream) WriteUint32(val uint32) {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, val)
	p.buf = append(p.buf, b...)
}

func (p *PropWriteStream) WriteUint64(val uint64) {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, val)
	p.buf = append(p.buf, b...)
}

func (p *PropWriteStream) WriteString(val string) {
	if len(val) > 0xFFFF {
		p.WriteUint16(0)
		return
	}
	p.WriteUint16(uint16(len(val)))
	p.buf = append(p.buf, val...)
}
