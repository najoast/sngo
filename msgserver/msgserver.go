package msgserver

import (
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/najoast/sngo/crypt"
)

// MsgServerConfig 消息服务器配置
type MsgServerConfig struct {
	Host    string `json:"host"`    // 监听地址
	Port    int    `json:"port"`    // 监听端口
	Name    string `json:"name"`    // 服务名称
	MaxConn int    `json:"maxconn"` // 最大连接数
	Timeout int    `json:"timeout"` // 超时时间(秒)
}

// Handler 消息服务器处理器接口
type Handler interface {
	// Connect 连接建立时调用
	Connect(fd int, addr string)

	// Disconnect 连接断开时调用
	Disconnect(fd int)

	// Error 连接出错时调用
	Error(fd int, msg string)

	// Message 收到消息时调用，返回响应数据
	Message(fd int, session uint32, msg []byte) []byte

	// Auth 验证用户身份，返回(uid, subid, error)
	Auth(username string, signature []byte) (string, string, error)
}

// Session 会话信息
type Session struct {
	ID       uint32    `json:"id"`
	UserID   string    `json:"userid"`
	SubID    string    `json:"subid"`
	Username string    `json:"username"`
	Secret   []byte    `json:"secret"`
	Seq      uint32    `json:"seq"` // 序列号
	ConnTime time.Time `json:"conn_time"`
	LastSeen time.Time `json:"last_seen"`
}

// Connection 连接信息
type Connection struct {
	fd      int
	conn    net.Conn
	session *Session
	seq     uint32 // 序列号，用于断线重连
	buffer  []byte // 接收缓冲区
}

// MsgServer 消息服务器
type MsgServer struct {
	config      MsgServerConfig
	handler     Handler
	listener    net.Listener
	connections map[int]*Connection // fd -> connection
	sessions    map[string]*Session // username -> session
	mu          sync.RWMutex
	nextFD      int32
	running     bool
}

// NewMsgServer 创建消息服务器
func NewMsgServer(config MsgServerConfig, handler Handler) *MsgServer {
	return &MsgServer{
		config:      config,
		handler:     handler,
		connections: make(map[int]*Connection),
		sessions:    make(map[string]*Session),
		nextFD:      1,
	}
}

// Start 启动消息服务器
func (ms *MsgServer) Start() error {
	addr := fmt.Sprintf("%s:%d", ms.config.Host, ms.config.Port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %v", addr, err)
	}

	ms.listener = listener
	ms.running = true
	log.Printf("Msg server started on %s", addr)

	go ms.acceptLoop()
	return nil
}

// Stop 停止消息服务器
func (ms *MsgServer) Stop() error {
	ms.running = false
	if ms.listener != nil {
		return ms.listener.Close()
	}
	return nil
}

// acceptLoop 接受连接循环
func (ms *MsgServer) acceptLoop() {
	for ms.running {
		conn, err := ms.listener.Accept()
		if err != nil {
			if ms.running {
				log.Printf("Accept error: %v", err)
			}
			return
		}

		go ms.handleConnection(conn)
	}
}

// handleConnection 处理客户端连接
func (ms *MsgServer) handleConnection(conn net.Conn) {
	defer conn.Close()

	// 分配fd
	fd := int(ms.nextFD)
	ms.nextFD++

	connection := &Connection{
		fd:     fd,
		conn:   conn,
		buffer: make([]byte, 0, 4096),
	}

	ms.mu.Lock()
	ms.connections[fd] = connection
	ms.mu.Unlock()

	defer func() {
		ms.mu.Lock()
		delete(ms.connections, fd)
		ms.mu.Unlock()

		if connection.session != nil {
			ms.handler.Disconnect(fd)
		}
	}()

	// 通知连接建立
	ms.handler.Connect(fd, conn.RemoteAddr().String())

	// 设置超时
	if ms.config.Timeout > 0 {
		conn.SetDeadline(time.Now().Add(time.Duration(ms.config.Timeout) * time.Second))
	}

	// 处理握手
	if !ms.handleHandshake(connection) {
		return
	}

	// 处理消息循环
	ms.messageLoop(connection)
}

