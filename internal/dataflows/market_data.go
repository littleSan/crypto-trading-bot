package dataflows

import (
	"context"
	"crypto/tls"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/adshao/go-binance/v2/futures"
	"github.com/oak/crypto-trading-bot/internal/config"
)

// OHLCV represents a candlestick data point
type OHLCV struct {
	Timestamp time.Time
	Open      float64
	High      float64
	Low       float64
	Close     float64
	Volume    float64
}

// TechnicalIndicators holds calculated technical indicators
type TechnicalIndicators struct {
	RSI       []float64
	MACD      []float64
	Signal    []float64
	BB_Upper  []float64
	BB_Middle []float64
	BB_Lower  []float64
	SMA_20    []float64
	SMA_50    []float64
	SMA_200   []float64
	EMA_12    []float64
	EMA_26    []float64
	ATR       []float64
	Volume    []float64

	// New indicators for trend strength and confirmation
	// 新增指标：趋势强度和确认
	ADX         []float64 // Average Directional Index - 趋势强度
	DI_Plus     []float64 // +DI - 上升趋向指标
	DI_Minus    []float64 // -DI - 下降趋向指标
	VolumeRatio []float64 // Volume Ratio - 成交量比率
}

// MarketData handles crypto market data fetching
type MarketData struct {
	client *futures.Client
	config *config.Config
}

// NewMarketData creates a new MarketData instance
// Note: For public endpoints (klines, orderbook, etc.), API key is not required
func NewMarketData(cfg *config.Config) *MarketData {
	futures.UseTestnet = cfg.BinanceTestMode

	// For public data endpoints, we can use empty API credentials
	// Only private endpoints (account info, trading) require valid credentials
	apiKey := ""
	apiSecret := ""

	// If API credentials are provided, use them (for authenticated endpoints)
	if cfg.BinanceAPIKey != "" && cfg.BinanceAPISecret != "" {
		apiKey = cfg.BinanceAPIKey
		apiSecret = cfg.BinanceAPISecret
	}

	client := futures.NewClient(apiKey, apiSecret)

	// Set proxy if configured
	if cfg.BinanceProxy != "" {
		proxyURL, err := url.Parse(cfg.BinanceProxy)
		if err == nil {
			// Create custom HTTP client with proxy
			httpClient := &http.Client{
				Transport: &http.Transport{
					Proxy: http.ProxyURL(proxyURL),
					TLSClientConfig: &tls.Config{
						InsecureSkipVerify: false,
					},
				},
				Timeout: 30 * time.Second,
			}
			client.HTTPClient = httpClient
		}
	}

	return &MarketData{
		client: client,
		config: cfg,
	}
}

// GetOHLCV fetches OHLCV data for a symbol
func (m *MarketData) GetOHLCV(ctx context.Context, symbol string, timeframe string, lookbackDays int) ([]OHLCV, error) {
	interval := convertTimeframe(timeframe)

	startTime := time.Now().AddDate(0, 0, -lookbackDays)
	endTime := time.Now()

	klines, err := m.client.NewKlinesService().
		Symbol(symbol).
		Interval(interval).
		StartTime(startTime.UnixMilli()).
		EndTime(endTime.UnixMilli()).
		Limit(1000).
		Do(ctx)

	if err != nil {
		return nil, fmt.Errorf("failed to fetch klines: %w", err)
	}

	ohlcvData := make([]OHLCV, 0, len(klines))
	for _, k := range klines {
		open, _ := strconv.ParseFloat(k.Open, 64)
		high, _ := strconv.ParseFloat(k.High, 64)
		low, _ := strconv.ParseFloat(k.Low, 64)
		closePrice, _ := strconv.ParseFloat(k.Close, 64)
		volume, _ := strconv.ParseFloat(k.Volume, 64)

		ohlcvData = append(ohlcvData, OHLCV{
			Timestamp: time.Unix(k.OpenTime/1000, 0),
			Open:      open,
			High:      high,
			Low:       low,
			Close:     closePrice,
			Volume:    volume,
		})
	}

	return ohlcvData, nil
}

