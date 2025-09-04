// Package protocol provides protocol support for SNGO framework
package protocol

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

// Protocol represents a communication protocol
type Protocol interface {
	// Encode encodes a message to bytes
	Encode(msg interface{}) ([]byte, error)

	// Decode decodes bytes to a message
	Decode(data []byte, msg interface{}) error

	// GetMessageType returns the message type from data
	GetMessageType(data []byte) (string, error)

	// GetSession returns the session from data
	GetSession(data []byte) (uint32, error)
}

// SimpleProtocol implements a simple protocol similar to sproto
type SimpleProtocol struct {
	c2sSchema map[string]*MessageSchema
	s2cSchema map[string]*MessageSchema
}

// MessageSchema defines the structure of a message
type MessageSchema struct {
	Name     string                  `json:"name"`
	Type     int                     `json:"type"`
	Request  map[string]*FieldSchema `json:"request,omitempty"`
	Response map[string]*FieldSchema `json:"response,omitempty"`
}

// FieldSchema defines a field in a message
type FieldSchema struct {
	Tag  int    `json:"tag"`
	Type string `json:"type"`
}

// Package represents a protocol package header
type Package struct {
	Type    int    `json:"type"`
	Session uint32 `json:"session"`
}

// NewSimpleProtocol creates a new simple protocol
func NewSimpleProtocol() *SimpleProtocol {
	return &SimpleProtocol{
		c2sSchema: make(map[string]*MessageSchema),
		s2cSchema: make(map[string]*MessageSchema),
	}
}

// RegisterC2S registers a client-to-server message schema
func (sp *SimpleProtocol) RegisterC2S(name string, msgType int, request, response map[string]*FieldSchema) {
	sp.c2sSchema[name] = &MessageSchema{
		Name:     name,
		Type:     msgType,
		Request:  request,
		Response: response,
	}
}

// RegisterS2C registers a server-to-client message schema
func (sp *SimpleProtocol) RegisterS2C(name string, msgType int, request, response map[string]*FieldSchema) {
	sp.s2cSchema[name] = &MessageSchema{
		Name:     name,
		Type:     msgType,
		Request:  request,
		Response: response,
	}
}

// Encode encodes a message to bytes
func (sp *SimpleProtocol) Encode(msg interface{}) ([]byte, error) {
	// For simplicity, use JSON encoding
	// In a real implementation, this would use binary encoding
	return json.Marshal(msg)
}

// Decode decodes bytes to a message
func (sp *SimpleProtocol) Decode(data []byte, msg interface{}) error {
	return json.Unmarshal(data, msg)
}

// GetMessageType returns the message type from data
func (sp *SimpleProtocol) GetMessageType(data []byte) (string, error) {
	var pkg Package
	if err := json.Unmarshal(data, &pkg); err != nil {
		return "", fmt.Errorf("failed to decode package: %w", err)
	}

	// Find message name by type
	for name, schema := range sp.c2sSchema {
		if schema.Type == pkg.Type {
			return name, nil
		}
	}

	for name, schema := range sp.s2cSchema {
		if schema.Type == pkg.Type {
			return name, nil
		}
	}

	return "", fmt.Errorf("unknown message type: %d", pkg.Type)
}

// GetSession returns the session from data
func (sp *SimpleProtocol) GetSession(data []byte) (uint32, error) {
	var pkg Package
	if err := json.Unmarshal(data, &pkg); err != nil {
		return 0, fmt.Errorf("failed to decode package: %w", err)
	}

	return pkg.Session, nil
}

// DefaultProtocol creates the default protocol used in skynet examples
func DefaultProtocol() *SimpleProtocol {
	proto := NewSimpleProtocol()

	// Register C2S messages
	proto.RegisterC2S("handshake", 1, nil, map[string]*FieldSchema{
		"msg": {Tag: 0, Type: "string"},
	})

	proto.RegisterC2S("get", 2, map[string]*FieldSchema{
		"what": {Tag: 0, Type: "string"},
	}, map[string]*FieldSchema{
		"result": {Tag: 0, Type: "string"},
	})

	proto.RegisterC2S("set", 3, map[string]*FieldSchema{
		"what":  {Tag: 0, Type: "string"},
		"value": {Tag: 1, Type: "string"},
	}, nil)

	proto.RegisterC2S("quit", 4, nil, nil)

	// Register S2C messages
	proto.RegisterS2C("heartbeat", 1, nil, nil)

	return proto
}