// handleHandshake 处理握手过程
func (ms *MsgServer) handleHandshake(conn *Connection) bool {
	// 读取握手消息: username:seq:signature
	line, err := ms.readLine(conn.conn)
	if err != nil {
		log.Printf("Failed to read handshake: %v", err)
		return false
	}

	parts := strings.Split(line, ":")
	if len(parts) != 3 {
		log.Printf("Invalid handshake format")
		return false
	}

	username := parts[0]
	seqStr := parts[1]
	signatureStr := parts[2]

	seq, err := strconv.ParseUint(seqStr, 10, 32)
	if err != nil {
		log.Printf("Invalid sequence number: %v", err)
		return false
	}

	signature, err := crypt.Base64Decode(signatureStr)
	if err != nil {
		log.Printf("Invalid signature: %v", err)
		return false
	}

	// 验证身份
	uid, subid, err := ms.handler.Auth(username, signature)
	if err != nil {
		log.Printf("Auth failed: %v", err)
		conn.conn.Write([]byte("401 Auth failed\n"))
		return false
	}

	ms.mu.Lock()
	defer ms.mu.Unlock()

	// 检查是否已有会话
	session, exists := ms.sessions[username]
	if exists {
		// 检查序列号是否正确（必须递增）
		if uint32(seq) <= session.Seq {
			log.Printf("Invalid sequence number: %d <= %d", seq, session.Seq)
			conn.conn.Write([]byte("402 Invalid sequence\n"))
			return false
		}

		// 更新会话信息
		session.Seq = uint32(seq)
		session.LastSeen = time.Now()
	} else {
		// 创建新会话
		session = &Session{
			ID:       uint32(conn.fd),
			UserID:   uid,
			SubID:    subid,
			Username: username,
			ConnTime: time.Now(),
			LastSeen: time.Now(),
		}
		ms.sessions[username] = session
	}

	conn.session = session
	conn.seq = uint32(seq)

	// 发送握手成功响应
	conn.conn.Write([]byte("200 OK\n"))

	log.Printf("User %s connected with fd %d", username, conn.fd)
	return true
}

// messageLoop 消息处理循环
func (ms *MsgServer) messageLoop(conn *Connection) {
	for {
		// 读取消息头: session:length
		line, err := ms.readLine(conn.conn)
		if err != nil {
			ms.handler.Error(conn.fd, err.Error())
			return
		}

		parts := strings.Split(line, ":")
		if len(parts) != 2 {
			log.Printf("Invalid message header format")
			continue
		}

		sessionID, err := strconv.ParseUint(parts[0], 10, 32)
		if err != nil {
			log.Printf("Invalid session ID: %v", err)
			continue
		}

		length, err := strconv.ParseUint(parts[1], 10, 32)
		if err != nil {
			log.Printf("Invalid message length: %v", err)
			continue
		}

		// 读取消息体
		msgData := make([]byte, length)
		_, err = conn.conn.Read(msgData)
		if err != nil {
			ms.handler.Error(conn.fd, err.Error())
			return
		}

		// 处理消息
		response := ms.handler.Message(conn.fd, uint32(sessionID), msgData)

		// 发送响应
		if response != nil {
			responseHeader := fmt.Sprintf("%d:%d\n", sessionID, len(response))
			conn.conn.Write([]byte(responseHeader))
			conn.conn.Write(response)
		}

		// 更新最后活跃时间
		if conn.session != nil {
			conn.session.LastSeen = time.Now()
		}
	}
}

// readLine 从连接读取一行
func (ms *MsgServer) readLine(conn net.Conn) (string, error) {
	buffer := make([]byte, 1024)
	n, err := conn.Read(buffer)
	if err != nil {
		return "", err
	}

	line := strings.TrimSpace(string(buffer[:n]))
	return line, nil
}

// Send 向指定fd发送消息
func (ms *MsgServer) Send(fd int, data []byte) error {
	ms.mu.RLock()
	conn, exists := ms.connections[fd]
	ms.mu.RUnlock()

	if !exists {
		return fmt.Errorf("connection not found: %d", fd)
	}

	// 生成唯一session ID用于服务器推送
	sessionID := uint32(0) // 0表示服务器推送
	header := fmt.Sprintf("%d:%d\n", sessionID, len(data))

	_, err := conn.conn.Write([]byte(header))
	if err != nil {
		return err
	}

	_, err = conn.conn.Write(data)
	return err
}

// Kick 踢出连接
func (ms *MsgServer) Kick(fd int) error {
	ms.mu.RLock()
	conn, exists := ms.connections[fd]
	ms.mu.RUnlock()

	if !exists {
		return fmt.Errorf("connection not found: %d", fd)
	}

	return conn.conn.Close()
}

// GetSession 获取会话信息
func (ms *MsgServer) GetSession(username string) *Session {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	return ms.sessions[username]
}

// GetConnections 获取所有连接信息
func (ms *MsgServer) GetConnections() map[int]*Connection {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	result := make(map[int]*Connection)
	for k, v := range ms.connections {
		result[k] = v
	}
	return result
}

// CleanupSessions 清理过期会话
func (ms *MsgServer) CleanupSessions(timeout time.Duration) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	now := time.Now()
	for username, session := range ms.sessions {
		if now.Sub(session.LastSeen) > timeout {
			delete(ms.sessions, username)
			log.Printf("Session expired: %s", username)
		}
	}
}
