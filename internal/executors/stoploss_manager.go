package executors

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/adshao/go-binance/v2/futures"
	"github.com/oak/crypto-trading-bot/internal/config"
	"github.com/oak/crypto-trading-bot/internal/logger"
)

// StopLossManager manages stop-loss for all active positions
// StopLossManager 管理所有活跃持仓的止损
type StopLossManager struct {
	positions map[string]*Position // symbol -> Position
	executor  *BinanceExecutor     // 执行器 / Executor
	config    *config.Config       // 配置 / Config
	logger    *logger.ColorLogger  // 日志 / Logger
	mu        sync.RWMutex         // 读写锁 / RW mutex
	ctx       context.Context      // 上下文 / Context
	cancel    context.CancelFunc   // 取消函数 / Cancel function
}

// NewStopLossManager creates a new StopLossManager
// NewStopLossManager 创建新的止损管理器
func NewStopLossManager(cfg *config.Config, executor *BinanceExecutor, log *logger.ColorLogger) *StopLossManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &StopLossManager{
		positions: make(map[string]*Position),
		executor:  executor,
		config:    cfg,
		logger:    log,
		ctx:       ctx,
		cancel:    cancel,
	}
}

// RegisterPosition registers a new position for stop-loss management
// RegisterPosition 注册新持仓进行止损管理
func (sm *StopLossManager) RegisterPosition(pos *Position) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	pos.HighestPrice = pos.EntryPrice // 初始化最高价/最低价 / Initialize highest/lowest
	pos.CurrentPrice = pos.EntryPrice
	pos.StopLossType = "fixed" // 初始为固定止损 / Initially fixed stop

	sm.positions[pos.Symbol] = pos
	sm.logger.Success(fmt.Sprintf("【%s】持仓已注册，入场价: %.2f, 初始止损: %.2f",
		pos.Symbol, pos.EntryPrice, pos.InitialStopLoss))
}

// RemovePosition removes a position from management
// RemovePosition 从管理中移除持仓
func (sm *StopLossManager) RemovePosition(symbol string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	delete(sm.positions, symbol)
	sm.logger.Info(fmt.Sprintf("【%s】持仓已移除", symbol))
}

// PlaceInitialStopLoss places initial stop-loss order for a position
// PlaceInitialStopLoss 为持仓下初始止损单
func (sm *StopLossManager) PlaceInitialStopLoss(ctx context.Context, pos *Position) error {
	return sm.placeStopLossOrder(ctx, pos, pos.InitialStopLoss)
}

// GetPosition gets a position by symbol
// GetPosition 根据交易对获取持仓
func (sm *StopLossManager) GetPosition(symbol string) *Position {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	return sm.positions[symbol]
}

// UpdatePosition updates position price and manages stop-loss
// UpdatePosition 更新持仓价格并管理止损
func (sm *StopLossManager) UpdatePosition(ctx context.Context, symbol string, currentPrice float64) error {
	sm.mu.Lock()
	pos, exists := sm.positions[symbol]
	if !exists {
		sm.mu.Unlock()
		return nil // 无持仓 / No position
	}
	sm.mu.Unlock()

	// Update price
	// 更新价格
	pos.UpdatePrice(currentPrice)

	// Phase 1: Manage stop ladder (move to breakeven)
	// 阶段 1: 阶梯式止损管理（移动到成本价）
	if err := sm.manageStopLadder(ctx, pos); err != nil {
		return fmt.Errorf("阶梯式止损管理失败: %w", err)
	}

	// Phase 2: Manage trailing stop
	// 阶段 2: 追踪止损管理
	if err := sm.manageTrailingStop(ctx, pos); err != nil {
		return fmt.Errorf("追踪止损管理失败: %w", err)
	}

	// Check if stop-loss should be triggered
	// 检查是否应该触发止损
	if pos.ShouldTriggerStopLoss() {
		sm.logger.Warning(fmt.Sprintf("【%s】触发止损！当前价: %.2f, 止损价: %.2f",
			pos.Symbol, pos.CurrentPrice, pos.CurrentStopLoss))
		return sm.executeStopLoss(ctx, pos)
	}

	return nil
}

