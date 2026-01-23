# 基于 Consul 的路由发现优化报告

## 1. 优化目标
本次优化旨在提升 `ConsulBalancer` 的**并发读取性能**和**系统健壮性**。
- **性能优化**：移除请求主链路上的互斥锁（Mutex），消除高并发场景下的锁竞争。
- **健壮性优化**：增加 Consul 连接异常时的指数退避重试机制，防止故障时不断重试导致资源耗尽；同时确保配置解析失败时保留旧规则（Graceful Degradation）。

## 2. 核心改动点

### 2.1 引入 `atomic.Value` 实现无锁读取
**问题**：原实现使用 `sync.RWMutex` 保护 `rules` map。每次请求路由（`SelectTarget`）都需要加读锁，每次配置更新（`watchRules`）需要加写锁。在高并发（万级 QPS）下，频繁的加锁/解锁操作会成为 CPU 热点。

**改进**：
- 将 `rules` 字段类型从 `map[string][]string` 改为 `atomic.Value`。
- `SelectTarget` 中使用 `Load()` 原子读取配置，完全无锁。
- `watchRules` 中构造新 map 后使用 `Store()` 原子替换。

**代码对比**：
```go
// Before
func (cb *ConsulBalancer) SelectTarget(...) string {
    cb.mu.RLock()
    defer cb.mu.RUnlock()
    // ...
}

// After
func (cb *ConsulBalancer) SelectTarget(...) string {
    rules := cb.rules.Load().(map[string][]string)
    // ...
}
```

### 2.2 指数退避重试 (Exponential Backoff)
**问题**：原实现当 Consul 连接失败或数据获取失败时，固定休眠 5 秒。如果 Consul 长期不可用，网关会持续以固定频率发起请求，缺乏弹性。

**改进**：
- 引入 `retryDelay` 变量，初始为 1s。
- 失败时休眠 `retryDelay`，然后翻倍（`retryDelay *= 2`），最大限制为 30s。
- 成功获取数据后，重置 `retryDelay` 为 1s。

### 2.3 优雅降级 (Graceful Degradation)
**问题**：配置解析失败（JSON 格式错误）时，如果不做处理，可能会导致规则为空或覆盖错误。

**改进**：
- 在 `watchRules` 中，只有当 `json.Unmarshal` 成功后，才调用 `cb.rules.Store(newRules)`。
- 如果解析失败，仅打印错误日志，**不更新**本地规则，确保网关仍能使用上一份正确的路由规则继续服务。

## 3. 性能预期
- **读路径**：由 `O(Lock)` 变为 `O(1)` 原子操作，理论上吞吐量不再受锁竞争限制。
- **写路径**：Copy-On-Write 模式，更新时会有一次 map 内存分配，但由于配置更新频率（秒级/分钟级）远低于请求频率（毫秒级），该开销可忽略不计。

## 4. 后续建议
- **监控集成**：建议添加 Prometheus 指标监控 `rules` 版本更新时间和解析失败次数。
- **健康检查**：目前仅从 KV 获取地址，建议升级为对接 Consul Service Health 接口，自动剔除不健康实例。
