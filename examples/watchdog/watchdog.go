package watchdog

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"

	"github.com/najoast/sngo/bootstrap"
	"github.com/najoast/sngo/core"
	"github.com/najoast/sngo/examples/agent"
)

// Watchdog manages connections and creates agents for each client
// 这是skynet中经典的Watchdog模式：管理连接，为每个客户端创建Agent
type Watchdog struct {
	gate   *core.Handle // Gate服务的句柄
	agents sync.Map     // fd -> agent handle mapping
	system core.ActorSystem
}

// NewWatchdog creates a new watchdog
func NewWatchdog() *Watchdog {
	return &Watchdog{}
}

// SocketEvent represents socket events from gate
type SocketEvent struct {
	Type    string `json:"type"`              // "open", "close", "error", "warning", "data"
	FD      int    `json:"fd"`                // file descriptor
	Address string `json:"address,omitempty"` // client address for open event
	Message string `json:"message,omitempty"` // error message or data
	Size    int    `json:"size,omitempty"`    // warning size
}

// WatchdogCommand represents commands to watchdog
type WatchdogCommand struct {
	Command string        `json:"command"`
	Args    []interface{} `json:"args"`
}

// HandleMessage implements the MessageHandler interface
func (w *Watchdog) HandleMessage(ctx context.Context, msg *core.Message) error {
	switch msg.Type {
	case core.MessageTypeRequest:
		return w.handleCommand(ctx, msg)
	case core.MessageTypeText:
		return w.handleSocketEvent(ctx, msg)
	default:
		return fmt.Errorf("unsupported message type: %d", msg.Type)
	}
}

func (w *Watchdog) handleCommand(ctx context.Context, msg *core.Message) error {
	var cmd WatchdogCommand
	if err := json.Unmarshal(msg.Data, &cmd); err != nil {
		return fmt.Errorf("failed to parse command: %w", err)
	}

	switch cmd.Command {
	case "start":
		// Start gate with configuration
		if len(cmd.Args) > 0 {
			if config, ok := cmd.Args[0].(map[string]interface{}); ok {
				return w.startGate(ctx, config)
			}
		}
		return fmt.Errorf("start command requires configuration")

	case "close":
		// Close specific connection
		if len(cmd.Args) > 0 {
			if fd, ok := cmd.Args[0].(float64); ok {
				return w.closeAgent(int(fd))
			}
		}
		return fmt.Errorf("close command requires fd")

	default:
		return fmt.Errorf("unknown command: %s", cmd.Command)
	}
}

func (w *Watchdog) handleSocketEvent(ctx context.Context, msg *core.Message) error {
	var event SocketEvent
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		return fmt.Errorf("failed to parse socket event: %w", err)
	}

	switch event.Type {
	case "open":
		return w.handleSocketOpen(ctx, event.FD, event.Address)
	case "close":
		return w.handleSocketClose(ctx, event.FD)
	case "error":
		return w.handleSocketError(ctx, event.FD, event.Message)
	case "warning":
		return w.handleSocketWarning(ctx, event.FD, event.Size)
	case "data":
		return w.handleSocketData(ctx, event.FD, event.Message)
	default:
		return fmt.Errorf("unknown socket event type: %s", event.Type)
	}
}

func (w *Watchdog) handleSocketOpen(ctx context.Context, fd int, addr string) error {
	log.Printf("New client from: %s (fd: %d)", addr, fd)

	// Create new agent for this client
	agentActor := agent.NewAgent()
	agentHandle, err := w.system.NewActor(agentActor, core.DefaultActorOptions())
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	// Store agent handle
	w.agents.Store(fd, agentHandle)

	// Send start command to agent
	startCmd := agent.AgentCommand{
		Command: "start",
		Args: []interface{}{
			map[string]interface{}{
				"gate":     w.gate,
				"client":   fd,
				"watchdog": "self", // TODO: get self handle
			},
		},
	}

	startData, err := json.Marshal(startCmd)
	if err != nil {
		return fmt.Errorf("failed to marshal start command: %w", err)
	}

	return w.system.Send(agentHandle.ID(), agentHandle.ID(), core.MessageTypeRequest, startData)
}

