package main

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/najoast/sngo/crypt"
)

func main() {
	// 连接到登录服务器
	loginConn, err := net.Dial("tcp", "127.0.0.1:8001")
	if err != nil {
		log.Fatalf("Failed to connect to login server: %v", err)
	}
	defer loginConn.Close()
	
	fmt.Println("Connected to login server")
	
	// 执行登录流程
	err = performLogin(loginConn)
	if err != nil {
		log.Fatalf("Login failed: %v", err)
	}
	
	fmt.Println("Login successful! Now connecting to game server...")
	
	// 连接到游戏服务器
	gameConn, err := net.Dial("tcp", "127.0.0.1:8888")
	if err != nil {
		log.Fatalf("Failed to connect to game server: %v", err)
	}
	defer gameConn.Close()
	
	// 执行游戏服务器握手
	err = performGameHandshake(gameConn)
	if err != nil {
		log.Fatalf("Game handshake failed: %v", err)
	}
	
	fmt.Println("Game handshake successful! Sending test messages...")
	
	// 发送测试消息
	sendTestMessages(gameConn)
}

func performLogin(conn net.Conn) error {
	reader := bufio.NewReader(conn)
	
	// 1. 接收challenge
	challengeStr, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read challenge: %v", err)
	}
	challengeStr = strings.TrimSpace(challengeStr)
	
	challenge, err := base64.StdEncoding.DecodeString(challengeStr)
	if err != nil {
		return fmt.Errorf("invalid challenge: %v", err)
	}
	
	fmt.Printf("Received challenge: %x\n", challenge)
	
	// 2. 生成客户端密钥对
	clientPrivate := crypt.RandomKey()
	clientPublic := crypt.DHExchange(clientPrivate)
	
	// 发送客户端公钥
	clientKeyStr := base64.StdEncoding.EncodeToString(clientPublic) + "\n"
	_, err = conn.Write([]byte(clientKeyStr))
	if err != nil {
		return fmt.Errorf("failed to send client key: %v", err)
	}
	
	// 3. 接收服务器公钥
	serverKeyStr, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read server key: %v", err)
	}
	serverKeyStr = strings.TrimSpace(serverKeyStr)
	
	serverPublic, err := base64.StdEncoding.DecodeString(serverKeyStr)
	if err != nil {
		return fmt.Errorf("invalid server key: %v", err)
	}
	
	// 4. 计算共享密钥
	secret := crypt.DHSecret(clientPrivate, serverPublic)
	fmt.Printf("Calculated secret: %x\n", secret)
	
	// 5. 计算并发送HMAC
	hmac := crypt.HMAC64(challenge, secret)
	hmacStr := base64.StdEncoding.EncodeToString(hmac) + "\n"
	_, err = conn.Write([]byte(hmacStr))
	if err != nil {
		return fmt.Errorf("failed to send HMAC: %v", err)
	}
	
	// 6. 构造并发送token
	user := "testuser"
	server := "sample"
	password := "password"
	
	userB64 := base64.StdEncoding.EncodeToString([]byte(user))
	serverB64 := base64.StdEncoding.EncodeToString([]byte(server))
	passwordB64 := base64.StdEncoding.EncodeToString([]byte(password))
	
	token := fmt.Sprintf("%s@%s:%s", userB64, serverB64, passwordB64)
	encryptedToken := crypt.DesEncode(secret, []byte(token))
	tokenStr := base64.StdEncoding.EncodeToString(encryptedToken) + "\n"
	
	_, err = conn.Write([]byte(tokenStr))
	if err != nil {
		return fmt.Errorf("failed to send token: %v", err)
	}
	
	// 7. 接收响应
	response, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read response: %v", err)
	}
	response = strings.TrimSpace(response)
	
	if !strings.HasPrefix(response, "200") {
		return fmt.Errorf("login failed: %s", response)
	}
	
	// 解析subid
	parts := strings.Split(response, " ")
	if len(parts) < 2 {
		return fmt.Errorf("invalid response format")
	}
	
	subidBytes, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return fmt.Errorf("invalid subid: %v", err)
	}
	
	fmt.Printf("Login successful! SubID: %s\n", string(subidBytes))
	return nil
}

func performGameHandshake(conn net.Conn) error {
	reader := bufio.NewReader(conn)
	
	// 构造握手消息: username:seq:signature
	username := "testuser:12345:1" // uid:subid:seq
	seq := "1"
	signature := base64.StdEncoding.EncodeToString([]byte("dummy_signature"))
	
	handshake := fmt.Sprintf("%s:%s:%s\n", username, seq, signature)
	_, err := conn.Write([]byte(handshake))
	if err != nil {
		return fmt.Errorf("failed to send handshake: %v", err)
	}
	
	// 接收响应
	response, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read handshake response: %v", err)
	}
	response = strings.TrimSpace(response)
	
	if !strings.HasPrefix(response, "200") {
		return fmt.Errorf("handshake failed: %s", response)
	}
	
	fmt.Println("Game handshake successful!")
	return nil
}

func sendTestMessages(conn net.Conn) {
	reader := bufio.NewReader(conn)
	
	for i := 1; i <= 3; i++ {
		// 发送消息: session:length\ndata
		message := fmt.Sprintf("Hello from client, message %d", i)
		session := uint32(i)
		
		header := fmt.Sprintf("%d:%d\n", session, len(message))
		_, err := conn.Write([]byte(header))
		if err != nil {
			log.Printf("Failed to send header: %v", err)
			continue
		}
		
		_, err = conn.Write([]byte(message))
		if err != nil {
			log.Printf("Failed to send message: %v", err)
			continue
		}
		
		// 读取响应头
		responseHeader, err := reader.ReadString('\n')
		if err != nil {
			log.Printf("Failed to read response header: %v", err)
			continue
		}
		
		// 解析响应长度
		parts := strings.Split(strings.TrimSpace(responseHeader), ":")
		if len(parts) != 2 {
			log.Printf("Invalid response header: %s", responseHeader)
			continue
		}
		
		length, err := strconv.Atoi(parts[1])
		if err != nil {
			log.Printf("Invalid response length: %v", err)
			continue
		}
		
		// 读取响应数据
		responseData := make([]byte, length)
		_, err = conn.Read(responseData)
		if err != nil {
			log.Printf("Failed to read response data: %v", err)
			continue
		}
		
		fmt.Printf("Response %d: %s\n", i, string(responseData))
		
		time.Sleep(1 * time.Second)
	}
}
