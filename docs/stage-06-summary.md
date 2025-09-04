# Stage 6: Configuration System Implementation

## 概述

第6阶段实现了SNGO框架的配置系统，提供了全面的配置管理功能，包括多格式支持、环境变量覆盖、热重载和验证机制。

## 实现的功能

### 1. 核心配置类型 (`types.go`)

#### 配置结构
```go
type Config struct {
    App       AppConfig       `yaml:"app" json:"app"`
    Log       LogConfig       `yaml:"log" json:"log"`
    Network   NetworkConfig   `yaml:"network" json:"network"`
    Actor     ActorConfig     `yaml:"actor" json:"actor"`
    Discovery DiscoveryConfig `yaml:"discovery" json:"discovery"`
    Monitor   MonitorConfig   `yaml:"monitor" json:"monitor"`
    Custom    map[string]interface{} `yaml:"custom,omitempty" json:"custom,omitempty"`
}
```

#### 应用配置
```go
type AppConfig struct {
    Name        string            `yaml:"name" json:"name"`
    Version     string            `yaml:"version" json:"version"`
    Environment Environment       `yaml:"environment" json:"environment"`
    Debug       bool              `yaml:"debug" json:"debug"`
    Description string            `yaml:"description,omitempty" json:"description,omitempty"`
    Metadata    map[string]string `yaml:"metadata,omitempty" json:"metadata,omitempty"`
}
```

#### 网络配置
```go
type NetworkConfig struct {
    Host           string        `yaml:"host" json:"host"`
    Port           int           `yaml:"port" json:"port"`
    ReadTimeout    time.Duration `yaml:"read_timeout" json:"read_timeout"`
    WriteTimeout   time.Duration `yaml:"write_timeout" json:"write_timeout"`
    MaxConnections int           `yaml:"max_connections" json:"max_connections"`
}
```

### 2. 配置加载器 (`loader.go`)

#### 多格式支持
- **YAML**: 主要配置格式
- **JSON**: 替代配置格式
- **自动检测**: 根据文件扩展名自动选择解析器

#### 配置搜索路径
```go
searchPaths := []string{
    ".",                          // 当前目录
    "./config",                   // config目录
    "./configs",                  // configs目录
    "/etc/sngo",                 // 系统配置目录
    os.Getenv("HOME") + "/.sngo", // 用户配置目录
}
```

#### 环境变量覆盖
```go
// 支持环境变量前缀
envPrefix := "SNGO"

// 示例：SNGO_APP_NAME 覆盖 app.name
// 示例：SNGO_NETWORK_PORT 覆盖 network.port
```

### 3. 配置监听器 (`watcher.go`)

#### 文件监听
```go
type Watcher struct {
    fsWatcher *fsnotify.Watcher
    loader    *Loader
    callbacks []ChangeCallback
    
    // 防抖机制
    debounceInterval time.Duration
    lastChangeTime   time.Time
}
```

#### 热重载功能
- **文件变更检测**: 使用 `fsnotify` 监听配置文件变化
- **防抖机制**: 避免频繁的配置重载
- **回调通知**: 配置变更时通知注册的回调函数
- **错误处理**: 配置重载失败时的错误处理

### 4. 配置验证

#### 结构验证
```go
func (c *Config) Validate() error {
    if c.App.Name == "" {
        return &ValidationError{
            Field:   "app.name",
            Message: "application name is required",
        }
    }
    
    if c.Network.Port < 1 || c.Network.Port > 65535 {
        return &ValidationError{
            Field:   "network.port",
            Message: "port must be between 1 and 65535",
        }
    }
    
    return nil
}
```

#### 环境验证
```go
const (
    EnvDevelopment Environment = "development"
    EnvTesting     Environment = "testing"
    EnvStaging     Environment = "staging"
    EnvProduction  Environment = "production"
)
```

## 使用示例

### 基本使用
```go
// 创建加载器
loader := config.NewLoader()

// 加载配置文件
cfg, err := loader.Load("config.yaml")
if err != nil {
    log.Fatal(err)
}

// 验证配置
if err := cfg.Validate(); err != nil {
    log.Fatal(err)
}
```

### 环境变量覆盖
```bash
# 设置环境变量
export SNGO_APP_NAME="my-app"
export SNGO_NETWORK_PORT="9090"
export SNGO_APP_DEBUG="true"

# 运行应用（配置会被环境变量覆盖）
./sngo
```

