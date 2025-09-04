package gate

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"sync"

	"github.com/najoast/sngo/bootstrap"
	"github.com/najoast/sngo/core"
)

// Connection represents a client connection
type Connection struct {
	FD     int          `json:"fd"`
	IP     string       `json:"ip"`
	Client int          `json:"client"`
	Agent  core.ActorID `json:"agent"`
	Mode   string       `json:"mode"`
	Conn   net.Conn     `json:"-"` // actual network connection
}

// Gate manages network connections and forwards messages
// 这是skynet中的Gate服务：管理网络连接，转发消息给Agent
type Gate struct {
	watchdog    core.ActorID
	listener    net.Listener
	connections sync.Map // fd -> *Connection
	nextFD      int
	system      core.ActorSystem
	running     bool
	mu          sync.RWMutex
}

// NewGate creates a new gate
func NewGate() *Gate {
	return &Gate{
		nextFD: 1,
	}
}

// GateConfig represents gate configuration
type GateConfig struct {
	Watchdog string `json:"watchdog"`
	Address  string `json:"address"`
	Port     int    `json:"port"`
}

// GateCommand represents commands to gate
type GateCommand struct {
	Command string        `json:"command"`
	Args    []interface{} `json:"args"`
}

// HandleMessage implements the MessageHandler interface
func (g *Gate) HandleMessage(ctx context.Context, msg *core.Message) error {
	switch msg.Type {
	case core.MessageTypeRequest:
		return g.handleCommand(ctx, msg)
	case core.MessageTypeText:
		return g.handleClientData(ctx, msg)
	default:
		return fmt.Errorf("unsupported message type: %d", msg.Type)
	}
}

func (g *Gate) handleCommand(ctx context.Context, msg *core.Message) error {
	var cmd GateCommand
	if err := json.Unmarshal(msg.Data, &cmd); err != nil {
		return fmt.Errorf("failed to parse command: %w", err)
	}

	switch cmd.Command {
	case "open":
		// Open gate with configuration
		if len(cmd.Args) > 0 {
			if config, ok := cmd.Args[0].(map[string]interface{}); ok {
				return g.open(config)
			}
		}
		return fmt.Errorf("open command requires configuration")

	case "forward":
		// Forward connection to agent
		if len(cmd.Args) >= 3 {
			fd := int(cmd.Args[0].(float64))
			client := int(cmd.Args[1].(float64))
			agentID := core.ActorID(cmd.Args[2].(float64))
			return g.forward(fd, client, agentID)
		}
		return fmt.Errorf("forward command requires fd, client, agent")

	case "accept":
		// Accept connection
		if len(cmd.Args) >= 1 {
			fd := int(cmd.Args[0].(float64))
			return g.accept(fd)
		}
		return fmt.Errorf("accept command requires fd")

	case "kick":
		// Kick connection
		if len(cmd.Args) >= 1 {
			fd := int(cmd.Args[0].(float64))
			return g.kick(fd)
		}
		return fmt.Errorf("kick command requires fd")

	default:
		return fmt.Errorf("unknown command: %s", cmd.Command)
	}
}

func (g *Gate) handleClientData(ctx context.Context, msg *core.Message) error {
	// Handle data from client connections
	log.Printf("Gate received client data: %s", string(msg.Data))
	return nil
}

func (g *Gate) open(config map[string]interface{}) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.running {
		return fmt.Errorf("gate already running")
	}

	// Extract configuration
	address := "127.0.0.1"
	port := 8888

	if addr, ok := config["address"].(string); ok {
		address = addr
	}
	if p, ok := config["port"].(float64); ok {
		port = int(p)
	}
	if p, ok := config["port"].(int); ok {
		port = p
	}

	// Start listening
	listenAddr := fmt.Sprintf("%s:%d", address, port)
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", listenAddr, err)
	}

	g.listener = listener
	g.running = true

	log.Printf("Gate listening on %s", listenAddr)

	// Start accepting connections in background
	go g.acceptLoop()

	return nil
}

func (g *Gate) acceptLoop() {
	for g.running {
		conn, err := g.listener.Accept()
		if err != nil {
			if g.running {
				log.Printf("Gate accept error: %v", err)
			}
			continue
		}

		// Handle new connection
		go g.handleNewConnection(conn)
	}
}

func (g *Gate) handleNewConnection(conn net.Conn) {
	g.mu.Lock()
	fd := g.nextFD
	g.nextFD++
	g.mu.Unlock()

	// Create connection record
	connection := &Connection{
		FD:   fd,
		IP:   conn.RemoteAddr().String(),
		Conn: conn,
	}

	g.connections.Store(fd, connection)

	log.Printf("New connection from %s (fd: %d)", connection.IP, fd)

	// Notify watchdog
	g.notifyWatchdog("open", fd, connection.IP)

	// Start reading from connection
	go g.readFromConnection(connection)
}

