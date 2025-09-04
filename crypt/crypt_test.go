package crypt

import (
	"bytes"
	"testing"
)

func TestRandomKey(t *testing.T) {
	key1 := RandomKey()
	key2 := RandomKey()

	if len(key1) != 8 {
		t.Errorf("Expected key length 8, got %d", len(key1))
	}

	if bytes.Equal(key1, key2) {
		t.Error("Random keys should be different")
	}
}

func TestBase64(t *testing.T) {
	data := []byte("hello world")
	encoded := Base64Encode(data)
	decoded, err := Base64Decode(encoded)

	if err != nil {
		t.Errorf("Base64Decode failed: %v", err)
	}

	if !bytes.Equal(data, decoded) {
		t.Errorf("Expected %v, got %v", data, decoded)
	}
}

func TestDHExchange(t *testing.T) {
	clientKey := RandomKey()
	serverKey := RandomKey()

	clientPublic := DHExchange(clientKey)
	serverPublic := DHExchange(serverKey)

	if len(clientPublic) != 8 {
		t.Errorf("Expected public key length 8, got %d", len(clientPublic))
	}

	if len(serverPublic) != 8 {
		t.Errorf("Expected public key length 8, got %d", len(serverPublic))
	}

	// Test that same key produces same result
	clientPublic2 := DHExchange(clientKey)
	if !bytes.Equal(clientPublic, clientPublic2) {
		t.Error("Same private key should produce same public key")
	}
}

func TestDHSecret(t *testing.T) {
	clientPrivate := RandomKey()
	serverPrivate := RandomKey()

	clientPublic := DHExchange(clientPrivate)
	serverPublic := DHExchange(serverPrivate)

	// Calculate shared secrets
	secretFromClient := DHSecret(clientPrivate, serverPublic)
	secretFromServer := DHSecret(serverPrivate, clientPublic)

	if len(secretFromClient) != 8 {
		t.Errorf("Expected secret length 8, got %d", len(secretFromClient))
	}

	if !bytes.Equal(secretFromClient, secretFromServer) {
		t.Errorf("Secrets should be equal: client=%x, server=%x",
			secretFromClient, secretFromServer)
	}
}

func TestHMAC64(t *testing.T) {
	challenge := []byte("challenge")
	secret := []byte("secret12") // 8 bytes

	hmac := HMAC64(challenge, secret)

	if len(hmac) != 8 {
		t.Errorf("Expected HMAC length 8, got %d", len(hmac))
	}

	// Test consistency
	hmac2 := HMAC64(challenge, secret)
	if !bytes.Equal(hmac, hmac2) {
		t.Error("HMAC should be consistent")
	}
}

func TestDESEncryption(t *testing.T) {
	key := []byte("12345678") // 8 bytes
	data := []byte("hello world test data")

	encrypted := DESEncode(key, data)
	decrypted := DESDecode(key, encrypted)

	if !bytes.Equal(data, decrypted) {
		t.Errorf("Expected %s, got %s", string(data), string(decrypted))
	}
}

func TestDESWithEmptyData(t *testing.T) {
	key := []byte("12345678")
	data := []byte("")

	encrypted := DESEncode(key, data)
	decrypted := DesDecode(key, encrypted)

	if !bytes.Equal(data, decrypted) {
		t.Errorf("Expected empty data, got %v", decrypted)
	}
}

func TestHashKey(t *testing.T) {
	text := "test string"
	hash := HashKey(text)

	if len(hash) != 8 {
		t.Errorf("Expected hash length 8, got %d", len(hash))
	}

	// Test consistency
	hash2 := HashKey(text)
	if !bytes.Equal(hash, hash2) {
		t.Error("Hash should be consistent")
	}
}
