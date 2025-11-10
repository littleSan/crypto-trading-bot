package agents

import (
	"context"
	"fmt"
	"strings"
	"sync"

	openaiComponent "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/oak/crypto-trading-bot/internal/config"
	"github.com/oak/crypto-trading-bot/internal/dataflows"
	"github.com/oak/crypto-trading-bot/internal/executors"
	"github.com/oak/crypto-trading-bot/internal/logger"
)

// SymbolReports holds reports for a single symbol
// SymbolReports 保存单个交易对的报告
type SymbolReports struct {
	Symbol              string
	MarketReport        string
	CryptoReport        string
	SentimentReport     string
	PositionInfo        string
	OHLCVData           []dataflows.OHLCV
	TechnicalIndicators *dataflows.TechnicalIndicators
}

// AgentState holds the state of all analysts' reports for multiple symbols
// AgentState 保存所有分析师对多个交易对的报告状态
type AgentState struct {
	Symbols       []string                  // 所有交易对 / All trading pairs
	Timeframe     string                    // 时间周期 / Timeframe
	Reports       map[string]*SymbolReports // 每个交易对的报告 / Reports for each symbol
	FinalDecision string                    // 最终交易决策 / Final trading decision
	mu            sync.RWMutex              // 读写锁 / Read-write mutex
}

// NewAgentState creates a new agent state for multiple symbols
// NewAgentState 为多个交易对创建新的状态
func NewAgentState(symbols []string, timeframe string) *AgentState {
	reports := make(map[string]*SymbolReports)
	for _, symbol := range symbols {
		reports[symbol] = &SymbolReports{
			Symbol: symbol,
		}
	}
	return &AgentState{
		Symbols:   symbols,
		Timeframe: timeframe,
		Reports:   reports,
	}
}

// SetMarketReport sets the market analysis report for a symbol
// SetMarketReport 设置某个交易对的市场分析报告
func (s *AgentState) SetMarketReport(symbol, report string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r, exists := s.Reports[symbol]; exists {
		r.MarketReport = report
	}
}

// SetCryptoReport sets the crypto analysis report for a symbol
// SetCryptoReport 设置某个交易对的加密货币分析报告
func (s *AgentState) SetCryptoReport(symbol, report string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r, exists := s.Reports[symbol]; exists {
		r.CryptoReport = report
	}
}

// SetSentimentReport sets the sentiment analysis report for a symbol
// SetSentimentReport 设置某个交易对的情绪分析报告
func (s *AgentState) SetSentimentReport(symbol, report string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r, exists := s.Reports[symbol]; exists {
		r.SentimentReport = report
	}
}

// SetPositionInfo sets the position information for a symbol
// SetPositionInfo 设置某个交易对的持仓信息
func (s *AgentState) SetPositionInfo(symbol, info string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r, exists := s.Reports[symbol]; exists {
		r.PositionInfo = info
	}
}

// SetFinalDecision sets the final trading decision
// SetFinalDecision 设置最终交易决策
func (s *AgentState) SetFinalDecision(decision string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.FinalDecision = decision
}

// GetSymbolReports returns reports for a specific symbol
// GetSymbolReports 返回特定交易对的报告
func (s *AgentState) GetSymbolReports(symbol string) *SymbolReports {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Reports[symbol]
}

// GetAllReports returns all reports as a formatted string
// GetAllReports 返回所有报告的格式化字符串
func (s *AgentState) GetAllReports() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var sb strings.Builder

	// 为每个交易对生成报告 / Generate reports for each symbol
	for _, symbol := range s.Symbols {
		reports := s.Reports[symbol]
		sb.WriteString(fmt.Sprintf("\n================ %s 分析报告 ================\n", symbol))
		sb.WriteString("\n=== 市场技术分析 ===\n")
		sb.WriteString(reports.MarketReport)
		sb.WriteString("\n\n=== 加密货币专属分析 ===\n")
		sb.WriteString(reports.CryptoReport)
		sb.WriteString("\n\n=== 市场情绪分析 ===\n")
		sb.WriteString(reports.SentimentReport)
		sb.WriteString("\n\n=== 当前持仓信息 ===\n")
		sb.WriteString(reports.PositionInfo)
		sb.WriteString("\n")
	}

	return sb.String()
}

