package loginserver

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"strings"
	"time"

	"github.com/najoast/sngo/crypt"
)

// GameServerActor 游戏服务器接口
type GameServerActor interface {
	GetHandle() string
	Send(message string) error
}

// LoginServerConfig 登录服务器配置
type LoginServerConfig struct {
	Host       string `json:"host"`       // 监听地址
	Port       int    `json:"port"`       // 监听端口
	Name       string `json:"name"`       // 服务名称
	MultiLogin bool   `json:"multilogin"` // 是否允许多重登录
}

// Handler 登录服务器处理器接口
type Handler interface {
	// AuthHandler 验证token，返回(server, uid, error)
	AuthHandler(token string) (string, string, error)

	// LoginHandler 处理登录请求，返回subid
	LoginHandler(server, uid string, secret []byte) (string, error)

	// CommandHandler 处理内部命令
	CommandHandler(command string, args ...interface{}) (interface{}, error)
}

// LoginServer 登录服务器
type LoginServer struct {
	config   LoginServerConfig
	handler  Handler
	listener net.Listener
	actors   map[string]GameServerActor // 注册的游戏服务器
	users    map[string]*UserInfo       // 在线用户
}

// UserInfo 用户信息
type UserInfo struct {
	UID     string    `json:"uid"`
	Server  string    `json:"server"`
	Address string    `json:"address"`
	SubID   string    `json:"subid"`
	LoginAt time.Time `json:"login_at"`
}

// NewLoginServer 创建登录服务器
func NewLoginServer(config LoginServerConfig, handler Handler) *LoginServer {
	return &LoginServer{
		config:  config,
		handler: handler,
		actors:  make(map[string]GameServerActor),
		users:   make(map[string]*UserInfo),
	}
}

// Start 启动登录服务器
func (ls *LoginServer) Start() error {
	addr := fmt.Sprintf("%s:%d", ls.config.Host, ls.config.Port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %v", addr, err)
	}

	ls.listener = listener
	log.Printf("Login server started on %s", addr)

	go ls.acceptLoop()
	return nil
}

// Stop 停止登录服务器
func (ls *LoginServer) Stop() error {
	if ls.listener != nil {
		return ls.listener.Close()
	}
	return nil
}

// acceptLoop 接受连接循环
func (ls *LoginServer) acceptLoop() {
	for {
		conn, err := ls.listener.Accept()
		if err != nil {
			log.Printf("Accept error: %v", err)
			return
		}

		go ls.handleConnection(conn)
	}
}