// BinaryProtocol implements a binary protocol similar to skynet's format
type BinaryProtocol struct {
	*SimpleProtocol
}

// NewBinaryProtocol creates a new binary protocol
func NewBinaryProtocol() *BinaryProtocol {
	return &BinaryProtocol{
		SimpleProtocol: DefaultProtocol(),
	}
}

// Encode encodes a message to binary format
func (bp *BinaryProtocol) Encode(msg interface{}) ([]byte, error) {
	// Convert message to map for binary encoding
	msgMap, err := structToMap(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to convert message to map: %w", err)
	}

	// Simple binary encoding: length + JSON data
	jsonData, err := json.Marshal(msgMap)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal message: %w", err)
	}

	// Prepend 2-byte length header
	length := uint16(len(jsonData))
	result := make([]byte, 2+len(jsonData))
	result[0] = byte(length >> 8)
	result[1] = byte(length & 0xFF)
	copy(result[2:], jsonData)

	return result, nil
}

// Decode decodes binary data to a message
func (bp *BinaryProtocol) Decode(data []byte, msg interface{}) error {
	if len(data) < 2 {
		return fmt.Errorf("data too short")
	}

	// Read length header
	length := uint16(data[0])<<8 | uint16(data[1])
	if len(data) < int(length)+2 {
		return fmt.Errorf("incomplete data")
	}

	// Decode JSON data
	jsonData := data[2 : 2+length]
	return json.Unmarshal(jsonData, msg)
}

// PackMessage packages a message with type and session
func PackMessage(msgType int, session uint32, data []byte) []byte {
	// Create package header
	pkg := Package{
		Type:    msgType,
		Session: session,
	}

	header, _ := json.Marshal(pkg)

	// Combine header and data
	result := make([]byte, len(header)+len(data))
	copy(result, header)
	copy(result[len(header):], data)

	return result
}

// UnpackMessage unpacks a message to get type, session and data
func UnpackMessage(data []byte) (int, uint32, []byte, error) {
	// For simplicity, assume the package header is JSON
	// In real implementation, this would use binary format

	// Find the end of header (look for first '{' and matching '}')
	headerEnd := findJSONEnd(data)
	if headerEnd == -1 {
		return 0, 0, nil, fmt.Errorf("invalid message format")
	}

	var pkg Package
	if err := json.Unmarshal(data[:headerEnd], &pkg); err != nil {
		return 0, 0, nil, fmt.Errorf("failed to decode package header: %w", err)
	}

	return pkg.Type, pkg.Session, data[headerEnd:], nil
}

// Helper functions

func structToMap(obj interface{}) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	v := reflect.ValueOf(obj)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return nil, fmt.Errorf("object is not a struct")
	}

	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		value := v.Field(i)

		if !value.CanInterface() {
			continue
		}

		// Use json tag if available
		name := field.Name
		if tag := field.Tag.Get("json"); tag != "" {
			if comma := strings.Index(tag, ","); comma != -1 {
				name = tag[:comma]
			} else {
				name = tag
			}
		}

		result[name] = value.Interface()
	}

	return result, nil
}

func findJSONEnd(data []byte) int {
	braceCount := 0
	inString := false
	escape := false

	for i, b := range data {
		if escape {
			escape = false
			continue
		}

		if b == '\\' {
			escape = true
			continue
		}

		if b == '"' {
			inString = !inString
			continue
		}

		if inString {
			continue
		}

		if b == '{' {
			braceCount++
		} else if b == '}' {
			braceCount--
			if braceCount == 0 {
				return i + 1
			}
		}
	}

	return -1
}

// Message types for skynet compatibility

// HandshakeRequest represents a handshake request
type HandshakeRequest struct {
	Package
}

// HandshakeResponse represents a handshake response
type HandshakeResponse struct {
	Package
	Msg string `json:"msg"`
}

// GetRequest represents a get request
type GetRequest struct {
	Package
	What string `json:"what"`
}

// GetResponse represents a get response
type GetResponse struct {
	Package
	Result string `json:"result"`
}

// SetRequest represents a set request
type SetRequest struct {
	Package
	What  string `json:"what"`
	Value string `json:"value"`
}

// SetResponse represents a set response
type SetResponse struct {
	Package
}

// QuitRequest represents a quit request
type QuitRequest struct {
	Package
}

// HeartbeatMessage represents a heartbeat message
type HeartbeatMessage struct {
	Package
}
