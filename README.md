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
- [ ] 集群支持
- [ ] 监控和调试工具

## 快速开始

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
├── cluster/       # 集群支持 (TODO)
├── service/       # 内置服务 (TODO)
├── gateway/       # 网关服务 (TODO)
├── tools/         # 工具链 (TODO)
├── examples/      # 示例代码 ✅
├── tests/         # 测试代码 (TODO)
└── docs/          # 文档 (TODO)
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