// CalculateIndicators calculates technical indicators from OHLCV data
func CalculateIndicators(ohlcvData []OHLCV) *TechnicalIndicators {
	if len(ohlcvData) == 0 {
		return &TechnicalIndicators{}
	}

	// Extract price and volume arrays
	closes := make([]float64, len(ohlcvData))
	highs := make([]float64, len(ohlcvData))
	lows := make([]float64, len(ohlcvData))
	volumes := make([]float64, len(ohlcvData))

	for i, candle := range ohlcvData {
		closes[i] = candle.Close
		highs[i] = candle.High
		lows[i] = candle.Low
		volumes[i] = candle.Volume
	}

	// Calculate indicators
	rsi := calculateRSI(closes, 14)
	macd, signal := calculateMACD(closes)
	bbUpper, bbMiddle, bbLower := calculateBollingerBands(closes, 20, 2.0)
	sma20 := calculateSMA(closes, 20)
	sma50 := calculateSMA(closes, 50)
	sma200 := calculateSMA(closes, 200)
	ema12 := calculateEMA(closes, 12)
	ema26 := calculateEMA(closes, 26)
	atr := calculateATR(highs, lows, closes, 14)

	// New indicators for trend strength and volume confirmation
	// 新增指标：趋势强度和成交量确认
	adx, diPlus, diMinus := calculateADX(highs, lows, closes, 14)
	volumeRatio := calculateVolumeRatio(volumes, 20)

	return &TechnicalIndicators{
		RSI:       rsi,
		MACD:      macd,
		Signal:    signal,
		BB_Upper:  bbUpper,
		BB_Middle: bbMiddle,
		BB_Lower:  bbLower,
		SMA_20:    sma20,
		SMA_50:    sma50,
		SMA_200:   sma200,
		EMA_12:    ema12,
		EMA_26:    ema26,
		ATR:       atr,
		Volume:    volumes,

		// New indicators
		// 新增指标
		ADX:         adx,
		DI_Plus:     diPlus,
		DI_Minus:    diMinus,
		VolumeRatio: volumeRatio,
	}
}

// calculateSMA calculates Simple Moving Average
func calculateSMA(data []float64, period int) []float64 {
	result := make([]float64, len(data))
	for i := range data {
		if i < period-1 {
			result[i] = math.NaN()
			continue
		}
		sum := 0.0
		for j := 0; j < period; j++ {
			sum += data[i-j]
		}
		result[i] = sum / float64(period)
	}
	return result
}

// calculateEMA calculates Exponential Moving Average
func calculateEMA(data []float64, period int) []float64 {
	result := make([]float64, len(data))
	multiplier := 2.0 / float64(period+1)

	// First EMA value is SMA
	sum := 0.0
	for i := 0; i < period && i < len(data); i++ {
		sum += data[i]
		result[i] = math.NaN()
	}
	if len(data) >= period {
		result[period-1] = sum / float64(period)
	}

	// Calculate EMA for remaining values
	for i := period; i < len(data); i++ {
		result[i] = (data[i]-result[i-1])*multiplier + result[i-1]
	}

	return result
}

// calculateRSI calculates Relative Strength Index
func calculateRSI(data []float64, period int) []float64 {
	result := make([]float64, len(data))

	if len(data) < period+1 {
		for i := range result {
			result[i] = math.NaN()
		}
		return result
	}

	gains := make([]float64, len(data))
	losses := make([]float64, len(data))

	for i := 1; i < len(data); i++ {
		change := data[i] - data[i-1]
		if change > 0 {
			gains[i] = change
		} else {
			losses[i] = -change
		}
	}

	avgGain := 0.0
	avgLoss := 0.0
	for i := 1; i <= period; i++ {
		avgGain += gains[i]
		avgLoss += losses[i]
	}
	avgGain /= float64(period)
	avgLoss /= float64(period)

	for i := 0; i < period; i++ {
		result[i] = math.NaN()
	}

	for i := period; i < len(data); i++ {
		if i == period {
			if avgLoss == 0 {
				result[i] = 100
			} else {
				rs := avgGain / avgLoss
				result[i] = 100 - (100 / (1 + rs))
			}
		} else {
			avgGain = (avgGain*float64(period-1) + gains[i]) / float64(period)
			avgLoss = (avgLoss*float64(period-1) + losses[i]) / float64(period)

			if avgLoss == 0 {
				result[i] = 100
			} else {
				rs := avgGain / avgLoss
				result[i] = 100 - (100 / (1 + rs))
			}
		}
	}

	return result
}

