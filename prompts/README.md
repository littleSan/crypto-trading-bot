# Trader Prompts

这个目录包含不同的交易策略 Prompt，你可以通过配置文件选择使用哪个 Prompt。

## 可用的 Prompt

### 1. `trader_system.txt` (默认 - 推荐)
**交易风格**：趋势交易，极度选择性
- 只在强趋势中交易（ADX > 25）
- 目标盈亏比 ≥ 2:1
- 置信度 ≥ 0.75 才交易
- 大部分时候 HOLD，耐心等待
- 适合追求长期稳定收益的交易者

### 2. `trader_aggressive.txt` (激进版)
**交易风格**：短线交易，积极捕捉机会
- 趋势或震荡突破时交易（ADX > 20）
- 目标盈亏比 ≥ 1.5:1
- 置信度 ≥ 0.6 即可交易
- 积极寻找机会，追求小赢积累
- 适合追求高频交易的激进交易者

## 如何使用

### 方法 1：在 `.env` 文件中配置

```bash
# 使用默认的趋势交易策略（推荐）
TRADER_PROMPT_PATH=prompts/trader_system.txt

# 或使用激进的短线策略
# TRADER_PROMPT_PATH=prompts/trader_aggressive.txt
```

### 方法 2：创建自己的 Prompt

1. 复制现有的 Prompt 文件：
```bash
cp prompts/trader_system.txt prompts/my_strategy.txt
```

2. 编辑 `prompts/my_strategy.txt`，调整交易哲学和决策原则

3. 在 `.env` 中指向你的自定义 Prompt：
```bash
TRADER_PROMPT_PATH=prompts/my_strategy.txt
```

4. 重启机器人，新的 Prompt 将生效

## Prompt 设计指南

一个好的 Trader Prompt 应该包含：

### 1. **交易哲学** (核心理念)
- 交易风格（趋势/震荡/套利）
- 风险偏好（保守/中性/激进）
- 时间周期（长线/短线）

### 2. **决策原则** (具体指导)
- 入场条件（技术指标、价格行为）
- 风险控制（止损、仓位）
- 出场策略（止盈、追踪止损）

### 3. **输出格式** (结构化输出)
必须包含：
- 交易方向（BUY/SELL/CLOSE_LONG/CLOSE_SHORT/HOLD）
- 置信度（0-1 的数值）
- 入场理由（为什么交易）
- 初始止损（具体价格）
- 预期盈亏比（风险回报比）
- 仓位建议（资金分配）

### 4. **重要提醒** (风险警告)
- 强调风险控制
- 提醒保持纪律
- 避免常见错误

## A/B 测试建议

对比不同 Prompt 的效果：

```bash
# 1. 测试保守策略（2周）
TRADER_PROMPT_PATH=prompts/trader_system.txt
make run-web

# 2. 测试激进策略（2周）
TRADER_PROMPT_PATH=prompts/trader_aggressive.txt
make run-web

# 3. 对比数据库统计
make query ARGS="stats"
```

关键指标对比：
- 总交易次数
- 执行率（执行次数 / 总会话数）
- 盈利交易 vs 亏损交易
- 平均盈亏比
- 最大回撤

## 注意事项

⚠️ **重要**：
1. 修改 Prompt 后需要**重启机器人**才能生效
2. 新的 Prompt 不会影响已有的持仓
3. 建议先在**测试模式**下验证新 Prompt（`BINANCE_TEST_MODE=true`）
4. 记录每个 Prompt 的表现，方便回溯和优化
5. 不要频繁更换 Prompt，至少观察 1-2 周的表现

## 常见问题

**Q: 如何知道当前使用的是哪个 Prompt？**
A: 启动时日志会显示：`使用交易策略 Prompt: prompts/trader_system.txt`

**Q: Prompt 文件损坏或读取失败怎么办？**
A: 系统会回退到代码中的默认 Prompt，并在日志中显示警告

**Q: 可以在运行时动态切换 Prompt 吗？**
A: 不可以，需要重启机器人。建议在非交易时间（K 线收盘后）重启

**Q: 如何备份我的 Prompt 配置？**
A: 使用 Git 管理 `prompts/` 目录，记录每次修改
