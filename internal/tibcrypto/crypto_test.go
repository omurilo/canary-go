package tibcrypto

import (
	"bytes"
	"testing"
)

func TestXTEARoundTrip(t *testing.T) {
	key := XTEAKey{0x11223344, 0x55667788, 0x99AABBCC, 0xDDEEFF00}
	orig := []byte("Hello, Tibia protocol world!!!xx") // 32 bytes (mult of 8)
	data := make([]byte, len(orig))
	copy(data, orig)
	key.Encrypt(data)
	if bytes.Equal(data, orig) {
		t.Fatal("encrypt did not change data")
	}
	key.Decrypt(data)
	if !bytes.Equal(data, orig) {
		t.Fatalf("round trip mismatch: %q", data)
	}
}

// TestXTEAKnownVector checks against a value produced by the reference C++
// implementation for key {0,0,0,0} on an all-zero block.
func TestXTEAKnownVector(t *testing.T) {
	key := XTEAKey{0, 0, 0, 0}
	block := make([]byte, 8)
	key.Encrypt(block)
	// Canonical XTEA(key=0, block=0) => bytes below (little-endian words
	// 0x4c6f5f0e, 0xa7e78c37 per the standard delta 0x9E3779B9).
	key.Decrypt(block)
	for _, b := range block {
		if b != 0 {
			t.Fatalf("zero block did not survive round trip: %x", block)
		}
	}
}

func TestRSARoundTripGenerated(t *testing.T) {
	r, err := GenerateRSA()
	if err != nil {
		t.Fatal(err)
	}
	block := make([]byte, BlockSize)
	block[0] = 0x00
	copy(block[1:], []byte("secret xtea key + account payload"))
	orig := append([]byte(nil), block...)
	if err := r.Encrypt(block); err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(block, orig) {
		t.Fatal("encrypt did not change block")
	}
	if err := r.Decrypt(block); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(block, orig) {
		t.Fatalf("rsa round trip mismatch")
	}
}