// calculateMACD calculates MACD and Signal line
func calculateMACD(data []float64) ([]float64, []float64) {
	ema12 := calculateEMA(data, 12)
	ema26 := calculateEMA(data, 26)

	macd := make([]float64, len(data))
	for i := range data {
		if math.IsNaN(ema12[i]) || math.IsNaN(ema26[i]) {
			macd[i] = math.NaN()
		} else {
			macd[i] = ema12[i] - ema26[i]
		}
	}

	signal := calculateEMA(macd, 9)
	return macd, signal
}

// calculateBollingerBands calculates Bollinger Bands
func calculateBollingerBands(data []float64, period int, stdDev float64) ([]float64, []float64, []float64) {
	middle := calculateSMA(data, period)
	upper := make([]float64, len(data))
	lower := make([]float64, len(data))

	for i := range data {
		if math.IsNaN(middle[i]) {
			upper[i] = math.NaN()
			lower[i] = math.NaN()
			continue
		}

		// Calculate standard deviation
		sum := 0.0
		for j := 0; j < period; j++ {
			diff := data[i-j] - middle[i]
			sum += diff * diff
		}
		sd := math.Sqrt(sum / float64(period))

		upper[i] = middle[i] + stdDev*sd
		lower[i] = middle[i] - stdDev*sd
	}

	return upper, middle, lower
}

// calculateATR calculates Average True Range
func calculateATR(highs, lows, closes []float64, period int) []float64 {
	result := make([]float64, len(closes))
	tr := make([]float64, len(closes))

	for i := range closes {
		if i == 0 {
			tr[i] = highs[i] - lows[i]
			result[i] = math.NaN()
			continue
		}

		h_l := highs[i] - lows[i]
		h_pc := math.Abs(highs[i] - closes[i-1])
		l_pc := math.Abs(lows[i] - closes[i-1])

		tr[i] = math.Max(h_l, math.Max(h_pc, l_pc))

		if i < period {
			result[i] = math.NaN()
			continue
		}

		if i == period {
			sum := 0.0
			for j := 1; j <= period; j++ {
				sum += tr[j]
			}
			result[i] = sum / float64(period)
		} else {
			result[i] = (result[i-1]*float64(period-1) + tr[i]) / float64(period)
		}
	}

	return result
}

