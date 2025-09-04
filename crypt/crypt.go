package crypt

import (
	"crypto/des"
	"crypto/hmac"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math/big"
)

// DH parameters - using a simpler approach for compatibility
var (
	// Use a smaller modulus for 8-byte key compatibility
	dhP = big.NewInt(0xFFFFFFFB) // Large prime that fits in 32 bits
	dhG = big.NewInt(2)
)

// RandomKey generates a random 8-byte key
func RandomKey() []byte {
	key := make([]byte, 8)
	rand.Read(key)
	return key
}

// DHExchange performs DH key exchange
func DHExchange(key []byte) []byte {
	if len(key) != 8 {
		panic("DH key must be 8 bytes")
	}

	// Convert 8-byte key to big int, but keep it reasonable size
	private := new(big.Int).SetBytes(key)

	// Keep private key smaller to avoid overflow
	private = private.And(private, big.NewInt(0x7FFFFFFF))
	if private.Cmp(big.NewInt(1)) <= 0 {
		private = big.NewInt(2) // Ensure private key > 1
	}

	// Calculate g^private mod p
	public := new(big.Int).Exp(dhG, private, dhP)

	// Convert back to 8 bytes (pad to 8 bytes)
	result := make([]byte, 8)
	publicBytes := public.Bytes()

	// Copy to the end of result array
	if len(publicBytes) <= 8 {
		copy(result[8-len(publicBytes):], publicBytes)
	} else {
		copy(result, publicBytes[len(publicBytes)-8:])
	}

	return result
}

// DHSecret calculates shared secret from private and public keys
func DHSecret(privateKey, publicKey []byte) []byte {
	if len(privateKey) != 8 || len(publicKey) != 8 {
		panic("DH keys must be 8 bytes")
	}

	// Convert keys to big ints
	private := new(big.Int).SetBytes(privateKey)
	public := new(big.Int).SetBytes(publicKey)

	// Keep private key smaller
	private = private.And(private, big.NewInt(0x7FFFFFFF))
	if private.Cmp(big.NewInt(1)) <= 0 {
		private = big.NewInt(2)
	}

	// Calculate shared secret: public^private mod p
	secret := new(big.Int).Exp(public, private, dhP)

	// Convert back to 8 bytes
	result := make([]byte, 8)
	secretBytes := secret.Bytes()

	if len(secretBytes) <= 8 {
		copy(result[8-len(secretBytes):], secretBytes)
	} else {
		copy(result, secretBytes[len(secretBytes)-8:])
	}

	return result
}

// Base64Encode encodes bytes to base64 string
func Base64Encode(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

// Base64Decode decodes base64 string to bytes
func Base64Decode(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}

// HexEncode encodes bytes to hex string
func HexEncode(data []byte) string {
	return hex.EncodeToString(data)
}

// HexDecode decodes hex string to bytes
func HexDecode(s string) ([]byte, error) {
	return hex.DecodeString(s)
}

// HMAC64 calculates HMAC-SHA1 and returns first 8 bytes
func HMAC64(challenge, secret []byte) []byte {
	h := hmac.New(sha1.New, secret)
	h.Write(challenge)
	sum := h.Sum(nil)
	return sum[:8] // Return first 8 bytes
}

// HMACHash calculates HMAC-SHA1 hash
func HMACHash(secret []byte, text string) []byte {
	return HMAC64([]byte(text), secret)
}

// HashKey creates a hash key from string (MD5)
func HashKey(text string) []byte {
	h := md5.New()
	h.Write([]byte(text))
	return h.Sum(nil)[:8] // Return first 8 bytes for compatibility
}

// DESEncode encrypts data using DES with given key
func DESEncode(key, data []byte) []byte {
	if len(key) != 8 {
		panic("DES key must be 8 bytes")
	}

	block, err := des.NewCipher(key)
	if err != nil {
		panic(fmt.Sprintf("Failed to create DES cipher: %v", err))
	}

	// Pad data to multiple of 8 bytes
	padLen := 8 - (len(data) % 8)
	if padLen == 8 {
		padLen = 0
	}

	padded := make([]byte, len(data)+padLen)
	copy(padded, data)
	// PKCS5 padding
	for i := len(data); i < len(padded); i++ {
		padded[i] = byte(padLen)
	}

	encrypted := make([]byte, len(padded))

	// Encrypt block by block
	for i := 0; i < len(padded); i += 8 {
		block.Encrypt(encrypted[i:i+8], padded[i:i+8])
	}

	return encrypted
}

// DESDecode decrypts DES encrypted data
func DESDecode(key, data []byte) []byte {
	if len(key) != 8 {
		panic("DES key must be 8 bytes")
	}

	if len(data)%8 != 0 {
		panic("DES data must be multiple of 8 bytes")
	}

	block, err := des.NewCipher(key)
	if err != nil {
		panic(fmt.Sprintf("Failed to create DES cipher: %v", err))
	}

	decrypted := make([]byte, len(data))

	// Decrypt block by block
	for i := 0; i < len(data); i += 8 {
		block.Decrypt(decrypted[i:i+8], data[i:i+8])
	}

	// Remove PKCS5 padding
	if len(decrypted) > 0 {
		padLen := int(decrypted[len(decrypted)-1])
		if padLen > 0 && padLen <= 8 && padLen <= len(decrypted) {
			decrypted = decrypted[:len(decrypted)-padLen]
		}
	}

	return decrypted
}

// Utility functions that match skynet's interface

// DesEncode is an alias for DESEncode to match skynet interface
func DesEncode(secret []byte, data []byte) []byte {
	return DESEncode(secret, data)
}

// DesDecode is an alias for DESDecode to match skynet interface
func DesDecode(secret []byte, data []byte) []byte {
	return DESDecode(secret, data)
}