func (g *Gate) readFromConnection(conn *Connection) {
	defer func() {
		conn.Conn.Close()
		g.connections.Delete(conn.FD)
		g.notifyWatchdog("close", conn.FD, "")
	}()

	buffer := make([]byte, 4096)
	for {
		n, err := conn.Conn.Read(buffer)
		if err != nil {
			if err != net.ErrClosed {
				log.Printf("Connection %d read error: %v", conn.FD, err)
				g.notifyWatchdog("error", conn.FD, err.Error())
			}
			break
		}

		data := buffer[:n]

		// If connection has agent, forward to agent
		if conn.Agent != 0 {
			g.forwardToAgent(conn, data)
		} else {
			// Otherwise notify watchdog
			g.notifyWatchdog("data", conn.FD, string(data))
		}
	}
}

func (g *Gate) forwardToAgent(conn *Connection, data []byte) {
	// Forward message to agent
	if g.system != nil {
		// In real implementation, we would use proper message protocol
		err := g.system.Send(conn.Agent, conn.Agent, core.MessageTypeText, data)
		if err != nil {
			log.Printf("Failed to forward data to agent %d: %v", conn.Agent, err)
		}
	}
}

func (g *Gate) notifyWatchdog(eventType string, fd int, data string) {
	if g.watchdog == 0 {
		return
	}

	event := map[string]interface{}{
		"type": eventType,
		"fd":   fd,
	}

	switch eventType {
	case "open":
		event["address"] = data
	case "error", "data":
		event["message"] = data
	}

	eventData, _ := json.Marshal(event)

	if g.system != nil {
		g.system.Send(g.watchdog, g.watchdog, core.MessageTypeText, eventData)
	}
}

func (g *Gate) forward(fd, client int, agentID core.ActorID) error {
	if connValue, ok := g.connections.Load(fd); ok {
		conn := connValue.(*Connection)
		conn.Client = client
		conn.Agent = agentID
		log.Printf("Connection %d forwarded to agent %d", fd, agentID)
		return nil
	}
	return fmt.Errorf("connection %d not found", fd)
}

func (g *Gate) accept(fd int) error {
	if connValue, ok := g.connections.Load(fd); ok {
		conn := connValue.(*Connection)
		conn.Mode = "accepted"
		log.Printf("Connection %d accepted", fd)
		return nil
	}
	return fmt.Errorf("connection %d not found", fd)
}

func (g *Gate) kick(fd int) error {
	if connValue, ok := g.connections.LoadAndDelete(fd); ok {
		conn := connValue.(*Connection)
		conn.Conn.Close()
		log.Printf("Connection %d kicked", fd)
		return nil
	}
	return fmt.Errorf("connection %d not found", fd)
}

func (g *Gate) close() error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if !g.running {
		return nil
	}

	g.running = false

	if g.listener != nil {
		g.listener.Close()
	}

	// Close all connections
	g.connections.Range(func(key, value interface{}) bool {
		conn := value.(*Connection)
		conn.Conn.Close()
		return true
	})

	log.Printf("Gate closed")
	return nil
}

// GateService wraps Gate as a bootstrap service
type GateService struct {
	gate   *Gate
	handle *core.Handle
	system core.ActorSystem
}

// NewGateService creates a new Gate service
func NewGateService() *GateService {
	return &GateService{
		gate: NewGate(),
	}
}

func (s *GateService) Name() string {
	return "gate"
}

func (s *GateService) Start(ctx context.Context) error {
	// Create actor system for this service
	s.system = core.NewActorSystem()
	s.gate.system = s.system

	// Create service actor
	handle, err := s.system.NewService("GATE", s.gate, core.DefaultActorOptions())
	if err != nil {
		return fmt.Errorf("failed to create Gate service: %w", err)
	}

	s.handle = handle

	log.Printf("Gate service started with handle: %v", handle)
	return nil
}

func (s *GateService) Stop(ctx context.Context) error {
	if s.gate != nil {
		log.Printf("Gate service stopping")
		return s.gate.close()
	}
	return nil
}

func (s *GateService) Health(ctx context.Context) (bootstrap.HealthStatus, error) {
	if s.handle == nil {
		return bootstrap.HealthStatus{
			State:   bootstrap.HealthStopped,
			Message: "Gate not running",
		}, nil
	}

	return bootstrap.HealthStatus{
		State:   bootstrap.HealthHealthy,
		Message: "Gate operational",
	}, nil
}

// GetHandle returns the actor handle
func (s *GateService) GetHandle() *core.Handle {
	return s.handle
}
