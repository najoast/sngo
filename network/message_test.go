// Package network provides tests for message encoding/decoding
package network

import (
	"testing"
	"time"
)

func TestMessage(t *testing.T) {
	t.Run("NewMessage", func(t *testing.T) {
		data := []byte("test data")
		msg := NewMessage(MessageTypeData, data)

		if msg.Type != MessageTypeData {
			t.Errorf("Expected type %v, got %v", MessageTypeData, msg.Type)
		}

		if string(msg.Data) != string(data) {
			t.Errorf("Expected data %s, got %s", string(data), string(msg.Data))
		}

		if msg.Flags != MessageFlagNone {
			t.Errorf("Expected flags %v, got %v", MessageFlagNone, msg.Flags)
		}
	})

	t.Run("SpecialMessages", func(t *testing.T) {
		// Test heartbeat message
		heartbeat := NewHeartbeatMessage()
		if heartbeat.Type != MessageTypeHeartbeat {
			t.Errorf("Expected heartbeat type, got %v", heartbeat.Type)
		}
		if len(heartbeat.Data) != 0 {
			t.Errorf("Expected empty data for heartbeat, got %d bytes", len(heartbeat.Data))
		}

		// Test ACK message
		ack := NewAckMessage(123)
		if ack.Type != MessageTypeAck {
			t.Errorf("Expected ACK type, got %v", ack.Type)
		}
		if ack.Sequence != 123 {
			t.Errorf("Expected sequence 123, got %v", ack.Sequence)
		}

		// Test error message
		errorMsg := NewErrorMessage("test error")
		if errorMsg.Type != MessageTypeError {
			t.Errorf("Expected error type, got %v", errorMsg.Type)
		}
		if string(errorMsg.Data) != "test error" {
			t.Errorf("Expected error data 'test error', got %s", string(errorMsg.Data))
		}

		// Test RPC message
		rpc := NewRPCMessage("source", "dest", []byte("rpc data"))
		if rpc.Type != MessageTypeRPC {
			t.Errorf("Expected RPC type, got %v", rpc.Type)
		}
		if rpc.Source != "source" {
			t.Errorf("Expected source 'source', got %s", rpc.Source)
		}
		if rpc.Destination != "dest" {
			t.Errorf("Expected destination 'dest', got %s", rpc.Destination)
		}
	})

	t.Run("MessageFlags", func(t *testing.T) {
		msg := NewMessage(MessageTypeData, []byte("test"))

		// Test setting flags
		msg.SetFlag(MessageFlagCompressed)
		if !msg.HasFlag(MessageFlagCompressed) {
			t.Error("Expected compressed flag to be set")
		}

		msg.SetFlag(MessageFlagEncrypted)
		if !msg.HasFlag(MessageFlagEncrypted) {
			t.Error("Expected encrypted flag to be set")
		}

		// Test clearing flags
		msg.ClearFlag(MessageFlagCompressed)
		if msg.HasFlag(MessageFlagCompressed) {
			t.Error("Expected compressed flag to be cleared")
		}

		if !msg.HasFlag(MessageFlagEncrypted) {
			t.Error("Expected encrypted flag to still be set")
		}
	})

	t.Run("MessageExpiration", func(t *testing.T) {
		msg := NewMessage(MessageTypeData, []byte("test"))

		// Message without TTL should not expire
		if msg.IsExpired() {
			t.Error("Message without TTL should not expire")
		}

		// Message with TTL in the future should not expire
		msg.TTL = 10 // 10 seconds
		if msg.IsExpired() {
			t.Error("Message with future TTL should not expire")
		}

		// Message with past timestamp should expire
		msg.Timestamp = time.Now().Add(-20 * time.Second)
		if !msg.IsExpired() {
			t.Error("Message with past timestamp should expire")
		}
	})

	t.Run("MessageClone", func(t *testing.T) {
		original := NewRPCMessage("src", "dst", []byte("original data"))
		original.SetFlag(MessageFlagCompressed)
		original.Sequence = 42
		original.SessionID = 123
		original.TTL = 30

		clone := original.Clone()

		// Check that all fields are copied
		if clone.Type != original.Type {
			t.Errorf("Clone type mismatch: expected %v, got %v", original.Type, clone.Type)
		}
		if clone.Flags != original.Flags {
			t.Errorf("Clone flags mismatch: expected %v, got %v", original.Flags, clone.Flags)
		}
		if clone.Sequence != original.Sequence {
			t.Errorf("Clone sequence mismatch: expected %v, got %v", original.Sequence, clone.Sequence)
		}
		if clone.SessionID != original.SessionID {
			t.Errorf("Clone session ID mismatch: expected %v, got %v", original.SessionID, clone.SessionID)
		}
		if clone.Source != original.Source {
			t.Errorf("Clone source mismatch: expected %v, got %v", original.Source, clone.Source)
		}
		if clone.Destination != original.Destination {
			t.Errorf("Clone destination mismatch: expected %v, got %v", original.Destination, clone.Destination)
		}
		if clone.TTL != original.TTL {
			t.Errorf("Clone TTL mismatch: expected %v, got %v", original.TTL, clone.TTL)
		}

		// Check that data is deep copied
		if string(clone.Data) != string(original.Data) {
			t.Errorf("Clone data mismatch: expected %s, got %s", string(original.Data), string(clone.Data))
		}

		// Modify clone data to ensure it's independent
		clone.Data[0] = 'X'
		if original.Data[0] == 'X' {
			t.Error("Clone data should be independent of original")
		}
	})
}