// handleConnection 处理客户端连接
func (ls *LoginServer) handleConnection(conn net.Conn) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Panic in handleConnection: %v", r)
		}
		conn.Close()
	}()

	log.Printf("New connection from %s", conn.RemoteAddr().String())

	// 设置超时
	conn.SetDeadline(time.Now().Add(30 * time.Second))

	// DH密钥交换阶段
	challenge := crypt.RandomKey()
	log.Printf("Generated challenge: %x", challenge)

	// 发送challenge
	challengeStr := crypt.Base64Encode(challenge) + "\n"
	_, err := conn.Write([]byte(challengeStr))
	if err != nil {
		log.Printf("Failed to send challenge: %v", err)
		return
	}
	log.Printf("Sent challenge: %s", challengeStr)

	// 接收客户端公钥
	clientKeyStr, err := ls.readLine(conn)
	if err != nil {
		log.Printf("Failed to read client key: %v", err)
		return
	}
	log.Printf("Received client key: %s", clientKeyStr)

	clientKey, err := crypt.Base64Decode(clientKeyStr)
	if err != nil {
		log.Printf("Invalid client key: %v", err)
		return
	}
	log.Printf("Decoded client key: %x", clientKey)

	// 生成服务器密钥对
	serverPrivate := crypt.RandomKey()
	serverPublic := crypt.DHExchange(serverPrivate)
	log.Printf("Generated server keys - private: %x, public: %x", serverPrivate, serverPublic)

	// 发送服务器公钥
	serverKeyStr := crypt.Base64Encode(serverPublic) + "\n"
	_, err = conn.Write([]byte(serverKeyStr))
	if err != nil {
		log.Printf("Failed to send server key: %v", err)
		return
	}
	log.Printf("Sent server key: %s", serverKeyStr)

	// 计算共享密钥
	secret := crypt.DHSecret(serverPrivate, clientKey)
	log.Printf("Calculated shared secret: %x", secret)

	// 接收HMAC验证
	hmacStr, err := ls.readLine(conn)
	if err != nil {
		log.Printf("Failed to read HMAC: %v", err)
		return
	}

	clientHMAC, err := crypt.Base64Decode(hmacStr)
	if err != nil {
		log.Printf("Invalid HMAC: %v", err)
		return
	}

	// 验证HMAC
	expectedHMAC := crypt.HMAC64(challenge, secret)
	if string(clientHMAC) != string(expectedHMAC) {
		log.Printf("HMAC verification failed")
		conn.Write([]byte("401 HMAC verification failed\n"))
		return
	}

	// 接收加密的token
	tokenStr, err := ls.readLine(conn)
	if err != nil {
		log.Printf("Failed to read token: %v", err)
		return
	}

	encryptedToken, err := crypt.Base64Decode(tokenStr)
	if err != nil {
		log.Printf("Invalid token: %v", err)
		return
	}

	// 解密token
	tokenBytes := crypt.DesDecode(secret, encryptedToken)
	token := string(tokenBytes)

	// 验证token
	server, uid, err := ls.handler.AuthHandler(token)
	if err != nil {
		log.Printf("Auth failed: %v", err)
		conn.Write([]byte(fmt.Sprintf("403 %s\n", err.Error())))
		return
	}

	// 检查游戏服务器是否存在
	gameServer, exists := ls.actors[server]
	if !exists {
		log.Printf("Unknown server: %s", server)
		conn.Write([]byte("404 Unknown server\n"))
		return
	}

	// 检查是否允许多重登录
	if !ls.config.MultiLogin {
		if existingUser, exists := ls.users[uid]; exists {
			// 踢出已存在的用户
			ls.kickUser(existingUser)
		}
	}

	// 向游戏服务器发送登录请求
	subid, err := ls.handler.LoginHandler(server, uid, secret)
	if err != nil {
		log.Printf("Login handler failed: %v", err)
		conn.Write([]byte(fmt.Sprintf("500 %s\n", err.Error())))
		return
	}

	// 记录用户信息
	userInfo := &UserInfo{
		UID:     uid,
		Server:  server,
		Address: gameServer.GetHandle(),
		SubID:   subid,
		LoginAt: time.Now(),
	}
	ls.users[uid] = userInfo

	// 返回成功响应和subid
	response := fmt.Sprintf("200 %s\n", crypt.Base64Encode([]byte(subid)))
	conn.Write([]byte(response))

	log.Printf("User %s logged into server %s with subid %s", uid, server, subid)
}

// readLine 从连接读取一行
func (ls *LoginServer) readLine(conn net.Conn) (string, error) {
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		return "", err
	}

	line := strings.TrimSpace(string(buf[:n]))
	return line, nil
}

// kickUser 踢出用户
func (ls *LoginServer) kickUser(userInfo *UserInfo) {
	if gameServer, exists := ls.actors[userInfo.Server]; exists {
		// 向游戏服务器发送踢出消息
		message := map[string]interface{}{
			"cmd":   "kick",
			"uid":   userInfo.UID,
			"subid": userInfo.SubID,
		}

		data, _ := json.Marshal(message)
		gameServer.Send(string(data))
	}

	delete(ls.users, userInfo.UID)
}

// RegisterGameServer 注册游戏服务器
func (ls *LoginServer) RegisterGameServer(server string, actor GameServerActor) {
	ls.actors[server] = actor
	log.Printf("Game server registered: %s -> %s", server, actor.GetHandle())
}

// Logout 用户登出
func (ls *LoginServer) Logout(uid, subid string) {
	if userInfo, exists := ls.users[uid]; exists {
		if userInfo.SubID == subid {
			delete(ls.users, uid)
			log.Printf("User %s logged out", uid)
		}
	}
}

// GetOnlineUsers 获取在线用户列表
func (ls *LoginServer) GetOnlineUsers() map[string]*UserInfo {
	return ls.users
}