// SimpleTradingGraph creates a simplified trading workflow using Eino Graph
type SimpleTradingGraph struct {
	config   *config.Config
	logger   *logger.ColorLogger
	executor *executors.BinanceExecutor
	state    *AgentState
}

// NewSimpleTradingGraph creates a new simple trading graph
// NewSimpleTradingGraph 创建新的简单交易图
func NewSimpleTradingGraph(cfg *config.Config, log *logger.ColorLogger, executor *executors.BinanceExecutor) *SimpleTradingGraph {
	return &SimpleTradingGraph{
		config:   cfg,
		logger:   log,
		executor: executor,
		state:    NewAgentState(cfg.CryptoSymbols, cfg.CryptoTimeframe),
	}
}

// BuildGraph constructs the trading workflow graph with parallel execution
func (g *SimpleTradingGraph) BuildGraph(ctx context.Context) (compose.Runnable[map[string]any, map[string]any], error) {
	graph := compose.NewGraph[map[string]any, map[string]any]()

	marketData := dataflows.NewMarketData(g.config)

	// Market Analyst Lambda - Fetches market data and calculates indicators for all symbols
	// Market Analyst Lambda - 为所有交易对获取市场数据并计算指标
	marketAnalyst := compose.InvokableLambda(func(ctx context.Context, input map[string]any) (map[string]any, error) {
		g.logger.Info("🔍 市场分析师：正在获取所有交易对的市场数据...")

		timeframe := g.config.CryptoTimeframe
		lookbackDays := g.config.CryptoLookbackDays

		// 并行分析所有交易对 / Analyze all symbols in parallel
		var wg sync.WaitGroup
		var mu sync.Mutex
		results := make(map[string]any)

		for _, symbol := range g.state.Symbols {
			wg.Add(1)
			go func(sym string) {
				defer wg.Done()

				g.logger.Info(fmt.Sprintf("  📊 正在分析 %s...", sym))

				binanceSymbol := g.config.GetBinanceSymbolFor(sym)

				// Fetch OHLCV data
				ohlcvData, err := marketData.GetOHLCV(ctx, binanceSymbol, timeframe, lookbackDays)
				if err != nil {
					g.logger.Warning(fmt.Sprintf("  ⚠️  %s OHLCV数据获取失败: %v", sym, err))
					return
				}

				// Calculate indicators
				indicators := dataflows.CalculateIndicators(ohlcvData)

				// Generate report
				report := dataflows.FormatIndicatorReport(sym, timeframe, ohlcvData, indicators)

				// Save to state (thread-safe)
				mu.Lock()
				if reports := g.state.Reports[sym]; reports != nil {
					reports.OHLCVData = ohlcvData
					reports.TechnicalIndicators = indicators
				}
				mu.Unlock()

				g.state.SetMarketReport(sym, report)

				g.logger.Success(fmt.Sprintf("  ✅ %s 市场分析完成", sym))
			}(symbol)
		}

		wg.Wait()
		g.logger.Success("✅ 所有交易对的市场分析完成")

		return results, nil
	})

	// Crypto Analyst Lambda - Fetches funding rate, order book, 24h stats for all symbols
	// Crypto Analyst Lambda - 为所有交易对获取资金费率、订单簿、24小时统计
	cryptoAnalyst := compose.InvokableLambda(func(ctx context.Context, input map[string]any) (map[string]any, error) {
		g.logger.Info("🔍 加密货币分析师：正在获取所有交易对的链上数据...")

		// 并行分析所有交易对 / Analyze all symbols in parallel
		var wg sync.WaitGroup
		results := make(map[string]any)

		for _, symbol := range g.state.Symbols {
			wg.Add(1)
			go func(sym string) {
				defer wg.Done()

				g.logger.Info(fmt.Sprintf("  🔗 正在分析 %s 链上数据...", sym))

				binanceSymbol := g.config.GetBinanceSymbolFor(sym)
				var reportBuilder strings.Builder

				reportBuilder.WriteString(fmt.Sprintf("=== %s 加密货币数据 ===\n\n", sym))

				// Funding rate
				fundingRate, err := marketData.GetFundingRate(ctx, binanceSymbol)
				if err != nil {
					reportBuilder.WriteString(fmt.Sprintf("资金费率获取失败: %v\n", err))
				} else {
					reportBuilder.WriteString(fmt.Sprintf("资金费率: %.6f (%.4f%%)\n", fundingRate, fundingRate*100))
				}

				// Order book
				orderBook, err := marketData.GetOrderBook(ctx, binanceSymbol, 20)
				if err != nil {
					reportBuilder.WriteString(fmt.Sprintf("订单簿获取失败: %v\n", err))
				} else {
					reportBuilder.WriteString(fmt.Sprintf("订单簿 - 买单量: %.2f, 卖单量: %.2f, 买卖比: %.2f\n",
						orderBook["bid_volume"], orderBook["ask_volume"], orderBook["bid_ask_ratio"]))
				}

				// 24h stats
				stats, err := marketData.Get24HrStats(ctx, binanceSymbol)
				if err != nil {
					reportBuilder.WriteString(fmt.Sprintf("24h统计获取失败: %v\n", err))
				} else {
					reportBuilder.WriteString(fmt.Sprintf("24h统计 - 价格变化: %s%%, 最高: $%s, 最低: $%s, 成交量: %s\n",
						stats["price_change_percent"], stats["high_price"], stats["low_price"], stats["volume"]))
				}

				report := reportBuilder.String()
				g.state.SetCryptoReport(sym, report)

				g.logger.Success(fmt.Sprintf("  ✅ %s 加密货币分析完成", sym))
			}(symbol)
		}

		wg.Wait()
		g.logger.Success("✅ 所有交易对的加密货币分析完成")

		return results, nil
	})

	// Sentiment Analyst Lambda - Fetches market sentiment for all symbols
	// Sentiment Analyst Lambda - 为所有交易对获取市场情绪
	sentimentAnalyst := compose.InvokableLambda(func(ctx context.Context, input map[string]any) (map[string]any, error) {
		g.logger.Info("🔍 情绪分析师：正在获取所有交易对的市场情绪...")

		// 并行分析所有交易对 / Analyze all symbols in parallel
		var wg sync.WaitGroup
		results := make(map[string]any)

		for _, symbol := range g.state.Symbols {
			wg.Add(1)
			go func(sym string) {
				defer wg.Done()

				g.logger.Info(fmt.Sprintf("  😊 正在分析 %s 市场情绪...", sym))

				// Extract base symbol (BTC from BTC/USDT)
				// 提取基础币种（从 BTC/USDT 提取 BTC）
				baseSymbol := strings.Split(sym, "/")[0]

				sentiment := dataflows.GetSentimentIndicators(ctx, baseSymbol)
				report := dataflows.FormatSentimentReport(sentiment)

				g.state.SetSentimentReport(sym, report)

				g.logger.Success(fmt.Sprintf("  ✅ %s 情绪分析完成", sym))
			}(symbol)
		}

		wg.Wait()
		g.logger.Success("✅ 所有交易对的情绪分析完成")

		return results, nil
	})

	// Position Info Lambda - Gets current position for all symbols
	// Position Info Lambda - 获取所有交易对的持仓信息
	positionInfo := compose.InvokableLambda(func(ctx context.Context, input map[string]any) (map[string]any, error) {
		g.logger.Info("📊 获取所有交易对的持仓信息...")

		// 并行获取所有交易对的持仓 / Get positions for all symbols in parallel
		var wg sync.WaitGroup
		results := make(map[string]any)

		for _, symbol := range g.state.Symbols {
			wg.Add(1)
			go func(sym string) {
				defer wg.Done()

				g.logger.Info(fmt.Sprintf("  📈 正在获取 %s 持仓...", sym))

				posInfo := g.executor.GetPositionSummary(ctx, sym)
				g.state.SetPositionInfo(sym, posInfo)

				g.logger.Success(fmt.Sprintf("  ✅ %s 持仓信息获取完成", sym))
			}(symbol)
		}

		wg.Wait()
		g.logger.Success("✅ 所有交易对的持仓信息获取完成")

		return results, nil
	})

	// Trader Lambda - Makes final decision using LLM
	trader := compose.InvokableLambda(func(ctx context.Context, input map[string]any) (map[string]any, error) {
		g.logger.Info("🤖 交易员：正在制定交易策略...")

		allReports := g.state.GetAllReports()

		// Try to use LLM for decision, fall back to simple rules if LLM fails
		var decision string
		var err error

		// Check if API key is configured
		if g.config.APIKey != "" && g.config.APIKey != "your_openai_key" {
			// ! Use LLM for decision
			decision, err = g.makeLLMDecision(ctx)
			if err != nil {
				g.logger.Warning(fmt.Sprintf("LLM 决策失败: %v", err))
				decision = g.makeSimpleDecision()
			}
		} else {
			g.logger.Info("OpenAI API Key 未配置，使用简单规则决策")
			decision = g.makeSimpleDecision()
		}

		g.state.SetFinalDecision(decision)

		g.logger.Decision(decision)

		return map[string]any{
			"decision":    decision,
			"all_reports": allReports,
		}, nil
	})

	// Add nodes to graph
	if err := graph.AddLambdaNode("market_analyst", marketAnalyst); err != nil {
		return nil, err
	}
	if err := graph.AddLambdaNode("crypto_analyst", cryptoAnalyst); err != nil {
		return nil, err
	}
	if err := graph.AddLambdaNode("sentiment_analyst", sentimentAnalyst); err != nil {
		return nil, err
	}
	if err := graph.AddLambdaNode("position_info", positionInfo); err != nil {
		return nil, err
	}
	if err := graph.AddLambdaNode("trader", trader); err != nil {
		return nil, err
	}

	// Parallel execution: market_analyst and sentiment_analyst run in parallel
	if err := graph.AddEdge(compose.START, "market_analyst"); err != nil {
		return nil, err
	}
	if err := graph.AddEdge(compose.START, "sentiment_analyst"); err != nil {
		return nil, err
	}

	// After market_analyst completes, run crypto_analyst
	if err := graph.AddEdge("market_analyst", "crypto_analyst"); err != nil {
		return nil, err
	}

	// After crypto_analyst completes, get position info
	if err := graph.AddEdge("crypto_analyst", "position_info"); err != nil {
		return nil, err
	}

	// Wait for both sentiment_analyst and position_info before trader
	if err := graph.AddEdge("sentiment_analyst", "trader"); err != nil {
		return nil, err
	}
	if err := graph.AddEdge("position_info", "trader"); err != nil {
		return nil, err
	}

	// Trader outputs to END
	if err := graph.AddEdge("trader", compose.END); err != nil {
		return nil, err
	}

	// Compile with AllPredecessor trigger mode (wait for all inputs)
	return graph.Compile(ctx, compose.WithNodeTriggerMode(compose.AllPredecessor))
}

