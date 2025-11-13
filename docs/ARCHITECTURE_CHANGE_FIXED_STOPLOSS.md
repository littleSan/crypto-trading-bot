# 架构变更：固定止损策略 (Fixed Stop-Loss Strategy)

**日期**: 2025-11-13
**版本**: v2.0
**影响范围**: StopLossManager, cmd/main.go, cmd/web/main.go

## 📋 变更摘要

系统从**双重止损机制**（本地监控 + 币安止损单）简化为**纯服务器端止损机制**（仅币安止损单）。

## 🎯 变更原因

### 旧架构的问题

1. **重复执行风险**：
   - 币安止损单触发后，本地监控（10 秒轮询）可能再次尝试平仓
   - 导致"无持仓可平"错误

2. **性能开销**：
   - 每 10 秒为所有持仓调用币安 API 获取价格
   - 持仓数量 × (60/10) × 60 × 24 = 大量 API 调用/天

3. **响应速度慢**：
   - 本地监控：10 秒轮询间隔
   - 币安止损单：毫秒级触发

4. **可靠性依赖**：
   - 本地监控依赖程序持续运行
   - 网络波动影响价格获取
   - 币安止损单：24/7 服务器端监控，不依赖本地

### 新架构的优势

✅ **完全依赖币安 STOP_MARKET 订单**：
- 24/7 服务器端监控
- 毫秒级触发速度
- 即使本地程序崩溃仍会执行
- 无重复执行风险
- 减少 API 调用开销

## 🔧 技术变更

### 1. StopLossManager 职责重新定义

**之前**（双重止损）：
```go
// 1. 持仓管理
// 2. 下币安止损单
// 3. 本地价格监控（每 10 秒）
// 4. 本地止损触发检查
// 5. 本地平仓执行

go stopLossManager.MonitorPositions(10 * time.Second)  // 启用本地监控
```

**现在**（纯服务器端止损）：
```go
// 1. 持仓信息管理（注册、移除、查询）
// 2. 币安止损单生命周期管理（下单、取消）
// 3. 持仓数据存储和检索

// go stopLossManager.MonitorPositions(10 * time.Second)  // 已禁用
```

### 2. 已弃用的方法

以下方法标记为 `DEPRECATED`，不应在固定止损策略下使用：

- `MonitorPositions(interval time.Duration)`
- `UpdatePosition(ctx, symbol, currentPrice)`
- `executeStopLoss(ctx, pos)`

### 3. 文件修改

#### `cmd/main.go` (line 226-233)
```go
// 注释掉本地监控启动
// go stopLossManager.MonitorPositions(10 * time.Second) // 已弃用
```

#### `cmd/web/main.go` (line 177-187)
```go
// 注释掉本地监控启动
// go func() {
//     globalStopLossManager.MonitorPositions(10 * time.Second)
// }()
```

#### `internal/executors/stoploss_manager.go`
- 更新类型注释说明新架构
- 标记已弃用方法

## 📊 工作流程对比

### 开仓时（无变化）

1. LLM 输出初始止损价格（如 `$101,500`）
2. 解析器提取止损价格
3. 创建持仓对象
4. 注册到 StopLossManager
5. 调用 `PlaceInitialStopLoss()` 下币安止损单

### 持仓期间

**旧架构**：
```
币安服务器监控（毫秒级）
     ↓
   触发止损单
     ↓
   自动平仓
     ↓
   ❌ 10 秒后本地监控检测到
     ↓
   ❌ 尝试再次平仓（失败）
```

**新架构**：
```
币安服务器监控（毫秒级）
     ↓
   触发止损单
     ↓
   自动平仓
     ↓
   ✅ 完成（无本地干预）
```

### 止损触发时

**币安自动执行**：
```
价格触及止损价
     ↓
币安服务器立即触发 STOP_MARKET 订单
     ↓
按市价平仓
     ↓
本地程序下次运行时发现持仓已平
```

## 🚀 使用指南

### 推荐配置

在 `.env` 中使用固定止损策略：
```bash
# 使用固定止损 Prompt
TRADER_PROMPT_PATH=prompts/trader_fixed_stoploss.txt

# 确保测试模式（首次使用）
BINANCE_TEST_MODE=true
```

### LLM Prompt 要求

**必须输出初始止损价格**：
```
**交易方向**: BUY
**初始止损**: $101500
**仓位建议**: 35%资金
```

**HOLD 时不输出止损价格**（固定止损策略）：
```
**交易方向**: HOLD
**持仓建议**: 继续持有
（不输出止损价格）
```

### 验证止损单

可以通过币安 Web 界面或 API 查询止损单状态：
```bash
# 查询当前所有止损单
# 在币安期货界面 -> 委托订单 -> 条件单
```

## ⚠️ 重要提醒

1. **止损单类型**：`STOP_MARKET`（止损市价单）
2. **ReduceOnly**：只平仓不开仓（安全保护）
3. **服务器端执行**：即使本地程序关闭，止损单仍会执行
4. **固定止损**：止损价格在开仓时设定，之后不再调整
5. **手动平仓**：需要先取消止损单，否则会报错

## 📈 性能提升

| 指标 | 旧架构（双重止损） | 新架构（纯服务器端） | 改善 |
|------|------------------|-------------------|------|
| 止损触发延迟 | 最多 10 秒 | 毫秒级 | **99.9%** |
| API 调用频率 | 每 10 秒 × 持仓数 | 0 | **100%** |
| 重复执行风险 | 存在 | 无 | **消除** |
| 程序崩溃影响 | 止损失效 | 无影响 | **完全可靠** |
| CPU 使用率 | 后台协程轮询 | 无后台轮询 | **降低** |

## 🔄 回滚方案

如果需要恢复本地监控（不推荐），取消以下注释：

**cmd/main.go:233**:
```go
go stopLossManager.MonitorPositions(10 * time.Second)
```

**cmd/web/main.go:184-187**:
```go
go func() {
    log.Success("🔍 启动持仓监控，间隔: 10 秒")
    globalStopLossManager.MonitorPositions(10 * time.Second)
}()
```

**注意**：回滚后会恢复重复执行风险和性能问题。

## 📚 相关文档

- `prompts/README.md` - Prompt 策略对比
- `prompts/trader_fixed_stoploss.txt` - 固定止损策略 Prompt
- `prompts/trader_system.txt` - 动态止损策略 Prompt
- `docs/STOP_LOSS_GUIDE.md` - 止损系统完整指南（需更新）

## ✅ 测试建议

1. **测试模式验证**：
   ```bash
   BINANCE_TEST_MODE=true
   TRADER_PROMPT_PATH=prompts/trader_fixed_stoploss.txt
   make run-web
   ```

2. **检查止损单下达**：
   - 查看日志：`【BTC/USDT】止损单已下达: 101500.00 (订单ID: xxx)`
   - 币安界面查询止损单

3. **验证无监控日志**：
   - 不应看到：`🔍 启动持仓监控，间隔: 10 秒`

4. **模拟止损触发**：
   - 手动取消止损单（模拟触发）
   - 检查系统是否正常识别持仓已平

## 🎯 结论

此架构变更将系统从**混合模式**简化为**纯服务器端模式**，利用币安的高性能基础设施，提高了可靠性、响应速度，并降低了系统复杂度和资源消耗。

**推荐所有使用固定止损策略的用户采用此架构。**
