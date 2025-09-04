# Stage 7: Bootstrap & Lifecycle Management System

## 概述

第7阶段实现了SNGO框架的Bootstrap系统，提供了完整的应用框架，包括依赖注入、服务生命周期管理、事件系统和优雅关闭功能。这是SNGO框架的核心应用层，将所有之前的模块整合成一个统一的、生产就绪的应用框架。

## 核心组件

### 1. 接口定义 (`interfaces.go`)

#### Service接口
```go
type Service interface {
    Start(ctx context.Context) error    // 启动服务
    Stop(ctx context.Context) error     // 停止服务
    Health(ctx context.Context) (HealthStatus, error) // 健康检查
    Name() string                       // 服务名称
}
```

#### Container接口（依赖注入）
```go
type Container interface {
    Register(name string, factory ServiceFactory) error
    RegisterInstance(name string, instance interface{}) error
    Resolve(name string) (interface{}, error)
    ResolveAs(name string, target interface{}) error
    Has(name string) bool
    Names() []string
}
```

#### LifecycleManager接口
```go
type LifecycleManager interface {
    Register(name string, service Service, deps ...string) error
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
    Health(ctx context.Context) (map[string]HealthStatus, error)
    Services() []string
    Events() <-chan LifecycleEvent
    AddListener(listener func(LifecycleEvent))
}
```

#### Application接口
```go
type Application interface {
    Configure(config interface{}) error
    Run(ctx context.Context) error
    Shutdown(ctx context.Context) error
    Container() Container
    LifecycleManager() LifecycleManager
}
```

### 2. 依赖注入容器 (`container.go`)

#### DefaultContainer
```go
type DefaultContainer struct {
    services  map[string]ServiceFactory    // 服务工厂
    instances map[string]interface{}       // 服务实例
    mutex     sync.RWMutex                // 并发保护
}
```

#### 服务作用域支持
```go
type ServiceScope int

const (
    ScopeSingleton ServiceScope = iota  // 单例模式
    ScopeTransient                      // 临时实例
    ScopeScoped                         // 作用域实例
)
```

#### ScopedContainer
```go
type ScopedContainer struct {
    *DefaultContainer
    scopes map[string]ServiceScope
}
```

#### Builder模式
```go
builder := NewContainerBuilder().
    WithService("cache", cacheFactory).
    WithInstance("config", configInstance).
    Build()
```

### 3. 生命周期管理器 (`lifecycle.go`)

#### 依赖解析算法
```go
// 使用拓扑排序解决依赖关系
func (lm *DefaultLifecycleManager) calculateStartOrder() ([]string, error) {
    // Kahn算法实现
    // 检测循环依赖
    // 确保正确的启动顺序
}
```

#### 事件系统
```go
type LifecycleEvent struct {
    Type      string                 `json:"type"`
    Service   string                 `json:"service,omitempty"`
    Timestamp time.Time              `json:"timestamp"`
    Error     error                  `json:"error,omitempty"`
    Data      map[string]interface{} `json:"data,omitempty"`
}
```

#### 健康监控
```go
type HealthStatus struct {
    State     HealthState            `json:"state"`
    Message   string                 `json:"message,omitempty"`
    LastCheck time.Time              `json:"last_check,omitempty"`
    Data      map[string]interface{} `json:"data,omitempty"`
}
```

### 4. 应用框架 (`application.go`)

#### DefaultApplication
```go
type DefaultApplication struct {
    config           interface{}
    container        Container
    lifecycleManager LifecycleManager
    configLoader     *config.Loader
    actorSystem      core.ActorSystem
    networkServer    network.Server
    running          bool
    shutdownChan     chan os.Signal
}
```

#### 核心服务集成
```go
// Actor系统服务
type ActorSystemService struct {
    app *DefaultApplication
}

// 网络服务器服务
type NetworkServerService struct {
    app *DefaultApplication
}
```

#### ApplicationBuilder
```go
app, err := bootstrap.NewApplicationBuilder().
    WithActorSystemConfig().
    WithNetworkConfig("localhost:8080").
    WithService("database", dbService, "actor-system").
    WithServiceFactory("cache", cacheFactory).
    Build()
```