// makeSimpleDecision creates a simple rule-based decision (fallback when LLM is disabled)
// makeSimpleDecision 创建基于规则的简单决策（LLM 禁用时的后备方案）
func (g *SimpleTradingGraph) makeSimpleDecision() string {
	var decision strings.Builder

	decision.WriteString("=== 多币种交易决策分析 ===\n\n")
	decision.WriteString("说明: 这是基于规则的简单决策（LLM 未启用）。\n\n")

	// Analyze each symbol
	// 分析每个交易对
	for _, symbol := range g.state.Symbols {
		reports := g.state.GetSymbolReports(symbol)
		if reports == nil {
			continue
		}

		decision.WriteString(fmt.Sprintf("【%s】\n", symbol))

		// Analyze technical indicators if available
		// 如果有技术指标数据，进行分析
		if reports.TechnicalIndicators != nil && len(reports.OHLCVData) > 0 {
			lastIdx := len(reports.OHLCVData) - 1
			rsi := reports.TechnicalIndicators.RSI
			macd := reports.TechnicalIndicators.MACD
			signal := reports.TechnicalIndicators.Signal

			decision.WriteString("技术面分析:\n")

			// RSI analysis
			if len(rsi) > lastIdx {
				rsiVal := rsi[lastIdx]
				decision.WriteString(fmt.Sprintf("- RSI(14): %.2f ", rsiVal))
				if rsiVal > 70 {
					decision.WriteString("(超买区域，可能回调)\n")
				} else if rsiVal < 30 {
					decision.WriteString("(超卖区域，可能反弹)\n")
				} else {
					decision.WriteString("(中性区域)\n")
				}
			}

			// MACD analysis
			if len(macd) > lastIdx && len(signal) > lastIdx {
				macdVal := macd[lastIdx]
				signalVal := signal[lastIdx]
				decision.WriteString(fmt.Sprintf("- MACD: %.2f, Signal: %.2f ", macdVal, signalVal))
				if macdVal > signalVal {
					decision.WriteString("(MACD在Signal之上，多头信号)\n")
				} else {
					decision.WriteString("(MACD在Signal之下，空头信号)\n")
				}
			}
		}

		decision.WriteString(fmt.Sprintf("**建议**: HOLD（观望）\n\n"))
	}

	decision.WriteString("\n**最终决策**: HOLD（观望）\n")
	decision.WriteString("说明: 规则决策默认观望，建议启用 LLM 获得更智能的决策。\n")

	return decision.String()
}