### 热重载
```go
// 创建监听器
watcher := config.NewWatcher(loader)

// 注册变更回调
watcher.AddCallback(func(newConfig *config.Config, err error) {
    if err != nil {
        log.Printf("配置重载失败: %v", err)
        return
    }
    
    log.Println("配置已重载")
    // 应用新配置...
})

// 开始监听
if err := watcher.Start("config.yaml"); err != nil {
    log.Fatal(err)
}
```

### 配置文件示例

#### YAML格式 (`config.yaml`)
```yaml
app:
  name: "sngo-server"
  version: "1.0.0"
  environment: "development"
  debug: true
  description: "SNGO Actor Framework Server"

log:
  level: "debug"
  format: "json"
  output: "stdout"
  color: true

network:
  host: "0.0.0.0"
  port: 8080
  read_timeout: "30s"
  write_timeout: "30s"
  max_connections: 1000

actor:
  default_mailbox_size: 1000
  max_actors: 10000
  message_timeout: "30s"

discovery:
  enabled: true
  registry_address: "localhost:2379"
  service_ttl: "30s"
  health_check_interval: "10s"

monitor:
  enabled: true
  metrics_port: 9090
  profiling_enabled: false
```

#### JSON格式 (`config.json`)
```json
{
  "app": {
    "name": "sngo-server",
    "version": "1.0.0",
    "environment": "production",
    "debug": false
  },
  "network": {
    "host": "0.0.0.0",
    "port": 8080,
    "read_timeout": "30s",
    "write_timeout": "30s",
    "max_connections": 1000
  }
}
```

## 技术特性

### 1. 类型安全
- 强类型配置结构
- 编译时类型检查
- 运行时验证

### 2. 灵活性
- 多格式支持（YAML/JSON）
- 可配置搜索路径
- 自定义配置字段

### 3. 生产就绪
- 热重载支持
- 环境变量覆盖
- 完整的错误处理
- 配置验证

### 4. 性能优化
- 防抖机制减少不必要的重载
- 缓存机制提高访问性能
- 最小化内存分配

## 测试覆盖

### 测试用例
```go
func TestConfig(t *testing.T)                // 基本配置功能
func TestConfigValidation(t *testing.T)     // 配置验证
func TestLoader(t *testing.T)               // 配置加载
func TestLoaderJSON(t *testing.T)           // JSON格式支持
func TestEnvironmentOverrides(t *testing.T) // 环境变量覆盖
func TestAutoLoad(t *testing.T)             // 自动配置发现
func TestWatcher(t *testing.T)              // 热重载功能
func TestFileProvider(t *testing.T)         // 文件提供者
```

### 测试结果
```
=== RUN   TestConfig
--- PASS: TestConfig (0.00s)
=== RUN   TestConfigValidation
--- PASS: TestConfigValidation (0.00s)
=== RUN   TestLoader
--- PASS: TestLoader (0.00s)
=== RUN   TestLoaderJSON
--- PASS: TestLoaderJSON (0.00s)
=== RUN   TestEnvironmentOverrides
--- PASS: TestEnvironmentOverrides (0.00s)
=== RUN   TestAutoLoad
--- PASS: TestAutoLoad (0.00s)
=== RUN   TestWatcher
--- PASS: TestWatcher (0.70s)
=== RUN   TestFileProvider
--- PASS: TestFileProvider (0.60s)
PASS
```

## 架构优势

### 1. 模块化设计
- 清晰的职责分离
- 可替换的组件
- 易于扩展

### 2. 标准化
- 统一的配置格式
- 标准的环境变量命名
- 一致的API设计

### 3. 可观测性
- 配置变更日志
- 验证错误详情
- 性能指标

## 与其他模块的集成

### Core模块
- Actor系统配置
- 消息超时设置
- 并发控制参数

### Network模块
- 服务器监听配置
- 连接池设置
- 超时参数

### 未来集群模块
- 节点发现配置
- 集群通信设置
- 故障转移参数

## 下一步发展

第6阶段的配置系统为后续开发奠定了基础：

1. **第7阶段**: Bootstrap系统将使用配置系统
2. **第8阶段**: 集群功能需要配置支持
3. **生产部署**: 环境特定的配置管理

配置系统现在已经完全就绪，可以支持SNGO框架的所有核心功能和未来扩展。
