# SNGO 架构设计文档

## 总体架构

SNGO 采用分层架构设计，从下到上分为：

1. **运行时层**：基于 Go Runtime 的 goroutine 调度
2. **Actor 层**：核心 Actor 模型实现 ✅
3. **网络层**：TCP/UDP 通信支持 ✅
4. **配置层**：配置管理和热重载 ✅
5. **应用层**：Bootstrap 框架和生命周期管理 ✅
6. **服务层**：内置和用户自定义服务 ✅
7. **业务层**：用户业务逻辑实现

## 核心组件

### 1. Actor 系统 (core/) ✅

- **Actor**：基本执行单元，每个 Actor 对应一个 goroutine
- **Message**：Actor 间通信的消息载体
- **Router**：消息路由和分发（直接投递、广播、一致性哈希）
- **Registry**：Actor 注册和查找
- **Handle Manager**：全局 Handle 管理系统
- **Service Discovery**：服务注册发现，负载均衡

### 2. 网络层 (network/) ✅

- **TCP/UDP Server**：高性能网络服务器
- **Connection Manager**：连接池和生命周期管理
- **Message Codec**：二进制消息编解码
- **Protocol**：网络协议抽象和实现
- **Statistics**：网络性能统计

### 3. 配置系统 (config/) ✅

- **Loader**：多格式配置加载（YAML/JSON）
- **Watcher**：配置文件热重载
- **Validation**：配置验证和类型安全
- **Environment Override**：环境变量覆盖
- **Type System**：强类型配置结构

### 4. Bootstrap 框架 (bootstrap/) ✅

- **Application**：应用主框架和生命周期
- **Container**：依赖注入容器（单例/瞬态/作用域）
- **LifecycleManager**：服务编排和依赖解析
- **Service Interface**：统一的服务接口标准
- **Event System**：生命周期事件广播
- **Health Monitoring**：服务健康状态监控

### 5. 集群层 (cluster/) 🔄

- **Node**：集群节点管理
- **Discovery**：跨节点服务发现
- **RPC**：跨节点远程调用

### 6. 工具层 (tools/) 🔄

- **CLI**：命令行工具
- **Monitoring**：监控和调试工具
- **Profiling**：性能分析工具

## 设计原则

### 1. 单一职责
每个组件只负责一个明确的功能领域。

### 2. 依赖倒置
高层模块不依赖低层模块，都依赖抽象接口。

### 3. 开放封闭
对扩展开放，对修改封闭。

### 4. 最小依赖
减少外部依赖，优先使用 Go 标准库。

### 5. 类型安全
利用 Go 的强类型系统，编译期错误检查。

### 6. 并发安全
所有组件都是 goroutine 安全的。

## 应用框架架构 (Bootstrap)

```
Application (应用入口)
├── LifecycleManager (生命周期管理)
│   ├── Service Registration (服务注册)
│   ├── Dependency Resolution (依赖解析)
│   ├── Startup Orchestration (启动编排)
│   └── Health Monitoring (健康监控)
├── Container (依赖注入容器)
│   ├── Service Factory (服务工厂)
│   ├── Instance Cache (实例缓存)
│   ├── Scope Management (作用域管理)
│   └── Type Resolution (类型解析)
├── Event System (事件系统)
│   ├── Lifecycle Events (生命周期事件)
│   ├── Event Broadcasting (事件广播)
│   └── Event Listeners (事件监听器)
└── Core Services (核心服务)
    ├── Actor System Service (Actor 系统服务)
    ├── Network Server Service (网络服务器服务)
    └── Config Watcher Service (配置监听服务)
```

## 线程模型

```
Main Goroutine
├── Application Framework
│   ├── Lifecycle Manager
│   ├── Event System
│   └── Health Monitor
├── Actor System
│   ├── Actor Goroutines (N)
│   ├── Message Router
│   └── Handle Manager
├── Network Layer
│   ├── TCP Server Goroutine
│   ├── Connection Handler Goroutines (M)
│   └── Message Codec
└── Config System
    ├── File Watcher Goroutine
    └── Config Reloader
```

- **Main Goroutine**：应用主控制流
- **Actor Goroutines**：每个 Actor 一个 goroutine
- **Network Goroutines**：网络 I/O 处理
- **System Goroutines**：配置监听、健康检查等

## 消息流

```
Client -> Network Server -> Message Codec -> Actor System -> Router -> Target Actor
                                          ↓
Target Actor -> Response Message -> Router -> Network Server -> Client
```

### 详细流程：
1. 客户端建立连接并发送消息
2. Network Server 接收原始数据
3. Message Codec 解码为结构化消息
4. Actor System 接收消息并路由
5. Router 根据路由策略选择目标 Actor
6. 目标 Actor 处理消息并可能产生响应
7. 响应消息经过相同路径返回客户端

## 配置流

```
Config Files (YAML/JSON) -> Loader -> Validation -> Environment Override -> Application
                         ↓
File Watcher -> Hot Reload -> Event Notification -> Service Reconfiguration
```

## 服务生命周期

```
Registration -> Dependency Resolution -> Startup -> Health Check -> Running -> Shutdown
     ↓              ↓                      ↓           ↓            ↓         ↓
Container    Topological Sort      Service.Start()  Periodic    Normal    Service.Stop()
             (Kahn Algorithm)                       Monitoring  Operation  (Reverse Order)
```

## 内存模型

- **Zero-Copy**：网络消息尽量避免拷贝
- **Object Pool**：重复使用对象减少 GC 压力
- **Message Batching**：批量处理减少系统调用
- **Scoped Services**：合理的服务实例生命周期管理

## 容错机制

- **Service Supervision**：服务监督和自动重启
- **Health Monitoring**：实时健康状态检查
- **Circuit Breaker**：熔断保护机制
- **Timeout Protection**：超时保护和资源清理
- **Graceful Shutdown**：优雅关闭和资源释放
- **Error Isolation**：错误隔离，避免级联失败

## 开发阶段状态

- ✅ **Stage 1-3**: Core Actor System (完成)
- ✅ **Stage 4-5**: Network Layer & Service Discovery (完成)
- ✅ **Stage 6**: Configuration System (完成)
- ✅ **Stage 7**: Bootstrap & Lifecycle Management (完成)
- 🔄 **Stage 8**: Integration Testing & Tools (进行中)

## 性能特性

- **高并发**：基于 goroutine 的轻量级并发模型
- **低延迟**：零拷贝网络通信和高效消息路由
- **高吞吐**：批量处理和连接池优化
- **内存友好**：对象池和作用域管理减少 GC 压力
- **可扩展**：模块化设计支持水平扩展

---

*文档版本：v0.7.0*
*更新时间：2025-09-04*
*项目完成度：87.5% (7/8 阶段)*
