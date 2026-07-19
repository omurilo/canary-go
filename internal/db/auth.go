package db

import (
	"crypto/sha1"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// VerifyPassword checks a plaintext attempt against a stored hash, supporting
// the two formats the C++ server accepts: Argon2 PHC strings and legacy
// hex-encoded SHA-1.
func VerifyPassword(attempt, stored string) bool {
	if strings.HasPrefix(stored, "$argon2") {
		return verifyArgon2(attempt, stored)
	}
	// Legacy SHA-1 (hex, lowercase).
	sum := sha1.Sum([]byte(attempt))
	return subtle.ConstantTimeCompare([]byte(hex.EncodeToString(sum[:])), []byte(strings.ToLower(stored))) == 1
}

// SHA1Hex returns the lowercase hex SHA-1 of s (used to seed test accounts).
func SHA1Hex(s string) string {
	sum := sha1.Sum([]byte(s))
	return hex.EncodeToString(sum[:])
}

// verifyArgon2 parses an Argon2id/i PHC string and re-derives the hash.
func verifyArgon2(attempt, phc string) bool {
	// Format: $argon2id$v=19$m=65536,t=3,p=1$<b64 salt>$<b64 hash>
	parts := strings.Split(phc, "$")
	if len(parts) != 6 {
		return false
	}
	variant := parts[1]
	if variant != "argon2id" && variant != "argon2i" {
		return false
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return false
	}
	var mem uint32
	var iters, par uint32
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &mem, &iters, &par); err != nil {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false
	}
	var got []byte
	if variant == "argon2id" {
		got = argon2.IDKey([]byte(attempt), salt, iters, mem, uint8(par), uint32(len(want)))
	} else {
		got = argon2.Key([]byte(attempt), salt, iters, mem, uint8(par), uint32(len(want)))
	}
	return subtle.ConstantTimeCompare(got, want) == 1
}