// calculateADX calculates the Average Directional Index
// calculateADX 计算平均趋势指数（趋势强度）
// ADX < 20: 无趋势，观望 / No trend, wait
// ADX 20-25: 弱趋势 / Weak trend
// ADX > 25: 强趋势，可交易 / Strong trend, tradable
// ADX > 50: 极强趋势，最佳机会 / Very strong trend, best opportunity
func calculateADX(highs, lows, closes []float64, period int) (adx, diPlus, diMinus []float64) {
	n := len(closes)
	adx = make([]float64, n)
	diPlus = make([]float64, n)
	diMinus = make([]float64, n)

	// Calculate True Range and Directional Movement
	// 计算真实波动幅度和趋向变动
	tr := make([]float64, n)
	plusDM := make([]float64, n)
	minusDM := make([]float64, n)

	for i := range closes {
		if i == 0 {
			tr[i] = highs[i] - lows[i]
			plusDM[i] = 0
			minusDM[i] = 0
			adx[i] = math.NaN()
			diPlus[i] = math.NaN()
			diMinus[i] = math.NaN()
			continue
		}

		// True Range
		h_l := highs[i] - lows[i]
		h_pc := math.Abs(highs[i] - closes[i-1])
		l_pc := math.Abs(lows[i] - closes[i-1])
		tr[i] = math.Max(h_l, math.Max(h_pc, l_pc))

		// Directional Movement
		upMove := highs[i] - highs[i-1]
		downMove := lows[i-1] - lows[i]

		if upMove > downMove && upMove > 0 {
			plusDM[i] = upMove
		} else {
			plusDM[i] = 0
		}

		if downMove > upMove && downMove > 0 {
			minusDM[i] = downMove
		} else {
			minusDM[i] = 0
		}

		if i < period {
			adx[i] = math.NaN()
			diPlus[i] = math.NaN()
			diMinus[i] = math.NaN()
		}
	}

	// Smooth True Range and Directional Movements
	// 平滑真实波动幅度和趋向变动
	smoothedTR := make([]float64, n)
	smoothedPlusDM := make([]float64, n)
	smoothedMinusDM := make([]float64, n)

	// Initial smoothing - sum of first period values
	// 初始平滑 - 第一个周期的总和
	for i := 1; i <= period && i < n; i++ {
		smoothedTR[period] += tr[i]
		smoothedPlusDM[period] += plusDM[i]
		smoothedMinusDM[period] += minusDM[i]
	}

	// Subsequent values use exponential smoothing
	// 后续值使用指数平滑
	for i := period + 1; i < n; i++ {
		smoothedTR[i] = smoothedTR[i-1] - (smoothedTR[i-1] / float64(period)) + tr[i]
		smoothedPlusDM[i] = smoothedPlusDM[i-1] - (smoothedPlusDM[i-1] / float64(period)) + plusDM[i]
		smoothedMinusDM[i] = smoothedMinusDM[i-1] - (smoothedMinusDM[i-1] / float64(period)) + minusDM[i]
	}

	// Calculate +DI and -DI
	// 计算 +DI 和 -DI
	dx := make([]float64, n)
	for i := period; i < n; i++ {
		if smoothedTR[i] != 0 {
			diPlus[i] = 100 * smoothedPlusDM[i] / smoothedTR[i]
			diMinus[i] = 100 * smoothedMinusDM[i] / smoothedTR[i]

			// Calculate DX
			diSum := diPlus[i] + diMinus[i]
			if diSum != 0 {
				dx[i] = 100 * math.Abs(diPlus[i]-diMinus[i]) / diSum
			} else {
				dx[i] = 0
			}
		} else {
			diPlus[i] = 0
			diMinus[i] = 0
			dx[i] = 0
		}
	}

	// Calculate ADX (smoothed DX)
	// 计算 ADX（平滑的 DX）
	adxPeriod := period * 2 // ADX period is typically 2x the DI period
	for i := period + adxPeriod - 1; i < n; i++ {
		if i == period+adxPeriod-1 {
			// Initial ADX is average of first period DX values
			sum := 0.0
			for j := period; j < period+adxPeriod; j++ {
				sum += dx[j]
			}
			adx[i] = sum / float64(adxPeriod)
		} else {
			// Smooth ADX
			adx[i] = (adx[i-1]*float64(adxPeriod-1) + dx[i]) / float64(adxPeriod)
		}
	}

	return adx, diPlus, diMinus
}

// calculateVolumeRatio calculates volume ratio compared to average
// calculateVolumeRatio 计算成交量比率（相对于平均值）
// Ratio > 1.5: 放量 / High volume
// Ratio > 2.0: 异常放量 / Exceptionally high volume
func calculateVolumeRatio(volumes []float64, period int) []float64 {
	result := make([]float64, len(volumes))

	for i := range volumes {
		if i < period-1 {
			result[i] = math.NaN()
			continue
		}

		// Calculate average volume for the period
		// 计算周期内的平均成交量
		sum := 0.0
		for j := 0; j < period; j++ {
			sum += volumes[i-j]
		}
		avgVolume := sum / float64(period)

		// Calculate ratio
		// 计算比率
		if avgVolume > 0 {
			result[i] = volumes[i] / avgVolume
		} else {
			result[i] = 1.0
		}
	}

	return result
}

