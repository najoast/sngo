# SNGO 开发规范

## 代码规范

### 1. 包命名
- 使用小写字母
- 简洁有意义
- 避免下划线

### 2. 接口设计
- 接口名以 -er 结尾（如 Handler, Router）
- 接口方法数量保持精简
- 优先使用小接口组合

### 3. 错误处理
```go
// 返回错误
func DoSomething() error {
    if err != nil {
        return fmt.Errorf("do something failed: %w", err)
    }
    return nil
}

// 处理错误
if err := DoSomething(); err != nil {
    log.Printf("error: %v", err)
    return err
}
```

### 4. 并发安全
- 优先使用 channel 通信
- 必要时使用 sync.RWMutex
- 避免共享状态

## 目录结构规范

### 每个包的标准结构
```
package/
├── doc.go          # 包文档
├── types.go        # 类型定义
├── interface.go    # 接口定义  
├── implementation.go # 实现代码
├── example_test.go # 示例测试
└── package_test.go # 单元测试
```

### 文件命名规范
- `types.go`：数据结构定义
- `interface.go`：接口定义
- `*_test.go`：测试文件
- `example_*.go`：示例代码

## 注释规范

### 包注释
```go
// Package core implements the fundamental Actor system for SNGO.
//
// This package provides the basic building blocks including Actor,
// Message, and Router components.
package core
```

### 公开接口注释
```go
// Actor represents a computational unit that processes messages sequentially.
// Each Actor runs in its own goroutine and communicates through channels.
type Actor interface {
    // Start begins the Actor's message processing loop.
    Start() error
    
    // Stop gracefully shuts down the Actor.
    Stop() error
}
```

## 测试规范

### 单元测试
- 每个公开函数都要有测试
- 使用 table-driven tests
- 测试覆盖率不低于 80%

```go
func TestActorSend(t *testing.T) {
    tests := []struct {
        name    string
        message Message
        want    error
    }{
        {"valid message", Message{Type: "test"}, nil},
        {"invalid message", Message{}, ErrInvalidMessage},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := actor.Send(tt.message)
            if got != tt.want {
                t.Errorf("Send() = %v, want %v", got, tt.want)
            }
        })
    }
}
```

### 基准测试
```go
func BenchmarkActorSend(b *testing.B) {
    actor := NewActor()
    msg := Message{Type: "benchmark"}
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        actor.Send(msg)
    }
}
```

## Git 规范

### 提交信息格式
```
<type>(<scope>): <subject>

<body>

<footer>
```

### 类型说明
- `feat`: 新功能
- `fix`: 修复 bug
- `docs`: 文档更新
- `style`: 代码格式调整
- `refactor`: 重构
- `test`: 测试相关
- `chore`: 构建工具等

### 示例
```
feat(core): implement basic Actor system

- Add Actor interface and implementation
- Add Message types and routing
- Add basic unit tests

Closes #1
```

## 性能规范

### 内存分配
- 尽量重用对象
- 使用 sync.Pool 管理临时对象
- 避免不必要的内存分配

### 并发性能
- 优化 channel 使用
- 减少 goroutine 创建开销
- 合理设置 buffer 大小

### 监控指标
- 每秒消息处理量
- 内存使用量
- CPU 使用率
- 网络延迟

---

*文档版本：v0.1.0*
*更新时间：2025-09-03*
