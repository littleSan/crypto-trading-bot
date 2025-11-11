package web

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/oak/crypto-trading-bot/internal/config"
	"github.com/oak/crypto-trading-bot/internal/executors"
	"github.com/oak/crypto-trading-bot/internal/logger"
	"github.com/oak/crypto-trading-bot/internal/storage"
)

// Server represents the web monitoring server
// Server 表示 Web 监控服务器
type Server struct {
	config          *config.Config
	logger          *logger.ColorLogger
	storage         *storage.Storage
	stopLossManager *executors.StopLossManager
	hertz           *server.Hertz
}

// NewServer creates a new web monitoring server
// NewServer 创建新的 Web 监控服务器
func NewServer(cfg *config.Config, log *logger.ColorLogger, db *storage.Storage, stopLossMgr *executors.StopLossManager) *Server {
	h := server.Default(server.WithHostPorts(fmt.Sprintf(":%d", cfg.WebPort)))

	s := &Server{
		config:          cfg,
		logger:          log,
		storage:         db,
		stopLossManager: stopLossMgr,
		hertz:           h,
	}

	s.setupRoutes()

	return s
}

// setupRoutes configures all HTTP routes
// setupRoutes 配置所有 HTTP 路由
func (s *Server) setupRoutes() {
	// Static pages
	// 静态页面
	s.hertz.GET("/", s.handleIndex)
	s.hertz.GET("/sessions", s.handleSessions)
	s.hertz.GET("/session/:id", s.handleSessionDetail)
	s.hertz.GET("/stats", s.handleStats)
	s.hertz.GET("/health", s.handleHealth)

	// New API endpoints
	// 新增 API 端点
	s.hertz.GET("/api/positions", s.handlePositions)
	s.hertz.GET("/api/positions/:symbol", s.handlePositionsBySymbol)
	s.hertz.GET("/api/symbols", s.handleSymbols)
}

// handleIndex renders the main dashboard
// handleIndex 渲染主仪表板
func (s *Server) handleIndex(ctx context.Context, c *app.RequestContext) {
	// Get stats for the first symbol (or aggregate later)
	// 获取第一个交易对的统计（或稍后聚合）
	var stats map[string]interface{}
	var err error
	if len(s.config.CryptoSymbols) > 0 {
		stats, err = s.storage.GetSessionStats(s.config.CryptoSymbols[0])
		if err != nil {
			c.JSON(http.StatusInternalServerError, utils.H{"error": err.Error()})
			return
		}
	} else {
		stats = map[string]interface{}{
			"total_sessions": 0,
			"executed_count": 0,
			"execution_rate": 0.0,
		}
	}

	sessions, err := s.storage.GetLatestSessions(10)
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.H{"error": err.Error()})
		return
	}

	// Get active positions
	// 获取活跃持仓
	positions, _ := s.storage.GetActivePositions()

	// Create template with custom functions
	// 创建带自定义函数的模板
	funcMap := template.FuncMap{
		"mul": func(a, b float64) float64 {
			return a * b
		},
		"extractAction": extractActionFromDecision,
	}
	tmpl := template.Must(template.New("index.html").Funcs(funcMap).ParseFiles("internal/web/templates/index.html"))

	data := map[string]interface{}{
		"Symbols":         s.config.CryptoSymbols,
		"Timeframe":       s.config.CryptoTimeframe,
		"Stats":           stats,
		"Sessions":        sessions,
		"Positions":       positions,
		"CurrentTime":     time.Now().Format("2006-01-02 15:04:05"),
		"LLMEnabled":      s.config.APIKey != "" && s.config.APIKey != "your_openai_key",
		"TestMode":        s.config.BinanceTestMode,
		"AutoExecute":     s.config.AutoExecute,
		"LeverageMin":     s.config.BinanceLeverageMin,
		"LeverageMax":     s.config.BinanceLeverageMax,
		"LeverageDynamic": s.config.BinanceLeverageDynamic,
	}

	// Execute template and render
	// 执行模板并渲染
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		c.JSON(http.StatusInternalServerError, utils.H{"error": err.Error()})
		return
	}

	c.Data(http.StatusOK, "text/html; charset=utf-8", buf.Bytes())
}

// handleSessions returns JSON list of sessions
func (s *Server) handleSessions(ctx context.Context, c *app.RequestContext) {
	limit := c.DefaultQuery("limit", "20")
	var limitInt int
	fmt.Sscanf(limit, "%d", &limitInt)

	sessions, err := s.storage.GetLatestSessions(limitInt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, utils.H{
		"sessions": sessions,
		"count":    len(sessions),
	})
}