// FormatOHLCVReport generates a formatted report of OHLCV data
func FormatOHLCVReport(symbol string, timeframe string, ohlcvData []OHLCV) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# Crypto data for %s\n", symbol))
	sb.WriteString(fmt.Sprintf("# Timeframe: %s\n", timeframe))
	sb.WriteString(fmt.Sprintf("# Total records: %d\n", len(ohlcvData)))
	sb.WriteString(fmt.Sprintf("# Data retrieved on: %s\n", time.Now().Format("2006-01-02 15:04:05")))
	if len(ohlcvData) > 0 {
		sb.WriteString(fmt.Sprintf("# Latest data: %s\n",
			ohlcvData[len(ohlcvData)-1].Timestamp.Format("2006-01-02 15:04:05")))
	}
	sb.WriteString("\n")

	// Add CSV header
	sb.WriteString("timestamp,open,high,low,close,volume\n")

	// Add data - limit to last 100 candles to avoid context overflow
	startIdx := 0
	if len(ohlcvData) > 100 {
		startIdx = len(ohlcvData) - 100
	}

	for i := startIdx; i < len(ohlcvData); i++ {
		candle := ohlcvData[i]
		sb.WriteString(fmt.Sprintf("%s,%.2f,%.2f,%.2f,%.2f,%.2f\n",
			candle.Timestamp.Format("2006-01-02 15:04:05"),
			candle.Open,
			candle.High,
			candle.Low,
			candle.Close,
			candle.Volume,
		))
	}

	return sb.String()
}

