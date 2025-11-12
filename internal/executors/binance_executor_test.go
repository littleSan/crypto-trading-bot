package executors

import (
	"context"
	"fmt"
	"github.com/oak/crypto-trading-bot/internal/config"
	"github.com/oak/crypto-trading-bot/internal/logger"
	"os"
	"testing"
)

// TestBinanceExecutor_SetupExchange 测试交易所设置（需要有效的 API key）
// TestBinanceExecutor_SetupExchange tests exchange setup (requires valid API key)
func TestBinanceExecutor_SetupExchange(t *testing.T) {
	cfg, err := config.LoadConfig("../../test/.env")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	log := logger.NewColorLogger(true)
	executor := NewBinanceExecutor(cfg, log)
	err = executor.SetupExchange(context.Background(), "BTCUSDT", 10)
	if err != nil {
		t.Fatalf("failed to setup exchange: %v", err)
	}
	t.Logf("setup exchange success")
}

// TestBinanceConnecting 测试币安连接（带代理，使用公开 API 不需要 API key）
// TestBinanceConnecting tests Binance connection (with proxy, uses public API without API key)
func TestBinanceConnecting(t *testing.T) {
	cfg := &config.Config{
		BinanceAPIKey:               "",
		BinanceAPISecret:            "",
		BinanceProxy:                "http://192.168.0.226:6152",
		BinanceProxyInsecureSkipTLS: true, // 设置为 true 以跳过 TLS 验证（某些代理需要）/ Set to true to skip TLS verification (required by some proxies)
		BinanceLeverage:             10,
		BinanceTestMode:             false,
		BinancePositionMode:         "oneway",
	}

	log := logger.NewColorLogger(true)

	// 使用 NewBinanceExecutor 创建执行器（会自动配置代理）
	// Create executor using NewBinanceExecutor (automatically configures proxy)
	executor := NewBinanceExecutor(cfg, log)

	// 测试 Ping（公开 API，不需要 API key）
	// Test Ping (public API, no API key required)
	err := executor.client.NewPingService().Do(context.Background())
	if err != nil {
		t.Fatalf("failed to connect to binance: %v", err)
	}
	t.Logf("✅ Successfully connected to Binance via proxy!")
}