## 主要功能特性

### 1. 依赖注入系统

#### 服务工厂注册
```go
container.Register("database", func(c Container) (interface{}, error) {
    // 从容器中解析依赖
    config, _ := c.Resolve("config")
    
    // 创建数据库服务
    return NewDatabaseService(config), nil
})
```

#### 类型安全解析
```go
var dbService DatabaseService
err := container.ResolveAs("database", &dbService)
if err != nil {
    log.Fatal(err)
}
```

#### 作用域管理
```go
// 单例服务（默认）
container.RegisterScoped("config", configFactory, ScopeSingleton)

// 临时服务（每次创建新实例）
container.RegisterScoped("request", requestFactory, ScopeTransient)
```

### 2. 服务生命周期管理

#### 自动依赖排序
```go
// 注册服务及其依赖
lifecycleManager.Register("database", dbService)
lifecycleManager.Register("cache", cacheService, "database")
lifecycleManager.Register("api", apiService, "database", "cache")

// 自动按依赖顺序启动：database -> cache -> api
lifecycleManager.Start(ctx)
```

#### 优雅关闭
```go
// 按相反顺序停止：api -> cache -> database
lifecycleManager.Stop(ctx)
```

#### 健康监控
```go
// 获取所有服务的健康状态
healthMap, err := lifecycleManager.Health(ctx)
for serviceName, status := range healthMap {
    fmt.Printf("%s: %s - %s\n", serviceName, status.State, status.Message)
}
```

### 3. 事件驱动架构

#### 事件监听
```go
lifecycleManager.AddListener(func(event LifecycleEvent) {
    switch event.Type {
    case "service.started":
        log.Printf("服务启动: %s", event.Service)
    case "service.stopped":
        log.Printf("服务停止: %s", event.Service)
    case "service.start_failed":
        log.Printf("服务启动失败: %s, 错误: %v", event.Service, event.Error)
    }
})
```

#### 实时事件流
```go
// 监听事件通道
events := lifecycleManager.Events()
go func() {
    for event := range events {
        // 处理事件
        handleLifecycleEvent(event)
    }
}()
```

### 4. 应用框架功能

#### 信号处理
```go
// 自动处理 SIGTERM 和 SIGINT
app.Run(ctx) // 阻塞直到收到关闭信号
```

#### 配置管理
```go
// 配置应用
config := map[string]interface{}{
    "network": map[string]interface{}{
        "address": "localhost:8080",
    },
}
app.Configure(config)
```

#### 超时控制
```go
// 启动超时
startCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
defer cancel()
err := app.LifecycleManager().Start(startCtx)

// 关闭超时
shutdownCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
defer cancel()
err := app.Shutdown(shutdownCtx)
```

## 使用示例

### 1. 自定义服务实现

```go
type DatabaseService struct {
    config *DatabaseConfig
    db     *sql.DB
}

func (s *DatabaseService) Name() string {
    return "database"
}

func (s *DatabaseService) Start(ctx context.Context) error {
    db, err := sql.Open("postgres", s.config.DSN)
    if err != nil {
        return err
    }
    s.db = db
    return s.db.PingContext(ctx)
}

func (s *DatabaseService) Stop(ctx context.Context) error {
    if s.db != nil {
        return s.db.Close()
    }
    return nil
}

func (s *DatabaseService) Health(ctx context.Context) (HealthStatus, error) {
    if s.db == nil {
        return HealthStatus{
            State:   HealthUnhealthy,
            Message: "Database not connected",
        }, nil
    }
    
    if err := s.db.PingContext(ctx); err != nil {
        return HealthStatus{
            State:   HealthUnhealthy,
            Message: fmt.Sprintf("Database ping failed: %v", err),
        }, nil
    }
    
    return HealthStatus{
        State:   HealthHealthy,
        Message: "Database is healthy",
        Data: map[string]interface{}{
            "connection_count": s.getConnectionCount(),
        },
    }, nil
}
```

### 2. 应用构建和运行

