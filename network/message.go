// Package network provides message types for network communication
package network

import (
	"encoding/binary"
	"fmt"
	"time"
)

// MessageType defines the type of network message
type MessageType uint32

const (
	// System message types (0-99)
	MessageTypeHeartbeat MessageType = 1
	MessageTypeAck       MessageType = 2
	MessageTypeError     MessageType = 3
	MessageTypeClose     MessageType = 4

	// User message types (100+)
	MessageTypeUserStart MessageType = 100
	MessageTypeRPC       MessageType = 101
	MessageTypeData      MessageType = 102
	MessageTypeBroadcast MessageType = 103
)

// String returns the string representation of MessageType
func (mt MessageType) String() string {
	switch mt {
	case MessageTypeHeartbeat:
		return "heartbeat"
	case MessageTypeAck:
		return "ack"
	case MessageTypeError:
		return "error"
	case MessageTypeClose:
		return "close"
	case MessageTypeRPC:
		return "rpc"
	case MessageTypeData:
		return "data"
	case MessageTypeBroadcast:
		return "broadcast"
	default:
		return fmt.Sprintf("unknown(%d)", mt)
	}
}

// MessageFlag defines message flags
type MessageFlag uint32

const (
	MessageFlagNone       MessageFlag = 0
	MessageFlagCompressed MessageFlag = 1 << 0
	MessageFlagEncrypted  MessageFlag = 1 << 1
	MessageFlagPriority   MessageFlag = 1 << 2
	MessageFlagReliable   MessageFlag = 1 << 3
	MessageFlagOrderedA   MessageFlag = 1 << 4
)

// Message represents a network message with header and payload
type Message struct {
	// Header fields
	Type      MessageType `json:"type"`
	Flags     MessageFlag `json:"flags"`
	Sequence  uint32      `json:"sequence"`
	SessionID uint64      `json:"session_id"`

	// Routing information
	Source      string `json:"source,omitempty"`
	Destination string `json:"destination,omitempty"`

	// Timing information
	Timestamp time.Time `json:"timestamp"`
	TTL       uint32    `json:"ttl,omitempty"`

	// Payload
	Data []byte `json:"data,omitempty"`

	// Metadata for internal use
	ConnectionID string      `json:"-"`
	UserData     interface{} `json:"-"`
}

// NewMessage creates a new message with the specified type and data
func NewMessage(msgType MessageType, data []byte) *Message {
	return &Message{
		Type:      msgType,
		Flags:     MessageFlagNone,
		Sequence:  0,
		SessionID: 0,
		Timestamp: time.Now(),
		Data:      data,
	}
}

// NewHeartbeatMessage creates a heartbeat message
func NewHeartbeatMessage() *Message {
	return NewMessage(MessageTypeHeartbeat, nil)
}

// NewAckMessage creates an acknowledgment message
func NewAckMessage(sequence uint32) *Message {
	msg := NewMessage(MessageTypeAck, nil)
	msg.Sequence = sequence
	return msg
}

// NewErrorMessage creates an error message
func NewErrorMessage(errorMsg string) *Message {
	return NewMessage(MessageTypeError, []byte(errorMsg))
}

// NewRPCMessage creates an RPC message
func NewRPCMessage(source, destination string, data []byte) *Message {
	msg := NewMessage(MessageTypeRPC, data)
	msg.Source = source
	msg.Destination = destination
	return msg
}

// SetFlag sets a message flag
func (m *Message) SetFlag(flag MessageFlag) {
	m.Flags |= flag
}

// ClearFlag clears a message flag
func (m *Message) ClearFlag(flag MessageFlag) {
	m.Flags &^= flag
}

// HasFlag checks if a message flag is set
func (m *Message) HasFlag(flag MessageFlag) bool {
	return m.Flags&flag != 0
}

// Size returns the total size of the message in bytes
func (m *Message) Size() int {
	return MessageHeaderSize + len(m.Data)
}

// IsExpired checks if the message has expired based on TTL
func (m *Message) IsExpired() bool {
	if m.TTL == 0 {
		return false
	}
	return time.Since(m.Timestamp) > time.Duration(m.TTL)*time.Second
}

// Clone creates a deep copy of the message
func (m *Message) Clone() *Message {
	clone := &Message{
		Type:         m.Type,
		Flags:        m.Flags,
		Sequence:     m.Sequence,
		SessionID:    m.SessionID,
		Source:       m.Source,
		Destination:  m.Destination,
		Timestamp:    m.Timestamp,
		TTL:          m.TTL,
		ConnectionID: m.ConnectionID,
		UserData:     m.UserData,
	}

	if m.Data != nil {
		clone.Data = make([]byte, len(m.Data))
		copy(clone.Data, m.Data)
	}

	return clone
}

// Constants for message serialization
const (
	// MessageHeaderSize is the fixed size of the message header in bytes
	MessageHeaderSize = 32

	// MaxMessageSize is the maximum allowed message size
	MaxMessageSize = 64 * 1024 * 1024 // 64MB

	// MaxDataSize is the maximum allowed data payload size
	MaxDataSize = MaxMessageSize - MessageHeaderSize
)