func TestBinaryMessageCodec(t *testing.T) {
	codec := NewBinaryMessageCodec()

	t.Run("EncodeDecodeEmpty", func(t *testing.T) {
		original := NewHeartbeatMessage()

		// Encode
		data, err := codec.Encode(original)
		if err != nil {
			t.Fatalf("Failed to encode message: %v", err)
		}

		if len(data) != MessageHeaderSize {
			t.Errorf("Expected encoded size %d, got %d", MessageHeaderSize, len(data))
		}

		// Decode
		decoded, err := codec.Decode(data)
		if err != nil {
			t.Fatalf("Failed to decode message: %v", err)
		}

		// Compare
		if decoded.Type != original.Type {
			t.Errorf("Type mismatch: expected %v, got %v", original.Type, decoded.Type)
		}
		if decoded.Flags != original.Flags {
			t.Errorf("Flags mismatch: expected %v, got %v", original.Flags, decoded.Flags)
		}
		if len(decoded.Data) != 0 {
			t.Errorf("Expected empty data, got %d bytes", len(decoded.Data))
		}
	})

	t.Run("EncodeDecodeWithData", func(t *testing.T) {
		testData := []byte("Hello, SNGO Network!")
		original := NewMessage(MessageTypeData, testData)
		original.Sequence = 42
		original.SessionID = 123456
		original.SetFlag(MessageFlagCompressed | MessageFlagPriority)

		// Encode
		data, err := codec.Encode(original)
		if err != nil {
			t.Fatalf("Failed to encode message: %v", err)
		}

		expectedSize := MessageHeaderSize + len(testData)
		if len(data) != expectedSize {
			t.Errorf("Expected encoded size %d, got %d", expectedSize, len(data))
		}

		// Decode
		decoded, err := codec.Decode(data)
		if err != nil {
			t.Fatalf("Failed to decode message: %v", err)
		}

		// Compare all fields
		if decoded.Type != original.Type {
			t.Errorf("Type mismatch: expected %v, got %v", original.Type, decoded.Type)
		}
		if decoded.Flags != original.Flags {
			t.Errorf("Flags mismatch: expected %v, got %v", original.Flags, decoded.Flags)
		}
		if decoded.Sequence != original.Sequence {
			t.Errorf("Sequence mismatch: expected %v, got %v", original.Sequence, decoded.Sequence)
		}
		if decoded.SessionID != original.SessionID {
			t.Errorf("SessionID mismatch: expected %v, got %v", original.SessionID, decoded.SessionID)
		}
		if string(decoded.Data) != string(testData) {
			t.Errorf("Data mismatch: expected %s, got %s", string(testData), string(decoded.Data))
		}
	})

	t.Run("HeaderOnlyEncodeDecode", func(t *testing.T) {
		original := NewMessage(MessageTypeRPC, []byte("test data"))
		original.Sequence = 99
		original.SessionID = 888

		// Encode header only
		headerData, err := codec.EncodeHeader(original)
		if err != nil {
			t.Fatalf("Failed to encode header: %v", err)
		}

		if len(headerData) != MessageHeaderSize {
			t.Errorf("Expected header size %d, got %d", MessageHeaderSize, len(headerData))
		}

		// Decode header only
		decoded, err := codec.DecodeHeader(headerData)
		if err != nil {
			t.Fatalf("Failed to decode header: %v", err)
		}

		// Compare header fields
		if decoded.Type != original.Type {
			t.Errorf("Type mismatch: expected %v, got %v", original.Type, decoded.Type)
		}
		if decoded.Sequence != original.Sequence {
			t.Errorf("Sequence mismatch: expected %v, got %v", original.Sequence, decoded.Sequence)
		}
		if decoded.SessionID != original.SessionID {
			t.Errorf("SessionID mismatch: expected %v, got %v", original.SessionID, decoded.SessionID)
		}

		// Data should be empty but with correct capacity
		if len(decoded.Data) != 0 {
			t.Errorf("Expected empty data, got %d bytes", len(decoded.Data))
		}
		if cap(decoded.Data) != len(original.Data) {
			t.Errorf("Expected data capacity %d, got %d", len(original.Data), cap(decoded.Data))
		}
	})

	t.Run("ErrorCases", func(t *testing.T) {
		// Test nil message
		_, err := codec.Encode(nil)
		if err == nil {
			t.Error("Expected error for nil message")
		}

		// Test empty data
		_, err = codec.Decode([]byte{})
		if err == nil {
			t.Error("Expected error for empty data")
		}

		// Test truncated data
		original := NewMessage(MessageTypeData, []byte("test"))
		data, _ := codec.Encode(original)
		truncated := data[:len(data)-2] // Remove last 2 bytes
		_, err = codec.Decode(truncated)
		if err == nil {
			t.Error("Expected error for truncated data")
		}

		// Test oversized message
		oversized := NewMessage(MessageTypeData, make([]byte, MaxDataSize+1))
		_, err = codec.Encode(oversized)
		if err == nil {
			t.Error("Expected error for oversized message")
		}
	})
}

