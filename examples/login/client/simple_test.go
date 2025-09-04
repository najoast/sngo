package main

import (
	"fmt"
	"log"
	"net"
)

func _main() {
	// 简单连接测试
	conn, err := net.Dial("tcp", "127.0.0.1:8001")
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	fmt.Println("Connected to login server")

	// 读取challenge
	buffer := make([]byte, 1024)
	n, err := conn.Read(buffer)
	if err != nil {
		log.Fatalf("Failed to read challenge: %v", err)
	}

	fmt.Printf("Received: %s", string(buffer[:n]))

	// 发送简单响应
	_, err = conn.Write([]byte("test\n"))
	if err != nil {
		log.Fatalf("Failed to send response: %v", err)
	}

	// 读取下一个响应
	n, err = conn.Read(buffer)
	if err != nil {
		fmt.Printf("Read error (expected): %v\n", err)
	} else {
		fmt.Printf("Server response: %s", string(buffer[:n]))
	}
}