```go
func main() {
    // 创建应用
    app, err := bootstrap.NewApplicationBuilder().
        WithActorSystemConfig().
        WithNetworkConfig("localhost:8080").
        WithService("database", &DatabaseService{
            config: &DatabaseConfig{DSN: "postgres://..."},
        }).
        WithService("api", &APIService{}, "database", "actor-system").
        WithServiceFactory("cache", func(c bootstrap.Container) (interface{}, error) {
            var db *DatabaseService
            if err := c.ResolveAs("database", &db); err != nil {
                return nil, err
            }
            return NewCacheService(db), nil
        }).
        Build()
    
    if err != nil {
        log.Fatal(err)
    }
    
    // 设置事件监听
    app.LifecycleManager().AddListener(func(event bootstrap.LifecycleEvent) {
        log.Printf("事件: %s", event.Type)
        if event.Service != "" {
            log.Printf("  服务: %s", event.Service)
        }
        if event.Error != nil {
            log.Printf("  错误: %v", event.Error)
        }
    })
    
    // 运行应用
    if err := app.Run(context.Background()); err != nil {
        log.Fatal(err)
    }
}
```

### 3. 完整示例输出

```
=== SNGO Bootstrap System Demo ===
Application configured with 4 services

=== Dependency Injection Demo ===
Creating cache service via factory...
Resolved cache service: *main.ExampleService

=== Service Lifecycle Demo ===
Starting all services...
Event: lifecycle.starting
Event: service.starting (service: database)
Starting database service...
database service started successfully
Event: service.started (service: database)
Event: service.starting (service: actor-system)
Event: service.started (service: actor-system)
Event: service.starting (service: network-server)
TCP server started on localhost:8080
Event: service.started (service: network-server)
Event: service.starting (service: api)
Starting api service...
api service started successfully
Event: service.started (service: api)
Event: lifecycle.started

=== Health Check Demo ===
Service Health Status:
  actor-system: healthy - Actor system running
  network-server: healthy - Network server running
  database: healthy - database service is healthy
  api: healthy - api service is healthy

=== Running Demo ===
Application running... (simulating 2 seconds of work)

=== Graceful Shutdown Demo ===
Shutting down application...
Event: lifecycle.stopping
Event: service.stopping (service: api)
Stopping api service...
api service stopped successfully
Event: service.stopped (service: api)
Event: service.stopping (service: network-server)
TCP server stopped
Event: service.stopped (service: network-server)
Event: service.stopping (service: actor-system)
Event: service.stopped (service: actor-system)
Event: service.stopping (service: database)
Stopping database service...
database service stopped successfully
Event: service.stopped (service: database)
Event: lifecycle.stopped
Application shut down successfully

=== Demo Complete ===
```

## 技术架构

### 1. 分层架构
```
应用层 (Application)
├── 生命周期管理层 (LifecycleManager)
├── 依赖注入层 (Container)
└── 服务层 (Services)
    ├── 核心服务 (Actor System)
    ├── 网络服务 (Network Server)
    └── 自定义服务 (User Services)
```

### 2. 控制流程
```
启动阶段：
1. 配置加载和验证
2. 依赖注入容器初始化
3. 服务注册和依赖解析
4. 按依赖顺序启动服务
5. 健康检查和事件广播

运行阶段：
1. 信号监听和处理
2. 周期性健康检查
3. 事件处理和日志记录
4. 错误恢复和重试

关闭阶段：
1. 接收关闭信号
2. 按相反顺序停止服务
3. 资源清理和最终确认
4. 优雅退出
```

### 3. 并发模型
- **协程安全**: 所有组件都是协程安全的
- **非阻塞事件**: 事件广播不会阻塞主流程
- **超时保护**: 所有操作都有超时保护
- **错误隔离**: 单个服务错误不影响其他服务

## 测试覆盖

### 测试用例
```go
func TestContainer(t *testing.T)         // 容器基本功能
func TestLifecycleManager(t *testing.T)  // 生命周期管理
func TestApplication(t *testing.T)       // 应用框架
func TestApplicationBuilder(t *testing.T) // 构建器模式
func TestScopedContainer(t *testing.T)   // 作用域容器
```

