package client

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"time"
)

// SimpleClient implements a basic client to test SNGO services
type SimpleClient struct {
	conn net.Conn
	addr string
	port int
}

// NewSimpleClient creates a new simple client
func NewSimpleClient(addr string, port int) *SimpleClient {
	return &SimpleClient{
		addr: addr,
		port: port,
	}
}

// Connect connects to the server
func (c *SimpleClient) Connect() error {
	address := net.JoinHostPort(c.addr, fmt.Sprintf("%d", c.port))

	conn, err := net.Dial("tcp", address)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", address, err)
	}

	c.conn = conn
	log.Printf("Connected to %s", address)
	return nil
}

// SendMessage sends a message to the server
func (c *SimpleClient) SendMessage(message string) error {
	if c.conn == nil {
		return fmt.Errorf("not connected")
	}

	// Simple protocol: 2-byte length + message
	msgBytes := []byte(message)
	length := uint16(len(msgBytes))

	// Send length
	if err := binary.Write(c.conn, binary.BigEndian, length); err != nil {
		return fmt.Errorf("failed to send length: %w", err)
	}

	// Send message
	if _, err := c.conn.Write(msgBytes); err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	log.Printf("Sent message: %s", message)
	return nil
}

// SendJSON sends a JSON message to the server
func (c *SimpleClient) SendJSON(data interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	return c.SendMessage(string(jsonData))
}

// ReceiveMessage receives a message from the server
func (c *SimpleClient) ReceiveMessage() (string, error) {
	if c.conn == nil {
		return "", fmt.Errorf("not connected")
	}

	// Read length
	var length uint16
	if err := binary.Read(c.conn, binary.BigEndian, &length); err != nil {
		return "", fmt.Errorf("failed to read length: %w", err)
	}

	// Read message
	msgBytes := make([]byte, length)
	if _, err := io.ReadFull(c.conn, msgBytes); err != nil {
		return "", fmt.Errorf("failed to read message: %w", err)
	}

	message := string(msgBytes)
	log.Printf("Received message: %s", message)
	return message, nil
}

// Close closes the connection
func (c *SimpleClient) Close() error {
	if c.conn != nil {
		err := c.conn.Close()
		c.conn = nil
		log.Printf("Connection closed")
		return err
	}
	return nil
}

// RunTest runs a simple test against the SNGO services
func RunTest() {
	log.Println("Starting SNGO Client Test")
	log.Println("========================")

	// Test 1: Connect to gate (if running)
	client := NewSimpleClient("127.0.0.1", 8888)

	log.Println("Attempting to connect to gate...")
	if err := client.Connect(); err != nil {
		log.Printf("Failed to connect to gate: %v", err)
		log.Println("Make sure the gate service is running and listening on port 8888")
		return
	}

	defer client.Close()

	// Test 2: Send login message
	loginMsg := map[string]interface{}{
		"type":     "login",
		"username": "testuser",
		"password": "testpass",
	}

	log.Println("Sending login message...")
	if err := client.SendJSON(loginMsg); err != nil {
		log.Printf("Failed to send login: %v", err)
		return
	}

	// Test 3: Send chat message
	time.Sleep(100 * time.Millisecond)

	chatMsg := map[string]interface{}{
		"type":    "chat",
		"message": "Hello, SNGO!",
	}

	log.Println("Sending chat message...")
	if err := client.SendJSON(chatMsg); err != nil {
		log.Printf("Failed to send chat: %v", err)
		return
	}

	// Test 4: Send ping
	time.Sleep(100 * time.Millisecond)

	pingMsg := map[string]interface{}{
		"type":      "ping",
		"timestamp": time.Now().Unix(),
	}

	log.Println("Sending ping...")
	if err := client.SendJSON(pingMsg); err != nil {
		log.Printf("Failed to send ping: %v", err)
		return
	}

	// Wait a bit to see server responses
	log.Println("Waiting for server responses...")
	time.Sleep(2 * time.Second)

	log.Println("Client test completed successfully!")
}
