# SNGO - Skynet in Go

SNGO 是一个基于 Go 语言的高性能 Actor 框架，灵感来源于云风的 [Skynet](https://github.com/cloudwu/skynet)。

## 设计目标

- **类型安全**：利用 Go 的强类型系统，避免 Lua 弱类型带来的维护问题
- **高性能**：基于 goroutine 和 channel 实现高并发 Actor 模型
- **简单部署**：单一可执行文件，无外部依赖
- **API 兼容**：提供类似 Skynet 的 API，降低迁移成本
- **分布式**：原生支持集群和节点间通信

## 核心特性

- [x] **Actor 模型实现** - 完整的 Actor 系统，支持消息传递和状态管理
- [x] **消息路由和传递** - 高效的消息路由，支持直接投递、广播和一致性哈希
- [x] **Handle 管理系统** - 全局 Handle 管理，支持服务注册和查找
- [x] **服务注册和发现** - 完整的服务发现系统，支持负载均衡和健康检查
- [x] **网络层** - TCP/UDP 服务器和客户端，支持连接管理和消息编解码
- [x] **配置系统** - 支持 YAML/JSON，环境变量覆盖，热重载和验证
- [x] **Bootstrap & 生命周期** - 应用框架，依赖注入，服务编排和优雅关闭
- [ ] 集群支持
- [ ] 监控和调试工具

## 快速开始

### 使用 Bootstrap 框架

```go
package main

import (
    "context"
    "log"
    
    "github.com/najoast/sngo/bootstrap"
)

func main() {
    // 使用 Builder 模式创建应用
    app, err := bootstrap.NewApplicationBuilder().
        WithActorSystemConfig().
        WithNetworkConfig("localhost:8080").
        WithService("my-service", &MyService{}).
        Build()
        
    if err != nil {
        log.Fatal(err)
    }
    
    // 运行应用直到收到关闭信号
    if err := app.Run(context.Background()); err != nil {
        log.Fatal(err)
    }
}
```

### 传统方式

```bash
# 构建
go build ./cmd/sngo

# 运行示例
./sngo examples/config.toml
```

## 项目结构

```
sngo/
├── core/          # 核心 Actor 系统 ✅
├── network/       # 网络层实现 ✅
├── config/        # 配置系统 ✅
├── bootstrap/     # 应用框架和生命周期管理 ✅
├── cluster/       # 集群支持 (TODO)
├── service/       # 内置服务 (TODO)
├── gateway/       # 网关服务 (TODO)
├── tools/         # 工具链 (TODO)
├── examples/      # 示例代码 ✅
├── tests/         # 测试代码 ✅
└── docs/          # 文档 (TODO)
```

## Bootstrap 系统

SNGO 提供完整的应用框架，包括：

- **依赖注入容器**：支持服务工厂、实例注册和作用域管理
- **生命周期管理**：自动服务启动排序、依赖解析和优雅关闭
- **应用框架**：信号处理、配置管理和核心服务集成
- **事件系统**：生命周期事件广播和监听

示例：
```go
// 创建自定义服务
type MyService struct {
    name string
}

func (s *MyService) Name() string { return s.name }
func (s *MyService) Start(ctx context.Context) error { return nil }
func (s *MyService) Stop(ctx context.Context) error { return nil }
func (s *MyService) Health(ctx context.Context) (bootstrap.HealthStatus, error) {
    return bootstrap.HealthStatus{State: bootstrap.HealthHealthy}, nil
}

// 使用 ApplicationBuilder
app, err := bootstrap.NewApplicationBuilder().
    WithService("my-service", &MyService{name: "test"}, "actor-system").
    WithServiceFactory("cache", func(c bootstrap.Container) (interface{}, error) {
        return NewCacheService(), nil
    }).
    Build()
```

## 与 Skynet 对比

| 特性 | Skynet | SNGO |
|------|--------|------|
| 语言 | C + Lua | Go |
| 类型系统 | 弱类型 | 强类型 |
| 部署 | 多文件 | 单文件 |
| 开发效率 | 高 | 更高 |
| 维护性 | 一般 | 优秀 |

## 贡献

欢迎提交 Issue 和 Pull Request！

## 许可证

MIT License