func (w *Watchdog) handleSocketClose(ctx context.Context, fd int) error {
	log.Printf("Socket close: %d", fd)
	return w.closeAgent(fd)
}

func (w *Watchdog) handleSocketError(ctx context.Context, fd int, errorMsg string) error {
	log.Printf("Socket error %d: %s", fd, errorMsg)
	return w.closeAgent(fd)
}

func (w *Watchdog) handleSocketWarning(ctx context.Context, fd int, size int) error {
	log.Printf("Socket warning %d: %d K bytes haven't sent out", fd, size)
	return nil
}

func (w *Watchdog) handleSocketData(ctx context.Context, fd int, data string) error {
	// In the original skynet watchdog, data events are not handled
	// They are typically handled by the gate service and forwarded to agents
	log.Printf("Socket data from %d: %s", fd, data)
	return nil
}

func (w *Watchdog) closeAgent(fd int) error {
	if agentHandle, ok := w.agents.LoadAndDelete(fd); ok {
		handle := agentHandle.(core.Actor)

		// Send kick command to gate
		if w.gate != nil {
			kickCmd := map[string]interface{}{
				"command": "kick",
				"fd":      fd,
			}
			kickData, _ := json.Marshal(kickCmd)
			w.system.Send(w.gate.ActorID, w.gate.ActorID, core.MessageTypeRequest, kickData)
		}

		// Send disconnect command to agent (fire and forget)
		disconnectCmd := agent.AgentCommand{
			Command: "disconnect",
		}
		disconnectData, _ := json.Marshal(disconnectCmd)
		w.system.Send(handle.ID(), handle.ID(), core.MessageTypeText, disconnectData)
	}

	return nil
}

func (w *Watchdog) startGate(ctx context.Context, config map[string]interface{}) error {
	// TODO: Create gate service
	// For now, we'll simulate gate creation
	log.Printf("Starting gate with config: %v", config)

	// In a real implementation, we would:
	// 1. Create a Gate service
	// 2. Configure it with the provided config
	// 3. Store the gate handle
	// w.gate = gateHandle

	return nil
}

// WatchdogService wraps Watchdog as a bootstrap service
type WatchdogService struct {
	watchdog *Watchdog
	handle   *core.Handle
	system   core.ActorSystem
}

// NewWatchdogService creates a new Watchdog service
func NewWatchdogService() *WatchdogService {
	return &WatchdogService{
		watchdog: NewWatchdog(),
	}
}

func (s *WatchdogService) Name() string {
	return "watchdog"
}

func (s *WatchdogService) Start(ctx context.Context) error {
	// Create actor system for this service
	s.system = core.NewActorSystem()
	s.watchdog.system = s.system

	// Create service actor
	handle, err := s.system.NewService("WATCHDOG", s.watchdog, core.DefaultActorOptions())
	if err != nil {
		return fmt.Errorf("failed to create Watchdog service: %w", err)
	}

	s.handle = handle

	log.Printf("Watchdog service started with handle: %v", handle)
	return nil
}

func (s *WatchdogService) Stop(ctx context.Context) error {
	if s.handle != nil && s.system != nil {
		log.Printf("Watchdog service stopping")
		// Close all agents
		s.watchdog.agents.Range(func(key, value interface{}) bool {
			fd := key.(int)
			s.watchdog.closeAgent(fd)
			return true
		})
	}
	return nil
}

func (s *WatchdogService) Health(ctx context.Context) (bootstrap.HealthStatus, error) {
	if s.handle == nil {
		return bootstrap.HealthStatus{
			State:   bootstrap.HealthStopped,
			Message: "Watchdog not running",
		}, nil
	}

	return bootstrap.HealthStatus{
		State:   bootstrap.HealthHealthy,
		Message: "Watchdog operational",
	}, nil
}

// GetHandle returns the actor handle
func (s *WatchdogService) GetHandle() *core.Handle {
	return s.handle
}