// MessageCodec handles message encoding and decoding
type MessageCodec interface {
	// Encode encodes a message to bytes
	Encode(msg *Message) ([]byte, error)

	// Decode decodes bytes to a message
	Decode(data []byte) (*Message, error)

	// EncodeHeader encodes only the message header
	EncodeHeader(msg *Message) ([]byte, error)

	// DecodeHeader decodes only the message header
	DecodeHeader(data []byte) (*Message, error)
}

// BinaryMessageCodec implements MessageCodec using binary encoding
type BinaryMessageCodec struct{}

// NewBinaryMessageCodec creates a new binary message codec
func NewBinaryMessageCodec() *BinaryMessageCodec {
	return &BinaryMessageCodec{}
}

// Encode encodes a message to binary format
func (c *BinaryMessageCodec) Encode(msg *Message) ([]byte, error) {
	if msg == nil {
		return nil, fmt.Errorf("message is nil")
	}

	dataLen := len(msg.Data)
	if dataLen > MaxDataSize {
		return nil, fmt.Errorf("message data too large: %d bytes (max %d)", dataLen, MaxDataSize)
	}

	// Calculate total size
	totalSize := MessageHeaderSize + dataLen
	buf := make([]byte, totalSize)

	// Encode header
	binary.BigEndian.PutUint32(buf[0:4], uint32(msg.Type))
	binary.BigEndian.PutUint32(buf[4:8], uint32(msg.Flags))
	binary.BigEndian.PutUint32(buf[8:12], msg.Sequence)
	binary.BigEndian.PutUint64(buf[12:20], msg.SessionID)
	binary.BigEndian.PutUint64(buf[20:28], uint64(msg.Timestamp.Unix()))
	binary.BigEndian.PutUint32(buf[28:32], uint32(dataLen))

	// Copy data
	if dataLen > 0 {
		copy(buf[MessageHeaderSize:], msg.Data)
	}

	return buf, nil
}

// Decode decodes binary data to a message
func (c *BinaryMessageCodec) Decode(data []byte) (*Message, error) {
	if len(data) < MessageHeaderSize {
		return nil, fmt.Errorf("data too short for message header: %d bytes", len(data))
	}

	// Decode header
	msg := &Message{
		Type:      MessageType(binary.BigEndian.Uint32(data[0:4])),
		Flags:     MessageFlag(binary.BigEndian.Uint32(data[4:8])),
		Sequence:  binary.BigEndian.Uint32(data[8:12]),
		SessionID: binary.BigEndian.Uint64(data[12:20]),
		Timestamp: time.Unix(int64(binary.BigEndian.Uint64(data[20:28])), 0),
	}

	dataLen := binary.BigEndian.Uint32(data[28:32])

	// Validate data length
	if int(dataLen) > MaxDataSize {
		return nil, fmt.Errorf("message data too large: %d bytes (max %d)", dataLen, MaxDataSize)
	}

	if len(data) < MessageHeaderSize+int(dataLen) {
		return nil, fmt.Errorf("data too short for message: expected %d, got %d",
			MessageHeaderSize+int(dataLen), len(data))
	}

	// Copy data
	if dataLen > 0 {
		msg.Data = make([]byte, dataLen)
		copy(msg.Data, data[MessageHeaderSize:MessageHeaderSize+int(dataLen)])
	}

	return msg, nil
}

// EncodeHeader encodes only the message header
func (c *BinaryMessageCodec) EncodeHeader(msg *Message) ([]byte, error) {
	if msg == nil {
		return nil, fmt.Errorf("message is nil")
	}

	buf := make([]byte, MessageHeaderSize)

	binary.BigEndian.PutUint32(buf[0:4], uint32(msg.Type))
	binary.BigEndian.PutUint32(buf[4:8], uint32(msg.Flags))
	binary.BigEndian.PutUint32(buf[8:12], msg.Sequence)
	binary.BigEndian.PutUint64(buf[12:20], msg.SessionID)
	binary.BigEndian.PutUint64(buf[20:28], uint64(msg.Timestamp.Unix()))
	binary.BigEndian.PutUint32(buf[28:32], uint32(len(msg.Data)))

	return buf, nil
}

// DecodeHeader decodes only the message header
func (c *BinaryMessageCodec) DecodeHeader(data []byte) (*Message, error) {
	if len(data) < MessageHeaderSize {
		return nil, fmt.Errorf("data too short for message header: %d bytes", len(data))
	}

	msg := &Message{
		Type:      MessageType(binary.BigEndian.Uint32(data[0:4])),
		Flags:     MessageFlag(binary.BigEndian.Uint32(data[4:8])),
		Sequence:  binary.BigEndian.Uint32(data[8:12]),
		SessionID: binary.BigEndian.Uint64(data[12:20]),
		Timestamp: time.Unix(int64(binary.BigEndian.Uint64(data[20:28])), 0),
	}

	dataLen := binary.BigEndian.Uint32(data[28:32])
	if int(dataLen) > MaxDataSize {
		return nil, fmt.Errorf("message data too large: %d bytes (max %d)", dataLen, MaxDataSize)
	}

	// Don't decode the data, just set the expected length
	if dataLen > 0 {
		msg.Data = make([]byte, 0, dataLen) // Reserve capacity but don't copy data
	}

	return msg, nil
}
