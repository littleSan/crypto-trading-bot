package config

import (
	"testing"
)

func TestLoadConfig(t *testing.T) {
	// 从项目根目录的 test 目录加载测试配置
	// Load test config from test directory in project root
	cfg, err := LoadConfig("../../test/.env")
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// 验证配置是否正确加载
	// Verify config is loaded correctly
	if cfg.CryptoSymbol == "" {
		t.Errorf("Expected CryptoSymbol to be set, got empty string")
	}

	t.Logf("Successfully loaded config with CryptoSymbol: %s", cfg.CryptoSymbol)
}

func TestCalculateLookbackDays(t *testing.T) {
	tests := []struct {
		timeframe string
		expected  int
	}{
		{"15m", 5},
		{"1h", 10},
		{"4h", 15},
		{"1d", 60},
		{"5m", 10},      // 默认值
		{"unknown", 10}, // 默认值
	}

	for _, tt := range tests {
		t.Run(tt.timeframe, func(t *testing.T) {
			result := calculateLookbackDays(tt.timeframe)
			if result != tt.expected {
				t.Errorf("calculateLookbackDays(%s): expected %d, got %d",
					tt.timeframe, tt.expected, result)
			}
		})
	}
}
