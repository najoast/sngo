package crypt

import (
	"fmt"
	"testing"
)

func TestDHDebug(t *testing.T) {
	// Use fixed keys for debugging
	clientPrivate := []byte{1, 2, 3, 4, 5, 6, 7, 8}
	serverPrivate := []byte{8, 7, 6, 5, 4, 3, 2, 1}

	fmt.Printf("Client private: %x\n", clientPrivate)
	fmt.Printf("Server private: %x\n", serverPrivate)

	// Generate public keys
	clientPublic := DHExchange(clientPrivate)
	serverPublic := DHExchange(serverPrivate)

	fmt.Printf("Client public: %x\n", clientPublic)
	fmt.Printf("Server public: %x\n", serverPublic)

	// Calculate shared secrets
	secretFromClient := DHSecret(clientPrivate, serverPublic)
	secretFromServer := DHSecret(serverPrivate, clientPublic)

	fmt.Printf("Secret from client: %x\n", secretFromClient)
	fmt.Printf("Secret from server: %x\n", secretFromServer)

	if string(secretFromClient) != string(secretFromServer) {
		t.Errorf("Secrets don't match")
	}
}
