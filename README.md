# SNGO - Skynet in Go

SNGO 是一个基于 Go 语言的高性能 Actor 框架，灵感来源于云风的 [Skynet](https://github.com/cloudwu/skynet)。

## 设计目标

- **类型安全**：利用 Go 的强类型系统，避免 Lua 弱类型带来的维护问题
- **高性能**：基于 goroutine 和 channel 实现高并发 Actor 模型
- **简单部署**：单一可执行文件，无外部依赖
- **API 兼容**：提供类似 Skynet 的 API，降低迁移成本
- **分布式**：原生支持集群和节点间通信

## 核心特性

- [x] Actor 模型实现
- [x] 消息路由和传递
- [x] Handle 管理系统
- [x] 服务注册和发现（基础）
- [x] 同步/异步调用接口
- [ ] TCP/UDP 网络层
- [ ] 集群支持
- [ ] 配置系统
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
├── core/          # 核心 Actor 系统
├── network/       # 网络层实现  
├── cluster/       # 集群支持
├── service/       # 内置服务
├── gateway/       # 网关服务
├── tools/         # 工具链
├── examples/      # 示例代码
├── tests/         # 测试代码
└── docs/          # 文档
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