// manageStopLadder implements ladder-style stop-loss management
// manageStopLadder 实现阶梯式止损管理
func (sm *StopLossManager) manageStopLadder(ctx context.Context, pos *Position) error {
	profitPct := pos.GetUnrealizedPnL()

	// Check strategy configuration
	// 检查策略配置
	strategy := sm.config.StopLossStrategy
	if strategy == "fixed" {
		// Fixed stop-loss only, no ladder management
		// 仅固定止损，不进行阶梯管理
		return nil
	}

	switch {
	case sm.config.EnableBreakeven && profitPct >= sm.config.BreakevenTrigger && pos.StopLossType == "fixed":
		// 达到保本触发点，移动到成本价（保本）
		// Reached breakeven trigger, move to entry price
		newStop := pos.EntryPrice
		if err := sm.moveStopLoss(ctx, pos, newStop, "breakeven",
			fmt.Sprintf("达到 %.1f%% 盈利，移至保本", sm.config.BreakevenTrigger*100)); err != nil {
			return err
		}
		pos.StopLossType = "breakeven"
		sm.logger.Success(fmt.Sprintf("【%s】✅ 止损移至成本价 %.2f（保本，当前盈利 %.2f%%）",
			pos.Symbol, newStop, profitPct*100))

	case sm.config.EnableTrailing && profitPct >= sm.config.TrailingTrigger && pos.StopLossType != "trailing":
		// 达到追踪止损触发点，启动追踪止损
		// Reached trailing trigger, activate trailing stop
		pos.StopLossType = "trailing"

		// Calculate initial trailing distance based on ATR
		// 基于 ATR 计算初始追踪距离
		if pos.ATR > 0 && pos.CurrentPrice > 0 {
			// Dynamic trailing distance: ATR% × multiplier
			// 动态追踪距离：ATR% × 倍数
			atrPercent := pos.ATR / pos.CurrentPrice
			pos.TrailingDistance = atrPercent * 2.5 // ATR 的 2.5 倍
			sm.logger.Success(fmt.Sprintf("【%s】🚀 启动追踪止损，动态距离 %.2f%% (ATR-based) （当前盈利 %.2f%%）",
				pos.Symbol, pos.TrailingDistance*100, profitPct*100))
		} else {
			// Fallback to configured initial distance
			// 回退到配置的初始距离
			pos.TrailingDistance = sm.config.TrailingDistanceInitial
			sm.logger.Success(fmt.Sprintf("【%s】🚀 启动追踪止损，距离 %.1f%% （当前盈利 %.2f%%）",
				pos.Symbol, pos.TrailingDistance*100, profitPct*100))
		}

	case sm.config.EnableTrailing && profitPct >= sm.config.TrailingTightenProfit &&
		pos.TrailingDistance > sm.config.TrailingDistanceTight:
		// 利润达到收紧阈值，收紧追踪距离
		// Profit reached tighten threshold, tighten trailing distance
		oldDistance := pos.TrailingDistance

		// Calculate tightened trailing distance based on ATR
		// 基于 ATR 计算收紧后的追踪距离
		if pos.ATR > 0 && pos.CurrentPrice > 0 {
			atrPercent := pos.ATR / pos.CurrentPrice
			pos.TrailingDistance = atrPercent * 2.0 // 收紧到 ATR 的 2 倍
			sm.logger.Info(fmt.Sprintf("【%s】📉 收紧追踪距离: %.2f%% → %.2f%% (ATR-based) （当前盈利 %.2f%%）",
				pos.Symbol, oldDistance*100, pos.TrailingDistance*100, profitPct*100))
		} else {
			// Fallback to configured tight distance
			// 回退到配置的收紧距离
			pos.TrailingDistance = sm.config.TrailingDistanceTight
			sm.logger.Info(fmt.Sprintf("【%s】📉 收紧追踪距离: %.1f%% → %.1f%% （当前盈利 %.2f%%）",
				pos.Symbol, oldDistance*100, pos.TrailingDistance*100, profitPct*100))
		}

	case sm.config.EnablePartialTakeProfit && profitPct >= sm.config.PartialTakeProfitTrigger &&
		!pos.PartialTPExecuted:
		// 部分止盈（可选功能，不推荐）
		// Partial take profit (optional, not recommended)
		sm.logger.Info(fmt.Sprintf("【%s】💰 达到分批止盈触发点 %.1f%% （当前盈利 %.2f%%）",
			pos.Symbol, sm.config.PartialTakeProfitTrigger*100, profitPct*100))
		// Note: Actual partial TP execution would be implemented here
		// 注意：实际的分批止盈执行逻辑需要在这里实现
		pos.PartialTPExecuted = true
	}

	return nil
}

