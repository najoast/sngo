package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/najoast/sngo/core"
)

// Agent handles individual client connections
// 这是skynet中的Agent模式：每个客户端连接对应一个Agent
type Agent struct {
	gate     *core.Handle
	client   int
	watchdog *core.Handle
}

// NewAgent creates a new agent
func NewAgent() *Agent {
	return &Agent{}
}

// AgentCommand represents commands to agent
type AgentCommand struct {
	Command string        `json:"command"`
	Args    []interface{} `json:"args"`
}

// HandleMessage implements the MessageHandler interface
func (a *Agent) HandleMessage(ctx context.Context, msg *core.Message) error {
	switch msg.Type {
	case core.MessageTypeRequest:
		return a.handleCommand(ctx, msg)
	case core.MessageTypeText:
		return a.handleMessage(ctx, msg)
	default:
		return fmt.Errorf("unsupported message type: %d", msg.Type)
	}
}

func (a *Agent) handleCommand(ctx context.Context, msg *core.Message) error {
	var cmd AgentCommand
	if err := json.Unmarshal(msg.Data, &cmd); err != nil {
		return fmt.Errorf("failed to parse command: %w", err)
	}

	switch cmd.Command {
	case "start":
		// Initialize agent with gate, client fd, and watchdog
		if len(cmd.Args) > 0 {
			if config, ok := cmd.Args[0].(map[string]interface{}); ok {
				return a.start(config)
			}
		}
		return fmt.Errorf("start command requires configuration")

	case "disconnect":
		// Handle client disconnect
		return a.disconnect()

	default:
		return fmt.Errorf("unknown command: %s", cmd.Command)
	}
}

func (a *Agent) handleMessage(ctx context.Context, msg *core.Message) error {
	// Handle direct messages (like disconnect)
	var cmd AgentCommand
	if err := json.Unmarshal(msg.Data, &cmd); err != nil {
		// If not a command, treat as client data
		log.Printf("Agent received data: %s", string(msg.Data))
		return nil
	}

	return a.handleCommand(ctx, msg)
}

func (a *Agent) start(config map[string]interface{}) error {
	log.Printf("Agent starting with config: %v", config)

	// Extract configuration
	if gateValue, ok := config["gate"]; ok {
		if gate, ok := gateValue.(*core.Handle); ok {
			a.gate = gate
		}
	}

	if clientValue, ok := config["client"]; ok {
		if client, ok := clientValue.(float64); ok {
			a.client = int(client)
		} else if client, ok := clientValue.(int); ok {
			a.client = client
		}
	}

	if watchdogValue, ok := config["watchdog"]; ok {
		if watchdog, ok := watchdogValue.(*core.Handle); ok {
			a.watchdog = watchdog
		}
	}

	log.Printf("Agent started for client %d", a.client)
	return nil
}

func (a *Agent) disconnect() error {
	log.Printf("Agent disconnecting client %d", a.client)
	// Cleanup agent resources
	// In a real implementation, this might involve:
	// - Saving client state
	// - Cleaning up resources
	// - Notifying other services
	return nil
}