// handleSessionDetail returns details of a specific session
func (s *Server) handleSessionDetail(ctx context.Context, c *app.RequestContext) {
	// This would require implementing GetSessionByID in storage
	c.JSON(http.StatusOK, utils.H{
		"message": "Session detail endpoint - to be implemented",
	})
}

// handleStats returns statistics
// handleStats 返回统计信息
func (s *Server) handleStats(ctx context.Context, c *app.RequestContext) {
	// Get symbol from query parameter, or use first symbol
	// 从查询参数获取交易对，或使用第一个交易对
	symbol := c.DefaultQuery("symbol", "")
	if symbol == "" && len(s.config.CryptoSymbols) > 0 {
		symbol = s.config.CryptoSymbols[0]
	}

	if symbol == "" {
		c.JSON(http.StatusBadRequest, utils.H{"error": "no symbol specified"})
		return
	}

	stats, err := s.storage.GetSessionStats(symbol)
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// handleHealth returns health status
func (s *Server) handleHealth(ctx context.Context, c *app.RequestContext) {
	c.JSON(http.StatusOK, utils.H{
		"status":  "healthy",
		"time":    time.Now(),
		"version": "1.0.0",
	})
}

// Start starts the web server
func (s *Server) Start() error {
	s.logger.Success(fmt.Sprintf("Web 监控启动: http://localhost:%d", s.config.WebPort))
	s.hertz.Spin()
	return nil
}

// Stop stops the web server
func (s *Server) Stop(ctx context.Context) error {
	return s.hertz.Shutdown(ctx)
}

// handlePositions returns all active positions
// handlePositions 返回所有活跃持仓
func (s *Server) handlePositions(ctx context.Context, c *app.RequestContext) {
	positions, err := s.storage.GetActivePositions()
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, utils.H{
		"positions": positions,
		"count":     len(positions),
	})
}

// handlePositionsBySymbol returns positions for a specific symbol
// handlePositionsBySymbol 返回特定交易对的持仓
func (s *Server) handlePositionsBySymbol(ctx context.Context, c *app.RequestContext) {
	symbol := c.Param("symbol")
	positions, err := s.storage.GetPositionsBySymbol(symbol)
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, utils.H{
		"symbol":    symbol,
		"positions": positions,
		"count":     len(positions),
	})
}

// handleSymbols returns all configured trading symbols
// handleSymbols 返回所有配置的交易对
func (s *Server) handleSymbols(ctx context.Context, c *app.RequestContext) {
	c.JSON(http.StatusOK, utils.H{
		"symbols":   s.config.CryptoSymbols,
		"count":     len(s.config.CryptoSymbols),
		"timeframe": s.config.CryptoTimeframe,
	})
}

// extractActionFromDecision extracts trading action from decision text
// extractActionFromDecision 从决策文本中提取交易动作
func extractActionFromDecision(decision string) string {
	if decision == "" {
		return "UNKNOWN"
	}

	// Convert to uppercase for matching
	// 转换为大写用于匹配
	text := strings.ToUpper(decision)

	// Try to find action patterns
	// 尝试查找动作模式
	patterns := []struct {
		action  string
		matches []string
	}{
		{"BUY", []string{"**交易方向**: BUY", "交易方向: BUY", "ACTION: BUY", "决策: BUY", "建议.*?买入", "建议.*?做多", "开多"}},
		{"SELL", []string{"**交易方向**: SELL", "交易方向: SELL", "ACTION: SELL", "决策: SELL", "建议.*?卖出", "建议.*?做空", "开空"}},
		{"CLOSE_LONG", []string{"**交易方向**: CLOSE_LONG", "交易方向: CLOSE_LONG", "ACTION: CLOSE_LONG", "决策: CLOSE_LONG", "平多", "平掉多单"}},
		{"CLOSE_SHORT", []string{"**交易方向**: CLOSE_SHORT", "交易方向: CLOSE_SHORT", "ACTION: CLOSE_SHORT", "决策: CLOSE_SHORT", "平空", "平掉空单"}},
		{"HOLD", []string{"**交易方向**: HOLD", "交易方向: HOLD", "ACTION: HOLD", "决策: HOLD", "观望", "持有", "不建议操作"}},
	}

	for _, p := range patterns {
		for _, pattern := range p.matches {
			// Try literal match first
			// 先尝试字面匹配
			if strings.Contains(text, strings.ToUpper(pattern)) {
				return p.action
			}
			// Try regex match
			// 尝试正则匹配
			if matched, _ := regexp.MatchString(pattern, text); matched {
				return p.action
			}
		}
	}

	return "HOLD"
}