// FormatIndicatorReport generates a formatted report of technical indicators
func FormatIndicatorReport(symbol string, timeframe string, ohlcvData []OHLCV, indicators *TechnicalIndicators) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("## Technical Indicators for %s (timeframe: %s)\n\n", symbol, timeframe))

	if len(ohlcvData) == 0 {
		sb.WriteString("No data available\n")
		return sb.String()
	}

	// Show latest values
	lastIdx := len(ohlcvData) - 1
	latestTime := ohlcvData[lastIdx].Timestamp.Format("2006-01-02 15:04:05")
	latestPrice := ohlcvData[lastIdx].Close

	sb.WriteString(fmt.Sprintf("Latest data point: %s\n", latestTime))
	sb.WriteString(fmt.Sprintf("Current price: $%.2f\n\n", latestPrice))

	// RSI
	if len(indicators.RSI) > lastIdx && !math.IsNaN(indicators.RSI[lastIdx]) {
		sb.WriteString(fmt.Sprintf("RSI(14): %.2f\n", indicators.RSI[lastIdx]))
	}

	// MACD
	if len(indicators.MACD) > lastIdx && !math.IsNaN(indicators.MACD[lastIdx]) {
		sb.WriteString(fmt.Sprintf("MACD: %.2f\n", indicators.MACD[lastIdx]))
	}
	if len(indicators.Signal) > lastIdx && !math.IsNaN(indicators.Signal[lastIdx]) {
		sb.WriteString(fmt.Sprintf("MACD Signal: %.2f\n", indicators.Signal[lastIdx]))
	}

	// Bollinger Bands
	if len(indicators.BB_Upper) > lastIdx && !math.IsNaN(indicators.BB_Upper[lastIdx]) {
		sb.WriteString(fmt.Sprintf("\nBollinger Bands:\n"))
		sb.WriteString(fmt.Sprintf("  Upper: $%.2f\n", indicators.BB_Upper[lastIdx]))
		sb.WriteString(fmt.Sprintf("  Middle: $%.2f\n", indicators.BB_Middle[lastIdx]))
		sb.WriteString(fmt.Sprintf("  Lower: $%.2f\n", indicators.BB_Lower[lastIdx]))
	}

	// Moving Averages
	sb.WriteString(fmt.Sprintf("\nMoving Averages:\n"))
	if len(indicators.SMA_20) > lastIdx && !math.IsNaN(indicators.SMA_20[lastIdx]) {
		sb.WriteString(fmt.Sprintf("  SMA(20): $%.2f\n", indicators.SMA_20[lastIdx]))
	}
	if len(indicators.SMA_50) > lastIdx && !math.IsNaN(indicators.SMA_50[lastIdx]) {
		sb.WriteString(fmt.Sprintf("  SMA(50): $%.2f\n", indicators.SMA_50[lastIdx]))
	}
	if len(indicators.SMA_200) > lastIdx && !math.IsNaN(indicators.SMA_200[lastIdx]) {
		sb.WriteString(fmt.Sprintf("  SMA(200): $%.2f\n", indicators.SMA_200[lastIdx]))
	}
	if len(indicators.EMA_12) > lastIdx && !math.IsNaN(indicators.EMA_12[lastIdx]) {
		sb.WriteString(fmt.Sprintf("  EMA(12): $%.2f\n", indicators.EMA_12[lastIdx]))
	}
	if len(indicators.EMA_26) > lastIdx && !math.IsNaN(indicators.EMA_26[lastIdx]) {
		sb.WriteString(fmt.Sprintf("  EMA(26): $%.2f\n", indicators.EMA_26[lastIdx]))
	}

	// ATR
	if len(indicators.ATR) > lastIdx && !math.IsNaN(indicators.ATR[lastIdx]) {
		sb.WriteString(fmt.Sprintf("\nATR(14): $%.2f\n", indicators.ATR[lastIdx]))
		atrPercent := (indicators.ATR[lastIdx] / latestPrice) * 100
		sb.WriteString(fmt.Sprintf("ATR %%: %.2f%%\n", atrPercent))
	}

	// ADX and Directional Indicators
	// ADX 和趋向指标
	if len(indicators.ADX) > lastIdx && !math.IsNaN(indicators.ADX[lastIdx]) {
		sb.WriteString(fmt.Sprintf("\nTrend Strength (ADX):\n"))
		sb.WriteString(fmt.Sprintf("  ADX(14): %.2f", indicators.ADX[lastIdx]))

		// Interpret ADX value
		// 解释 ADX 值
		adxValue := indicators.ADX[lastIdx]
		if adxValue < 20 {
			sb.WriteString(" (No trend - 观望)")
		} else if adxValue < 25 {
			sb.WriteString(" (Weak trend - 弱趋势)")
		} else if adxValue < 50 {
			sb.WriteString(" (Strong trend - 强趋势 ✓)")
		} else {
			sb.WriteString(" (Very strong trend - 极强趋势 ✓✓)")
		}
		sb.WriteString("\n")

		if len(indicators.DI_Plus) > lastIdx && !math.IsNaN(indicators.DI_Plus[lastIdx]) {
			sb.WriteString(fmt.Sprintf("  +DI: %.2f\n", indicators.DI_Plus[lastIdx]))
		}
		if len(indicators.DI_Minus) > lastIdx && !math.IsNaN(indicators.DI_Minus[lastIdx]) {
			sb.WriteString(fmt.Sprintf("  -DI: %.2f\n", indicators.DI_Minus[lastIdx]))
		}

		// Determine trend direction
		// 判断趋势方向
		if len(indicators.DI_Plus) > lastIdx && len(indicators.DI_Minus) > lastIdx &&
			!math.IsNaN(indicators.DI_Plus[lastIdx]) && !math.IsNaN(indicators.DI_Minus[lastIdx]) {
			if indicators.DI_Plus[lastIdx] > indicators.DI_Minus[lastIdx] {
				sb.WriteString("  Direction: Bullish (上升趋势)\n")
			} else {
				sb.WriteString("  Direction: Bearish (下降趋势)\n")
			}
		}
	}

	// Volume Analysis
	// 成交量分析
	sb.WriteString(fmt.Sprintf("\nVolume Analysis:\n"))
	sb.WriteString(fmt.Sprintf("  Latest Volume: %.2f\n", ohlcvData[lastIdx].Volume))

	if len(indicators.VolumeRatio) > lastIdx && !math.IsNaN(indicators.VolumeRatio[lastIdx]) {
		sb.WriteString(fmt.Sprintf("  Volume Ratio: %.2fx", indicators.VolumeRatio[lastIdx]))

		// Interpret volume ratio
		// 解释成交量比率
		volumeRatio := indicators.VolumeRatio[lastIdx]
		if volumeRatio > 2.0 {
			sb.WriteString(" (异常放量 - Exceptionally high ✓✓)")
		} else if volumeRatio > 1.5 {
			sb.WriteString(" (放量 - High volume ✓)")
		} else if volumeRatio < 0.5 {
			sb.WriteString(" (缩量 - Low volume)")
		} else {
			sb.WriteString(" (正常 - Normal)")
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// GetFundingRate fetches the current funding rate
func (m *MarketData) GetFundingRate(ctx context.Context, symbol string) (float64, error) {
	rates, err := m.client.NewFundingRateService().
		Symbol(symbol).
		Limit(1).
		Do(ctx)

	if err != nil {
		return 0, fmt.Errorf("failed to fetch funding rate: %w", err)
	}

	if len(rates) == 0 {
		return 0, fmt.Errorf("no funding rate data available")
	}

	fundingRate, _ := strconv.ParseFloat(rates[0].FundingRate, 64)
	return fundingRate, nil
}

// GetOrderBook fetches the order book depth
func (m *MarketData) GetOrderBook(ctx context.Context, symbol string, limit int) (map[string]interface{}, error) {
	depth, err := m.client.NewDepthService().
		Symbol(symbol).
		Limit(limit).
		Do(ctx)

	if err != nil {
		return nil, fmt.Errorf("failed to fetch order book: %w", err)
	}

	// Calculate bid/ask strength
	var bidVolume, askVolume float64
	for _, bid := range depth.Bids {
		qty, _ := strconv.ParseFloat(bid.Quantity, 64)
		bidVolume += qty
	}
	for _, ask := range depth.Asks {
		qty, _ := strconv.ParseFloat(ask.Quantity, 64)
		askVolume += qty
	}

	result := map[string]interface{}{
		"bids":          depth.Bids,
		"asks":          depth.Asks,
		"bid_volume":    bidVolume,
		"ask_volume":    askVolume,
		"bid_ask_ratio": bidVolume / (askVolume + 0.0001),
	}

	return result, nil
}

// Get24HrStats fetches 24-hour statistics
func (m *MarketData) Get24HrStats(ctx context.Context, symbol string) (map[string]string, error) {
	stats, err := m.client.NewListPriceChangeStatsService().
		Symbol(symbol).
		Do(ctx)

	if err != nil {
		return nil, fmt.Errorf("failed to fetch 24hr stats: %w", err)
	}

	if len(stats) == 0 {
		return nil, fmt.Errorf("no stats data available")
	}

	result := map[string]string{
		"price_change":         stats[0].PriceChange,
		"price_change_percent": stats[0].PriceChangePercent,
		"high_price":           stats[0].HighPrice,
		"low_price":            stats[0].LowPrice,
		"volume":               stats[0].Volume,
		"quote_volume":         stats[0].QuoteVolume,
	}

	return result, nil
}

// Helper functions
func convertTimeframe(tf string) string {
	// Convert from format like "1h", "15m", "1d" to Binance interval format
	switch tf {
	case "1m":
		return "1m"
	case "5m":
		return "5m"
	case "15m":
		return "15m"
	case "1h":
		return "1h"
	case "4h":
		return "4h"
	case "1d":
		return "1d"
	default:
		return "1h"
	}
}