// makeLLMDecision uses LLM to generate trading decision
func (g *SimpleTradingGraph) makeLLMDecision(ctx context.Context) (string, error) {
	// Create OpenAI config
	cfg := &openaiComponent.ChatModelConfig{
		APIKey:  g.config.APIKey,
		BaseURL: g.config.BackendURL,
		Model:   g.config.QuickThinkLLM,
	}

	// Create ChatModel
	chatModel, err := openaiComponent.NewChatModel(ctx, cfg)
	if err != nil {
		g.logger.Warning(fmt.Sprintf("LLM 初始化失败，使用简单规则决策: %v", err))
		return g.makeSimpleDecision(), nil
	}

	// Prepare the prompt with all reports
	allReports := g.state.GetAllReports()

	systemPrompt := `你是一位经验丰富的加密货币交易员，负责将多个交易对的分析报告整合成具体的交易决策。

**多币种决策策略**：
1. **综合评估**：查看所有交易对的市场、链上、情绪数据，找出最优机会
2. **选择性交易**：从所有交易对中选择 1-2 个最有说服力的交易机会，避免过度分散
3. **风险管理**：考虑仓位分配，单个交易对风险不超过总资金的 30%
4. **高盈亏比**：只选择盈亏比 > 2:1 的交易，耐心等待大机会
5. **仓位控制**：如果已有持仓，考虑是否需要调整或平仓

**决策输出格式**（重要）：
为每个交易对给出明确决策，格式如下：

【交易对名称】
**交易方向**: BUY / SELL / CLOSE_LONG / CLOSE_SHORT / HOLD
**置信度**: 0-1 的数值，如 0.8
**理由**: 简明扼要的理由（1-2 句话）
**仓位建议**: 如 "30% 资金" 或 "维持观望"

---

最后必须包含一个总结，说明为什么选择这些交易对，以及整体的风险控制策略。

请用中文回答，语言简洁专业。`

	userPrompt := fmt.Sprintf(`请分析以下数据并给出交易决策：

%s

请给出你的分析和最终决策。`, allReports)

	// Create messages
	messages := []*schema.Message{
		schema.SystemMessage(systemPrompt),
		schema.UserMessage(userPrompt),
	}

	// Call LLM
	g.logger.Info(fmt.Sprintf("🤖 正在调用 LLM 生成交易决策, 使用的模型:%v", g.config.QuickThinkLLM))
	response, err := chatModel.Generate(ctx, messages)
	if err != nil {
		g.logger.Warning(fmt.Sprintf("LLM 调用失败，使用简单规则决策: %v", err))
		return g.makeSimpleDecision(), nil
	}

	g.logger.Success("✅ LLM 决策生成完成")

	// Log token usage if available
	if response.ResponseMeta != nil && response.ResponseMeta.Usage != nil {
		g.logger.Info(fmt.Sprintf("Token 使用: %d (输入: %d, 输出: %d)",
			response.ResponseMeta.Usage.TotalTokens,
			response.ResponseMeta.Usage.PromptTokens,
			response.ResponseMeta.Usage.CompletionTokens))
	}

	return response.Content, nil
}

// Run executes the trading graph
func (g *SimpleTradingGraph) Run(ctx context.Context) (map[string]any, error) {
	g.logger.Header("启动交易分析工作流", '=', 80)

	compiled, err := g.BuildGraph(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to build graph: %w", err)
	}

	input := map[string]any{
		"symbol":    g.config.CryptoSymbol,
		"timeframe": g.config.CryptoTimeframe,
	}

	result, err := compiled.Invoke(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("graph execution failed: %w", err)
	}

	g.logger.Header("工作流执行完成", '=', 80)

	return result, nil
}

// GetState returns the current agent state
func (g *SimpleTradingGraph) GetState() *AgentState {
	return g.state
}