// manageTrailingStop implements trailing stop-loss
// manageTrailingStop 实现追踪止损
func (sm *StopLossManager) manageTrailingStop(ctx context.Context, pos *Position) error {
	if pos.StopLossType != "trailing" {
		return nil
	}

	var newStop float64
	if pos.Side == "long" {
		newStop = pos.HighestPrice * (1 - pos.TrailingDistance)
	} else {
		// For short positions
		// 空仓
		newStop = pos.HighestPrice * (1 + pos.TrailingDistance)
	}

	// Stop-loss can only move in favorable direction
	// 止损只能朝有利方向移动
	shouldUpdate := false
	if pos.Side == "long" && newStop > pos.CurrentStopLoss {
		shouldUpdate = true
	} else if pos.Side == "short" && newStop < pos.CurrentStopLoss {
		shouldUpdate = true
	}

	if shouldUpdate {
		oldStop := pos.CurrentStopLoss
		if err := sm.moveStopLoss(ctx, pos, newStop, "trailing",
			fmt.Sprintf("追踪止损移动（最高价: %.2f）", pos.HighestPrice)); err != nil {
			return err
		}
		sm.logger.Info(fmt.Sprintf("【%s】📈 追踪止损移动: %.2f → %.2f (盈利: %.2f%%)",
			pos.Symbol, oldStop, newStop, pos.GetUnrealizedPnL()*100))
	}

	return nil
}

// moveStopLoss moves the stop-loss price and updates Binance order
// moveStopLoss 移动止损价格并更新币安订单
func (sm *StopLossManager) moveStopLoss(ctx context.Context, pos *Position, newStop float64, stopType, reason string) error {
	oldStop := pos.CurrentStopLoss

	// Record history
	// 记录历史
	pos.AddStopLossEvent(oldStop, newStop, reason, "program")

	// Cancel old stop-loss order if exists
	// 取消旧的止损单（如果存在）
	if pos.StopLossOrderID != "" {
		if err := sm.cancelStopLossOrder(ctx, pos); err != nil {
			sm.logger.Warning(fmt.Sprintf("取消旧止损单失败: %v", err))
			// Continue anyway / 继续执行
		}
	}

	// Place new stop-loss order
	// 下新的止损单
	if err := sm.placeStopLossOrder(ctx, pos, newStop); err != nil {
		return fmt.Errorf("下止损单失败: %w", err)
	}

	pos.CurrentStopLoss = newStop
	return nil
}

// placeStopLossOrder places a stop-loss order on Binance
// placeStopLossOrder 在币安下止损单
func (sm *StopLossManager) placeStopLossOrder(ctx context.Context, pos *Position, stopPrice float64) error {
	var orderSide futures.SideType
	if pos.Side == "short" {
		orderSide = futures.SideTypeBuy
	} else {
		orderSide = futures.SideTypeSell
	}

	binanceSymbol := sm.config.GetBinanceSymbolFor(pos.Symbol)

	// Create stop-loss order
	// 创建止损单
	order, err := sm.executor.client.NewCreateOrderService().
		Symbol(binanceSymbol).
		Side(orderSide).
		Type(futures.OrderTypeStopMarket).
		StopPrice(fmt.Sprintf("%.2f", stopPrice)).
		Quantity(fmt.Sprintf("%.4f", pos.Quantity)).
		ReduceOnly(true). // 只平仓不开仓 / Close only
		Do(ctx)

	if err != nil {
		return fmt.Errorf("下止损单失败: %w", err)
	}

	pos.StopLossOrderID = fmt.Sprintf("%d", order.OrderID)
	sm.logger.Success(fmt.Sprintf("【%s】止损单已下达: %.2f (订单ID: %s)",
		pos.Symbol, stopPrice, pos.StopLossOrderID))

	return nil
}