### 测试结果
```
=== RUN   TestContainer
--- PASS: TestContainer (0.00s)
=== RUN   TestLifecycleManager
--- PASS: TestLifecycleManager (0.00s)
=== RUN   TestApplication
--- PASS: TestApplication (0.00s)
=== RUN   TestApplicationBuilder
--- PASS: TestApplicationBuilder (0.00s)
=== RUN   TestScopedContainer
--- PASS: TestScopedContainer (0.00s)
PASS
ok      github.com/najoast/sngo/bootstrap       0.199s
```

## 性能特性

### 1. 启动性能
- **并发启动**: 无依赖的服务可以并发启动
- **延迟初始化**: 按需创建服务实例
- **缓存优化**: 单例服务实例缓存

### 2. 运行时性能
- **事件缓冲**: 事件通道缓冲减少阻塞
- **健康检查优化**: 可配置的检查间隔
- **内存管理**: 最小化内存分配

### 3. 关闭性能
- **并发关闭**: 无依赖的服务可以并发关闭
- **超时保护**: 防止单个服务阻塞整体关闭
- **资源清理**: 确保所有资源正确释放

## 生产就绪特性

### 1. 错误处理
- **详细错误信息**: 包含上下文的错误消息
- **错误传播**: 错误正确传播到调用方
- **恢复机制**: 服务启动失败时的恢复策略

### 2. 可观测性
- **生命周期事件**: 完整的事件追踪
- **健康监控**: 实时健康状态监控
- **性能指标**: 启动时间、内存使用等指标

### 3. 配置管理
- **类型安全配置**: 强类型配置结构
- **环境特定配置**: 支持不同环境的配置
- **配置验证**: 启动前验证配置有效性

## 与其他模块的集成

### Core模块集成
- **Actor系统管理**: Actor系统作为托管服务
- **Handle管理**: 集成Handle生命周期
- **消息路由**: 与消息系统集成

### Network模块集成
- **网络服务器管理**: TCP/UDP服务器生命周期
- **连接管理**: 网络连接的健康监控
- **配置解析**: 网络配置的自动解析

### Config模块集成
- **配置加载**: 使用配置系统加载应用配置
- **热重载支持**: 配置变更时的服务重启
- **环境变量**: 环境变量覆盖支持

## 设计模式应用

### 1. 依赖注入模式
- **Constructor Injection**: 通过工厂函数注入依赖
- **Service Locator**: 容器作为服务定位器
- **Inversion of Control**: 控制反转实现解耦

### 2. 生命周期模式
- **Template Method**: 统一的服务生命周期模板
- **Observer**: 事件监听器模式
- **Chain of Responsibility**: 依赖链处理

### 3. 建造者模式
- **Fluent Interface**: 流畅的配置API
- **Step Builder**: 分步骤的应用构建
- **Method Chaining**: 方法链式调用

## 扩展性设计

### 1. 接口扩展
- **插件系统**: 可扩展的服务接口
- **中间件支持**: 服务生命周期中间件
- **事件处理器**: 自定义事件处理器

### 2. 功能扩展
- **服务发现集成**: 与服务注册中心集成
- **配置中心集成**: 与配置中心集成
- **监控系统集成**: 与监控系统集成

### 3. 部署扩展
- **容器化支持**: Docker容器部署
- **Kubernetes集成**: K8s原生支持
- **云平台集成**: 云服务集成

## 总结

第7阶段的Bootstrap系统完成了SNGO框架的核心应用层建设，提供了：

1. **完整的依赖注入系统** - 类型安全、作用域管理、Builder模式
2. **智能的生命周期管理** - 依赖解析、事件驱动、健康监控
3. **现代的应用框架** - 信号处理、配置管理、优雅关闭
4. **生产级的特性** - 错误处理、可观测性、性能优化

Bootstrap系统将所有之前的模块（Core、Network、Config）整合成一个统一的、易于使用的应用框架，为开发者提供了现代化的Go应用开发体验。

现在SNGO框架已经具备了构建生产级Actor系统应用的所有核心功能。
