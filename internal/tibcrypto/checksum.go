package tibcrypto

import "hash/adler32"

// Adler32 returns the Adler-32 checksum in the exact (b<<16)|a form the client
// expects. Go's stdlib produces this layout directly.
func Adler32(data []byte) uint32 {
	return adler32.Checksum(data)
}