// cancelStopLossOrder cancels an existing stop-loss order
// cancelStopLossOrder 取消现有的止损单
func (sm *StopLossManager) cancelStopLossOrder(ctx context.Context, pos *Position) error {
	if pos.StopLossOrderID == "" {
		return nil
	}

	binanceSymbol := sm.config.GetBinanceSymbolFor(pos.Symbol)

	_, err := sm.executor.client.NewCancelOrderService().
		Symbol(binanceSymbol).
		OrderID(parseInt64(pos.StopLossOrderID)).
		Do(ctx)

	if err != nil {
		return fmt.Errorf("取消止损单失败: %w", err)
	}

	sm.logger.Info(fmt.Sprintf("【%s】旧止损单已取消: %s", pos.Symbol, pos.StopLossOrderID))
	pos.StopLossOrderID = ""

	return nil
}

// executeStopLoss executes stop-loss (close position)
// executeStopLoss 执行止损（平仓）
func (sm *StopLossManager) executeStopLoss(ctx context.Context, pos *Position) error {
	sm.logger.Warning(fmt.Sprintf("【%s】🛑 执行止损平仓", pos.Symbol))

	// Close position via market order
	// 通过市价单平仓
	action := ActionCloseLong
	if pos.Side == "short" {
		action = ActionCloseShort
	}

	result := sm.executor.ExecuteTrade(ctx, pos.Symbol, action, pos.Quantity, "触发止损")

	if result.Success {
		sm.logger.Success(fmt.Sprintf("【%s】止损平仓成功，盈亏: %.2f%%",
			pos.Symbol, pos.GetUnrealizedPnL()*100))
		sm.RemovePosition(pos.Symbol)
	} else {
		sm.logger.Error(fmt.Sprintf("【%s】止损平仓失败: %s", pos.Symbol, result.Message))
		return fmt.Errorf("止损平仓失败: %s", result.Message)
	}

	return nil
}

// MonitorPositions monitors all positions in real-time
// MonitorPositions 实时监控所有持仓
func (sm *StopLossManager) MonitorPositions(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	sm.logger.Info(fmt.Sprintf("🔍 启动持仓监控，间隔: %v", interval))

	for {
		select {
		case <-sm.ctx.Done():
			sm.logger.Info("持仓监控已停止")
			return

		case <-ticker.C:
			sm.mu.RLock()
			positions := make([]*Position, 0, len(sm.positions))
			for _, pos := range sm.positions {
				positions = append(positions, pos)
			}
			sm.mu.RUnlock()

			for _, pos := range positions {
				// Get latest price from Binance
				// 从币安获取最新价格
				currentPrice, err := sm.getCurrentPrice(sm.ctx, pos.Symbol)
				if err != nil {
					sm.logger.Warning(fmt.Sprintf("获取 %s 价格失败: %v", pos.Symbol, err))
					continue
				}

				// Update position and manage stop-loss
				// 更新持仓并管理止损
				if err := sm.UpdatePosition(sm.ctx, pos.Symbol, currentPrice); err != nil {
					sm.logger.Error(fmt.Sprintf("更新 %s 持仓失败: %v", pos.Symbol, err))
				}
			}
		}
	}
}

// getCurrentPrice gets current price from Binance
// getCurrentPrice 从币安获取当前价格
func (sm *StopLossManager) getCurrentPrice(ctx context.Context, symbol string) (float64, error) {
	binanceSymbol := sm.config.GetBinanceSymbolFor(symbol)

	prices, err := sm.executor.client.NewListPricesService().
		Symbol(binanceSymbol).
		Do(ctx)

	if err != nil {
		return 0, fmt.Errorf("获取价格失败: %w", err)
	}

	if len(prices) == 0 {
		return 0, fmt.Errorf("未获取到价格数据")
	}

	price, err := parseFloat(prices[0].Price)
	if err != nil {
		return 0, fmt.Errorf("解析价格失败: %w", err)
	}

	return price, nil
}

// GetAllPositions returns all active positions
// GetAllPositions 返回所有活跃持仓
func (sm *StopLossManager) GetAllPositions() []*Position {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	positions := make([]*Position, 0, len(sm.positions))
	for _, pos := range sm.positions {
		positions = append(positions, pos)
	}
	return positions
}

// Stop stops the stop-loss manager
// Stop 停止止损管理器
func (sm *StopLossManager) Stop() {
	sm.cancel()
}

// Helper function to parse int64
// 辅助函数：解析 int64
func parseInt64(s string) int64 {
	var i int64
	fmt.Sscanf(s, "%d", &i)
	return i
}
