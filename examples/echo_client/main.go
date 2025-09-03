// Package main provides a simple echo client example using SNGO network layer
package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/najoast/sngo/network"
)

func main() {
	// Create client
	client, err := network.CreateTCPClient(nil)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	// Set up message handler to receive server responses
	client.SetMessageHandler(&ClientMessageHandler{})

	// Connect to server
	serverAddr := "localhost:8080"
	if len(os.Args) > 1 {
		serverAddr = os.Args[1]
	}

	fmt.Printf("Connecting to echo server at %s...\n", serverAddr)
	conn, err := client.Connect(serverAddr)
	if err != nil {
		log.Fatalf("Failed to connect to server: %v", err)
	}

	fmt.Printf("Connected to server! Connection ID: %s\n", conn.ID())
	fmt.Println("Type messages to send to the server. Commands:")
	fmt.Println("  /ping - Send ping RPC call")
	fmt.Println("  /time - Get server time")
	fmt.Println("  /stats - Get connection statistics")
	fmt.Println("  /quit - Disconnect and exit")
	fmt.Println()

	// Read user input and send messages
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("Enter message: ")
		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		// Handle commands
		if strings.HasPrefix(input, "/") {
			if handleCommand(client, input) {
				break // Exit if quit command
			}
		} else {
			// Send regular message
			msg := network.NewMessage(network.MessageTypeData, []byte(input))
			err = client.SendMessage(msg)
			if err != nil {
				fmt.Printf("Failed to send message: %v\n", err)
			}
		}
	}

	// Disconnect
	fmt.Println("Disconnecting...")
	err = client.Disconnect()
	if err != nil {
		log.Printf("Failed to disconnect: %v", err)
	}
	fmt.Println("Disconnected.")
}

func handleCommand(client network.Client, command string) bool {
	switch command {
	case "/quit":
		return true
	case "/ping":
		rpc := network.NewRPCMessage("client", "server", []byte("ping"))
		err := client.SendMessage(rpc)
		if err != nil {
			fmt.Printf("Failed to send ping: %v\n", err)
		}
	case "/time":
		rpc := network.NewRPCMessage("client", "server", []byte("time"))
		err := client.SendMessage(rpc)
		if err != nil {
			fmt.Printf("Failed to get time: %v\n", err)
		}
	case "/stats":
		rpc := network.NewRPCMessage("client", "server", []byte("stats"))
		err := client.SendMessage(rpc)
		if err != nil {
			fmt.Printf("Failed to get stats: %v\n", err)
		}
	default:
		fmt.Printf("Unknown command: %s\n", command)
	}
	return false
}

// ClientMessageHandler handles messages from the server
type ClientMessageHandler struct{}

func (h *ClientMessageHandler) OnMessage(conn network.Connection, msg *network.Message) {
	switch msg.Type {
	case network.MessageTypeData:
		if strings.HasPrefix(string(msg.Data), "Welcome") {
			fmt.Printf("Server: %s\n", string(msg.Data))
		} else {
			fmt.Printf("Server echo: %s\n", string(msg.Data))
		}
	case network.MessageTypeRPC:
		fmt.Printf("RPC response: %s\n", string(msg.Data))
	case network.MessageTypeAck:
		fmt.Printf("Server acknowledged sequence %d\n", msg.Sequence)
	default:
		fmt.Printf("Received message type %v: %s\n", msg.Type, string(msg.Data))
	}
}

func (h *ClientMessageHandler) OnError(conn network.Connection, err error) {
	fmt.Printf("Message error: %v\n", err)
}
