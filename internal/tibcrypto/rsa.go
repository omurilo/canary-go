package tibcrypto

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"os"
)

// BlockSize is the fixed RSA block size (1024-bit key => 128 bytes).
const BlockSize = 128

// RSA wraps a 1024-bit private key and performs the raw (unpadded) modular
// exponentiation the Tibia protocol relies on: m = c^d mod n.
type RSA struct {
	key *rsa.PrivateKey
}

// LoadRSAFromPEM parses a PKCS#1 (or PKCS#8) PEM private key from disk.
func LoadRSAFromPEM(path string) (*RSA, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseRSAFromPEM(data)
}

// ParseRSAFromPEM parses a PEM-encoded RSA private key from a byte slice.
func ParseRSAFromPEM(data []byte) (*RSA, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("tibcrypto: no PEM block found")
	}
	var key *rsa.PrivateKey
	switch block.Type {
	case "RSA PRIVATE KEY":
		k, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, err
		}
		key = k
	case "PRIVATE KEY":
		k, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, err
		}
		rk, ok := k.(*rsa.PrivateKey)
		if !ok {
			return nil, errors.New("tibcrypto: PKCS8 key is not RSA")
		}
		key = rk
	default:
		return nil, fmt.Errorf("tibcrypto: unsupported PEM type %q", block.Type)
	}
	if key.N.BitLen() > BlockSize*8 {
		return nil, fmt.Errorf("tibcrypto: RSA key too large (%d bits)", key.N.BitLen())
	}
	if err := key.Validate(); err != nil {
		return nil, err
	}
	return &RSA{key: key}, nil
}

// Decrypt performs the raw private-key operation on a single 128-byte block,
// in place. The client encrypts with the matching public key (no padding), so
// the caller must verify the leading plaintext byte is 0x00 afterwards.
func (r *RSA) Decrypt(block []byte) error {
	if len(block) != BlockSize {
		return fmt.Errorf("tibcrypto: RSA block must be %d bytes, got %d", BlockSize, len(block))
	}
	c := new(big.Int).SetBytes(block)
	m := new(big.Int).Exp(c, r.key.D, r.key.N)
	m.FillBytes(block) // big-endian, left zero-padded to BlockSize
	return nil
}

// Encrypt performs the raw public-key operation (m^e mod n) on a 128-byte block
// in place. Used by the test client to mimic the real client.
func (r *RSA) Encrypt(block []byte) error {
	if len(block) != BlockSize {
		return fmt.Errorf("tibcrypto: RSA block must be %d bytes, got %d", BlockSize, len(block))
	}
	m := new(big.Int).SetBytes(block)
	c := new(big.Int).Exp(m, big.NewInt(int64(r.key.E)), r.key.N)
	c.FillBytes(block)
	return nil
}

// Public returns the underlying public key (modulus/exponent) for clients.
func (r *RSA) Public() *rsa.PublicKey { return &r.key.PublicKey }

// GenerateRSA creates a fresh 1024-bit key (used only for tests/tooling).
func GenerateRSA() (*RSA, error) {
	k, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		return nil, err
	}
	return &RSA{key: k}, nil
}
