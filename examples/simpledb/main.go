package simpledb

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/najoast/sngo/bootstrap"
	"github.com/najoast/sngo/core"
) // SimpleDB implements a simple in-memory key-value database
// 注意：Actor模式中，每个Actor内部是串行处理消息的，不需要锁！
type SimpleDB struct {
	data map[string]string
}

// NewSimpleDB creates a new simple database
func NewSimpleDB() *SimpleDB {
	return &SimpleDB{
		data: make(map[string]string),
	}
}

// DBRequest represents a database request
type DBRequest struct {
	Command string        `json:"command"`
	Args    []interface{} `json:"args"`
}

// DBResponse represents a database response
type DBResponse struct {
	Result interface{} `json:"result"`
	Error  string      `json:"error,omitempty"`
}

// HandleMessage implements the MessageHandler interface
func (db *SimpleDB) HandleMessage(ctx context.Context, msg *core.Message) error {
	switch msg.Type {
	case core.MessageTypeRequest:
		// Parse request
		var req DBRequest
		if err := json.Unmarshal(msg.Data, &req); err != nil {
			// Try to handle as string command
			command := string(msg.Data)
			parts := strings.Fields(command)
			if len(parts) == 0 {
				return fmt.Errorf("empty command")
			}

			req = DBRequest{
				Command: parts[0],
				Args:    make([]interface{}, len(parts)-1),
			}
			for i, arg := range parts[1:] {
				req.Args[i] = arg
			}
		}

		// Handle request
		response := db.handleRequest(ctx, &req)

		// Serialize response
		data, err := json.Marshal(response)
		if err != nil {
			return fmt.Errorf("failed to serialize response: %w", err)
		}

		// For now, just log the response
		// In a full implementation, we would send back a response message
		log.Printf("SimpleDB response: %s", string(data))
		return nil

	default:
		return fmt.Errorf("unsupported message type: %d", msg.Type)
	}
}

func (db *SimpleDB) handleRequest(ctx context.Context, req *DBRequest) *DBResponse {
	cmd := strings.ToUpper(req.Command)

	switch cmd {
	case "GET":
		if len(req.Args) != 1 {
			return &DBResponse{Error: "GET requires 1 argument"}
		}
		key, ok := req.Args[0].(string)
		if !ok {
			return &DBResponse{Error: "key must be string"}
		}

		result := db.get(key)
		return &DBResponse{Result: result}

	case "SET":
		if len(req.Args) != 2 {
			return &DBResponse{Error: "SET requires 2 arguments"}
		}
		key, ok := req.Args[0].(string)
		if !ok {
			return &DBResponse{Error: "key must be string"}
		}
		value, ok := req.Args[1].(string)
		if !ok {
			return &DBResponse{Error: "value must be string"}
		}

		old := db.set(key, value)
		return &DBResponse{Result: old}

	case "DELETE", "DEL":
		if len(req.Args) != 1 {
			return &DBResponse{Error: "DELETE requires 1 argument"}
		}
		key, ok := req.Args[0].(string)
		if !ok {
			return &DBResponse{Error: "key must be string"}
		}

		old := db.delete(key)
		return &DBResponse{Result: old}

	case "EXISTS":
		if len(req.Args) != 1 {
			return &DBResponse{Error: "EXISTS requires 1 argument"}
		}
		key, ok := req.Args[0].(string)
		if !ok {
			return &DBResponse{Error: "key must be string"}
		}

		exists := db.exists(key)
		return &DBResponse{Result: exists}

	case "KEYS":
		pattern := "*"
		if len(req.Args) > 0 {
			if p, ok := req.Args[0].(string); ok {
				pattern = p
			}
		}

		keys := db.keys(pattern)
		return &DBResponse{Result: keys}

	case "CLEAR":
		count := db.clear()
		return &DBResponse{Result: count}

	case "SIZE", "COUNT":
		size := db.size()
		return &DBResponse{Result: size}

	case "PING":
		// Handle ping command for compatibility
		msg := "PONG"
		if len(req.Args) > 0 {
			if s, ok := req.Args[0].(string); ok {
				msg = s
			}
		}
		log.Printf("SimpleDB received PING: %s", msg)
		return &DBResponse{Result: msg}

	default:
		return &DBResponse{Error: fmt.Sprintf("unknown command: %s", cmd)}
	}
}

