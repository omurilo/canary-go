// Package tibcrypto implements the exact cryptographic primitives used by the
// Tibia/Canary network protocol: raw RSA-1024, XTEA (32 rounds), and the
// Adler-32 packet checksum. Byte-for-byte compatible with the C++ server.
package tibcrypto

import "encoding/binary"

// XTEAKey is a 128-bit key expressed as four little-endian uint32 words, exactly
// as the client transmits it inside the RSA block.
type XTEAKey [4]uint32

// xteaDelta is the canonical TFS/OTServ delta constant. The C++ server stores it
// negated (0x61C88647) and flips the add/sub direction; the two are equivalent.
const xteaDelta uint32 = 0x9E3779B9

// KeyFromBytes reads four consecutive little-endian uint32 words.
func KeyFromBytes(b []byte) XTEAKey {
	var k XTEAKey
	for i := 0; i < 4; i++ {
		k[i] = binary.LittleEndian.Uint32(b[i*4:])
	}
	return k
}

// Encrypt encrypts data in place. len(data) must be a multiple of 8.
func (k XTEAKey) Encrypt(data []byte) {
	for i := 0; i+8 <= len(data); i += 8 {
		v0 := binary.LittleEndian.Uint32(data[i:])
		v1 := binary.LittleEndian.Uint32(data[i+4:])
		var sum uint32
		for r := 0; r < 32; r++ {
			v0 += (((v1 << 4) ^ (v1 >> 5)) + v1) ^ (sum + k[sum&3])
			sum += xteaDelta
			v1 += (((v0 << 4) ^ (v0 >> 5)) + v0) ^ (sum + k[(sum>>11)&3])
		}
		binary.LittleEndian.PutUint32(data[i:], v0)
		binary.LittleEndian.PutUint32(data[i+4:], v1)
	}
}

// Decrypt decrypts data in place. len(data) must be a multiple of 8.
func (k XTEAKey) Decrypt(data []byte) {
	for i := 0; i+8 <= len(data); i += 8 {
		v0 := binary.LittleEndian.Uint32(data[i:])
		v1 := binary.LittleEndian.Uint32(data[i+4:])
		sum := uint32(0xC6EF3720)
		for r := 0; r < 32; r++ {
			v1 -= (((v0 << 4) ^ (v0 >> 5)) + v0) ^ (sum + k[(sum>>11)&3])
			sum -= xteaDelta
			v0 -= (((v1 << 4) ^ (v1 >> 5)) + v1) ^ (sum + k[sum&3])
		}
		binary.LittleEndian.PutUint32(data[i:], v0)
		binary.LittleEndian.PutUint32(data[i+4:], v1)
	}
}
