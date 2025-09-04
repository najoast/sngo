package main

import (
	"encoding/base64"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/najoast/sngo/loginserver"
	"github.com/najoast/sngo/msgserver"
)

// LoginHandler 登录处理器
type LoginHandler struct {
	loginServer *loginserver.LoginServer
	msgServer   *msgserver.MsgServer
	gameServers map[string]string // server -> address
}

// NewLoginHandler 创建登录处理器
func NewLoginHandler() *LoginHandler {
	return &LoginHandler{
		gameServers: make(map[string]string),
	}
}

// AuthHandler 实现loginserver.Handler接口
func (h *LoginHandler) AuthHandler(token string) (string, string, error) {
	// token格式: base64(user)@base64(server):base64(password)
	parts := strings.Split(token, "@")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid token format")
	}

	userPart := parts[0]
	serverPassPart := parts[1]

	serverPassParts := strings.Split(serverPassPart, ":")
	if len(serverPassParts) != 2 {
		return "", "", fmt.Errorf("invalid token format")
	}

	// 解码用户名、服务器名和密码
	userBytes, err := base64.StdEncoding.DecodeString(userPart)
	if err != nil {
		return "", "", fmt.Errorf("invalid user encoding")
	}

	serverBytes, err := base64.StdEncoding.DecodeString(serverPassParts[0])
	if err != nil {
		return "", "", fmt.Errorf("invalid server encoding")
	}

	passwordBytes, err := base64.StdEncoding.DecodeString(serverPassParts[1])
	if err != nil {
		return "", "", fmt.Errorf("invalid password encoding")
	}

	user := string(userBytes)
	server := string(serverBytes)
	password := string(passwordBytes)

	// 验证密码（简单验证）
	if password != "password" {
		return "", "", fmt.Errorf("invalid password")
	}

	// 检查服务器是否存在
	if _, exists := h.gameServers[server]; !exists {
		return "", "", fmt.Errorf("unknown server: %s", server)
	}

	return server, user, nil
}

// LoginHandler 实现loginserver.Handler接口
func (h *LoginHandler) LoginHandler(server, uid string, secret []byte) (string, error) {
	// 生成subid（简单的时间戳）
	subid := fmt.Sprintf("%d", time.Now().Unix())

	log.Printf("User %s logging into server %s with subid %s", uid, server, subid)

	// 这里可以向游戏服务器发送登录准备请求
	// 在实际的skynet实现中，这里会调用game server的登录接口

	return subid, nil
}

// CommandHandler 实现loginserver.Handler接口
func (h *LoginHandler) CommandHandler(command string, args ...interface{}) (interface{}, error) {
	switch command {
	case "register_gate":
		if len(args) < 2 {
			return nil, fmt.Errorf("register_gate requires server and address")
		}
		server := args[0].(string)
		address := args[1].(string)
		h.gameServers[server] = address
		log.Printf("Registered game server: %s -> %s", server, address)
		return "OK", nil
	case "logout":
		if len(args) < 2 {
			return nil, fmt.Errorf("logout requires uid and subid")
		}
		uid := args[0].(string)
		subid := args[1].(string)
		if h.loginServer != nil {
			h.loginServer.Logout(uid, subid)
		}
		return "OK", nil
	default:
		return nil, fmt.Errorf("unknown command: %s", command)
	}
}

// MsgHandler 消息处理器
type MsgHandler struct {
	sessions map[string][]byte // username -> secret
}

// NewMsgHandler 创建消息处理器
func NewMsgHandler() *MsgHandler {
	return &MsgHandler{
		sessions: make(map[string][]byte),
	}
}

// Connect 实现msgserver.Handler接口
func (h *MsgHandler) Connect(fd int, addr string) {
	log.Printf("Client connected: fd=%d, addr=%s", fd, addr)
}

// Disconnect 实现msgserver.Handler接口
func (h *MsgHandler) Disconnect(fd int) {
	log.Printf("Client disconnected: fd=%d", fd)
}

// Error 实现msgserver.Handler接口
func (h *MsgHandler) Error(fd int, msg string) {
	log.Printf("Client error: fd=%d, msg=%s", fd, msg)
}

// Message 实现msgserver.Handler接口
func (h *MsgHandler) Message(fd int, session uint32, msg []byte) []byte {
	log.Printf("Received message: fd=%d, session=%d, msg=%s", fd, session, string(msg))

	// 简单的echo响应
	response := fmt.Sprintf("Echo: %s", string(msg))
	return []byte(response)
}

// Auth 实现msgserver.Handler接口
func (h *MsgHandler) Auth(username string, signature []byte) (string, string, error) {
	// 解析username: uid:subid:seq
	parts := strings.Split(username, ":")
	if len(parts) != 3 {
		return "", "", fmt.Errorf("invalid username format")
	}

	uid := parts[0]
	subid := parts[1]
	seqStr := parts[2]

	// 验证序列号格式
	_, err := strconv.ParseUint(seqStr, 10, 32)
	if err != nil {
		return "", "", fmt.Errorf("invalid sequence number")
	}

	// 在实际实现中，这里应该验证signature
	// signature应该是用secret对username进行HMAC签名的结果

	// 简单验证，实际应该从LoginServer获取secret并验证
	if len(signature) == 0 {
		return "", "", fmt.Errorf("missing signature")
	}

	return uid, subid, nil
}

func main() {
	// 创建处理器
	loginHandler := NewLoginHandler()
	msgHandler := NewMsgHandler()

	// 创建登录服务器配置
	loginConfig := loginserver.LoginServerConfig{
		Host:       "127.0.0.1",
		Port:       8001,
		Name:       "login_master",
		MultiLogin: false,
	}

	// 创建消息服务器配置
	msgConfig := msgserver.MsgServerConfig{
		Host:    "127.0.0.1",
		Port:    8888,
		Name:    "sample",
		MaxConn: 64,
		Timeout: 300,
	}

	// 创建服务器实例
	loginServer := loginserver.NewLoginServer(loginConfig, loginHandler)
	msgServer := msgserver.NewMsgServer(msgConfig, msgHandler)

	// 设置引用
	loginHandler.loginServer = loginServer
	loginHandler.msgServer = msgServer

	// 注册游戏服务器
	loginHandler.CommandHandler("register_gate", "sample", "127.0.0.1:8888")

	// 启动服务器
	err := loginServer.Start()
	if err != nil {
		log.Fatalf("Failed to start login server: %v", err)
	}

	err = msgServer.Start()
	if err != nil {
		log.Fatalf("Failed to start msg server: %v", err)
	}

	log.Println("Login framework started")
	log.Printf("Login server listening on %s:%d", loginConfig.Host, loginConfig.Port)
	log.Printf("Message server listening on %s:%d", msgConfig.Host, msgConfig.Port)

	// 保持运行
	select {}
}