func (db *SimpleDB) get(key string) string {
	// Actor模式：串行处理，无需锁
	return db.data[key]
}

func (db *SimpleDB) set(key, value string) string {
	// Actor模式：串行处理，无需锁
	old := db.data[key]
	db.data[key] = value
	return old
}

func (db *SimpleDB) delete(key string) string {
	// Actor模式：串行处理，无需锁
	old := db.data[key]
	delete(db.data, key)
	return old
}

func (db *SimpleDB) exists(key string) bool {
	// Actor模式：串行处理，无需锁
	_, exists := db.data[key]
	return exists
}

func (db *SimpleDB) keys(pattern string) []string {
	// Actor模式：串行处理，无需锁
	keys := make([]string, 0, len(db.data))
	for key := range db.data {
		if pattern == "*" || matchPattern(key, pattern) {
			keys = append(keys, key)
		}
	}
	return keys
}

func (db *SimpleDB) clear() int {
	// Actor模式：串行处理，无需锁
	count := len(db.data)
	db.data = make(map[string]string)
	return count
}

func (db *SimpleDB) size() int {
	// Actor模式：串行处理，无需锁
	return len(db.data)
}

// Simple pattern matching (supports * wildcard)
func matchPattern(text, pattern string) bool {
	if pattern == "*" {
		return true
	}

	// Simple implementation - can be enhanced for more complex patterns
	if strings.Contains(pattern, "*") {
		parts := strings.Split(pattern, "*")
		if len(parts) == 2 {
			prefix, suffix := parts[0], parts[1]
			return strings.HasPrefix(text, prefix) && strings.HasSuffix(text, suffix)
		}
	}

	return text == pattern
}

// SimpleDBService wraps SimpleDB as a bootstrap service
type SimpleDBService struct {
	db     *SimpleDB
	handle *core.Handle
	system core.ActorSystem
}

// NewSimpleDBService creates a new SimpleDB service
func NewSimpleDBService() *SimpleDBService {
	return &SimpleDBService{
		db: NewSimpleDB(),
	}
}

func (s *SimpleDBService) Name() string {
	return "simpledb"
}

func (s *SimpleDBService) Start(ctx context.Context) error {
	// ActorSystem会由bootstrap自动注册，但我们无法在这里访问container
	// 暂时创建一个本地的ActorSystem用于测试
	s.system = core.NewActorSystem()

	// Create service actor
	handle, err := s.system.NewService("SIMPLEDB", s.db, core.DefaultActorOptions())
	if err != nil {
		return fmt.Errorf("failed to create SimpleDB service: %w", err)
	}

	s.handle = handle

	log.Printf("SimpleDB service started with handle: %v", handle)
	return nil
}

func (s *SimpleDBService) Stop(ctx context.Context) error {
	if s.handle != nil && s.system != nil {
		// Service cleanup would be handled by the actor system
		// when the system shuts down
		log.Printf("SimpleDB service stopping")
	}
	return nil
}

func (s *SimpleDBService) Health(ctx context.Context) (bootstrap.HealthStatus, error) {
	if s.handle == nil {
		return bootstrap.HealthStatus{
			State:   bootstrap.HealthStopped,
			Message: "SimpleDB not running",
		}, nil
	}

	return bootstrap.HealthStatus{
		State:   bootstrap.HealthHealthy,
		Message: "SimpleDB operational",
	}, nil
}

// GetHandle returns the actor handle
func (s *SimpleDBService) GetHandle() *core.Handle {
	return s.handle
}
