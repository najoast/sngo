# SNGO 架构设计文档

## 总体架构

SNGO 采用分层架构设计，从下到上分为：

1. **运行时层**：基于 Go Runtime 的 goroutine 调度
2. **Actor 层**：核心 Actor 模型实现
3. **网络层**：TCP/UDP 通信支持
4. **服务层**：内置和用户自定义服务
5. **应用层**：业务逻辑实现

## 核心组件

### 1. Actor 系统 (core/)

- **Actor**：基本执行单元，每个 Actor 对应一个 goroutine
- **Message**：Actor 间通信的消息载体
- **Router**：消息路由和分发
- **Registry**：Actor 注册和查找

### 2. 网络层 (network/)

- **Socket Server**：TCP/UDP 监听和连接管理
- **Protocol**：消息协议定义和编解码
- **Gateway**：外部连接代理

### 3. 集群层 (cluster/)

- **Node**：集群节点管理
- **Discovery**：服务发现机制
- **RPC**：跨节点远程调用

### 4. 服务层 (service/)

- **Bootstrap**：系统启动服务
- **Logger**：日志服务
- **Monitor**：监控服务

## 设计原则

### 1. 单一职责
每个组件只负责一个明确的功能领域。

### 2. 依赖倒置
高层模块不依赖低层模块，都依赖抽象接口。

### 3. 开放封闭
对扩展开放，对修改封闭。

### 4. 最小依赖
减少外部依赖，优先使用 Go 标准库。

## 线程模型

```
Main Thread
├── Network Thread Pool
├── Worker Thread Pool  
└── Monitor Thread
```

- **Main Thread**：程序入口，负责初始化和协调
- **Network Threads**：处理网络 I/O
- **Worker Threads**：执行 Actor 逻辑
- **Monitor Thread**：监控和统计

## 消息流

```
Client -> Gateway -> Router -> Actor -> Router -> Actor
```

1. 客户端发送消息到网关
2. 网关解析消息并路由到目标 Actor
3. Actor 处理消息并可能发送新消息
4. 循环往复

## 内存模型

- **Zero-Copy**：网络消息尽量避免拷贝
- **Pool**：重复使用对象减少 GC 压力
- **Batch**：批量处理减少系统调用

## 容错机制

- **Supervision**：Actor 监督树
- **Restart**：故障自动重启
- **Circuit Breaker**：熔断保护
- **Timeout**：超时处理

---

*文档版本：v0.1.0*
*更新时间：2025-09-03*