func TestMessageTypeString(t *testing.T) {
	tests := []struct {
		msgType  MessageType
		expected string
	}{
		{MessageTypeHeartbeat, "heartbeat"},
		{MessageTypeAck, "ack"},
		{MessageTypeError, "error"},
		{MessageTypeClose, "close"},
		{MessageTypeRPC, "rpc"},
		{MessageTypeData, "data"},
		{MessageTypeBroadcast, "broadcast"},
		{MessageType(999), "unknown(999)"},
	}

	for _, test := range tests {
		result := test.msgType.String()
		if result != test.expected {
			t.Errorf("MessageType(%d).String() = %s, expected %s",
				test.msgType, result, test.expected)
		}
	}
}

func TestConnectionStateString(t *testing.T) {
	tests := []struct {
		state    ConnectionState
		expected string
	}{
		{ConnectionStateConnected, "connected"},
		{ConnectionStateDisconnected, "disconnected"},
		{ConnectionStateReconnecting, "reconnecting"},
		{ConnectionStateClosed, "closed"},
		{ConnectionState(999), "unknown"},
	}

	for _, test := range tests {
		result := test.state.String()
		if result != test.expected {
			t.Errorf("ConnectionState(%d).String() = %s, expected %s",
				test.state, result, test.expected)
		}
	}
}
