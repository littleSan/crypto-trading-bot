"""
加密货币数据获取模块 - 使用 CCXT 从币安获取数据
"""
from typing import Annotated
from datetime import datetime, timedelta
import ccxt
import pandas as pd
import os
from .config import get_config


def get_exchange(require_auth=False):
    """获取配置的交易所实例"""
    config = get_config()
    
    proxy = config.get('binance_proxy', None)
    
    exchange_config = {
        'options': {'defaultType': 'future'},
        'enableRateLimit': True,
    }
    
    if require_auth:
        api_key = os.getenv('BINANCE_API_KEY', '')
        secret = os.getenv('BINANCE_SECRET', '')
        
        if api_key and secret:
            exchange_config['apiKey'] = api_key
            exchange_config['secret'] = secret
    
    if proxy:
        exchange_config['proxies'] = {
            'http': proxy,
            'https': proxy,
        }
    
    exchange = ccxt.binance(exchange_config)
    return exchange


def get_crypto_ohlcv(
    symbol: Annotated[str, "交易对符号，如 BTC/USDT"],
    start_date: Annotated[str, "开始日期 yyyy-mm-dd"],
    end_date: Annotated[str, "结束日期 yyyy-mm-dd"],
    timeframe: Annotated[str, "时间周期，如 1h, 15m, 1d"] = "1h"
):
    """获取加密货币的 OHLCV 数据"""
    try:
        exchange = get_exchange(require_auth=False)
        
        start_dt = datetime.strptime(start_date, "%Y-%m-%d")
        # 结束时间使用当前时间，而不是日期的 00:00
        end_dt = datetime.now()
        
        start_ts = int(start_dt.timestamp() * 1000)
        end_ts = int(end_dt.timestamp() * 1000)
        
        all_ohlcv = []
        current_ts = start_ts
        
        while current_ts < end_ts:
            ohlcv = exchange.fetch_ohlcv(symbol, timeframe=timeframe, since=current_ts, limit=1000)
            
            if not ohlcv:
                break
            
            all_ohlcv.extend(ohlcv)
            current_ts = ohlcv[-1][0] + 1
            
            if ohlcv[-1][0] >= end_ts:
                break
        
        filtered_ohlcv = [candle for candle in all_ohlcv if start_ts <= candle[0] < end_ts]
        
        if not filtered_ohlcv:
            return f"No data found for {symbol} between {start_date} and {end_date}"
        
        df = pd.DataFrame(filtered_ohlcv, columns=['timestamp', 'open', 'high', 'low', 'close', 'volume'])
        # 币安返回的是 UTC 时间，转换为本地时间（UTC+8）
        df['timestamp'] = pd.to_datetime(df['timestamp'], unit='ms', utc=True).dt.tz_convert('Asia/Shanghai').dt.tz_localize(None)
        
        csv_string = df.to_csv(index=False)
        
        header = f"# Crypto data for {symbol} from {start_date} to {end_date}\n"
        header += f"# Timeframe: {timeframe}\n"
        header += f"# Total records: {len(df)}\n"
        header += f"# Data retrieved on: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')} (本地时间)\n"
        if len(df) > 0:
            header += f"# Latest data: {df['timestamp'].iloc[-1]} (本地时间)\n"
        header += "\n"
        
        return header + csv_string
        
    except Exception as e:
        return f"Error fetching crypto data: {str(e)}"


def get_crypto_indicators(
    symbol: Annotated[str, "交易对符号，如 BTC/USDT"],
    indicator: Annotated[str, "技术指标名称"],
    curr_date: Annotated[str, "当前交易日期 yyyy-mm-dd"],
    look_back_days: Annotated[int, "回看天数"] = 30,
    timeframe: Annotated[str, "时间周期"] = "1h"
):
    """
    计算加密货币的技术指标
    
    参考 y_finance.py 的成功模式
    """
    try:
        from stockstats import wrap
        
        # 获取数据 - 使用当前时间而不是日期字符串
        # 这样可以获取到最新的数据（包括当天的最新K线）
        curr_dt = datetime.now()  # 使用当前时间
        end_date = curr_dt.strftime("%Y-%m-%d")
        start_dt = curr_dt - timedelta(days=look_back_days + 100)
        start_date = start_dt.strftime("%Y-%m-%d")
        
        csv_data = get_crypto_ohlcv(symbol, start_date, end_date, timeframe)
        
        if csv_data.startswith("Error") or csv_data.startswith("No data"):
            return csv_data
        
        # 解析 CSV
        lines = [l for l in csv_data.split('\n') if not l.startswith('#') and l.strip()]
        csv_content = '\n'.join(lines)
        data = pd.read_csv(pd.io.common.StringIO(csv_content))
        
        # 使用首字母大写的 Date（参考 yfinance 模式）
        data = data.rename(columns={'timestamp': 'Date'})
        data['Date'] = pd.to_datetime(data['Date'])
        
        # 重命名其他列为首字母大写
        data = data.rename(columns={
            'open': 'Open',
            'high': 'High',
            'low': 'Low',
            'close': 'Close',
            'volume': 'Volume'
        })
        
        # 使用 stockstats（参考 y_finance._get_stock_stats_bulk）
        df = wrap(data)
        df[indicator]  # 触发计算
        
        # 格式化 Date 为字符串（stockstats 可能修改了它）
        if 'Date' in df.columns:
            df["Date"] = pd.to_datetime(df["Date"]).dt.strftime("%Y-%m-%d %H:%M")
        elif pd.api.types.is_datetime64_any_dtype(df.index):
            # Date 在索引中
            df = df.reset_index()
            if df.columns[0] != 'Date':
                df = df.rename(columns={df.columns[0]: 'Date'})
            df["Date"] = pd.to_datetime(df["Date"]).dt.strftime("%Y-%m-%d %H:%M")
        
        # 创建字典映射
        result_dict = {}
        for _, row in df.iterrows():
            date_str = row["Date"]
            indicator_value = row[indicator]
            
            if pd.isna(indicator_value):
                result_dict[date_str] = "N/A"
            else:
                result_dict[date_str] = str(indicator_value)
        
        # 过滤日期范围并格式化输出
        # 使用当前实时时间作为结束时间
        curr_date_dt = datetime.now()
        start_display_dt = curr_date_dt - timedelta(days=look_back_days)
        
        # 收集符合日期范围的数据
        filtered_data = []
        for date_str, value_str in result_dict.items():
            try:
                date_obj = datetime.strptime(date_str, "%Y-%m-%d %H:%M")
                if start_display_dt <= date_obj <= curr_date_dt:
                    filtered_data.append((date_obj, date_str, value_str))
            except:
                pass
        
        # 按日期排序
        filtered_data.sort(key=lambda x: x[0])
        
        # 限制返回数量，避免上下文溢出
        # 对于小时级别：最多返回最近 24 条（1天）
        # 对于其他级别：根据 timeframe 调整
        max_records = 24 if timeframe == '1h' else 48
        
        if len(filtered_data) > max_records:
            # 只保留最近的 N 条数据
            filtered_data = filtered_data[-max_records:]
        
        result_str = f"## {indicator} values for {symbol} (timeframe: {timeframe}):\n\n"
        result_str += f"Showing last {len(filtered_data)} data points:\n\n"
        
        for _, date_str, value_str in filtered_data:
            result_str += f"{date_str}: {value_str}\n"
        
        return result_str
        
    except Exception as e:
        import traceback
        return f"Error: {str(e)}\n\n{traceback.format_exc()}"


def get_funding_rate(symbol: Annotated[str, "交易对符号，如 BTC/USDT"]):
    """获取永续合约的资金费率"""
    try:
        exchange = get_exchange(require_auth=False)
        funding_history = exchange.fetch_funding_rate_history(symbol, limit=10)
        
        if not funding_history:
            return f"No funding rate data available for {symbol}"
        
        result_str = f"## Funding Rate History for {symbol}:\n\n"
        result_str += "Recent funding rates (positive = longs pay shorts, negative = shorts pay longs):\n\n"
        
        for entry in funding_history[-10:]:
            timestamp = datetime.fromtimestamp(entry['timestamp'] / 1000)
            rate = entry['fundingRate'] * 100
            result_str += f"{timestamp.strftime('%Y-%m-%d %H:%M')}: {rate:.4f}%\n"
        
        avg_rate = sum([e['fundingRate'] for e in funding_history[-10:]]) / len(funding_history[-10:])
        result_str += f"\nAverage funding rate (last 10): {avg_rate * 100:.4f}%\n"
        
        if avg_rate > 0.01:
            result_str += "Market sentiment: BULLISH (longs dominating)\n"
        elif avg_rate < -0.01:
            result_str += "Market sentiment: BEARISH (shorts dominating)\n"
        else:
            result_str += "Market sentiment: NEUTRAL\n"
        
        return result_str
    except Exception as e:
        return f"Error fetching funding rate: {str(e)}"


def get_order_book(symbol: Annotated[str, "交易对符号，如 BTC/USDT"], limit: Annotated[int, "深度档位数量"] = 20):
    """获取订单簿数据"""
    try:
        exchange = get_exchange(require_auth=False)
        orderbook = exchange.fetch_order_book(symbol, limit=limit)
        
        bids = orderbook['bids'][:limit]
        asks = orderbook['asks'][:limit]
        
        result_str = f"## Order Book for {symbol}:\n\n"
        result_str += f"Timestamp: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}\n\n"
        
        total_bid_volume = sum([bid[1] for bid in bids])
        total_ask_volume = sum([ask[1] for ask in asks])
        
        result_str += f"Total bid volume (top {limit}): {total_bid_volume:.4f}\n"
        result_str += f"Total ask volume (top {limit}): {total_ask_volume:.4f}\n"
        result_str += f"Bid/Ask ratio: {total_bid_volume/total_ask_volume:.4f}\n\n"
        
        result_str += "Top 5 Bids:\n"
        for i, bid in enumerate(bids[:5]):
            result_str += f"  {i+1}. Price: {bid[0]:.2f}, Volume: {bid[1]:.4f}\n"
        
        result_str += "\nTop 5 Asks:\n"
        for i, ask in enumerate(asks[:5]):
            result_str += f"  {i+1}. Price: {ask[0]:.2f}, Volume: {ask[1]:.4f}\n"
        
        if total_bid_volume > total_ask_volume * 1.2:
            result_str += "\nMarket pressure: BUYING PRESSURE (strong support)\n"
        elif total_ask_volume > total_bid_volume * 1.2:
            result_str += "\nMarket pressure: SELLING PRESSURE (strong resistance)\n"
        else:
            result_str += "\nMarket pressure: BALANCED\n"
        
        return result_str
    except Exception as e:
        return f"Error fetching order book: {str(e)}"


def get_market_info(symbol: Annotated[str, "交易对符号，如 BTC/USDT"]):
    """获取市场基本信息"""
    try:
        exchange = get_exchange(require_auth=False)
        ticker = exchange.fetch_ticker(symbol)
        
        result_str = f"## Market Info for {symbol}:\n\n"
        result_str += f"Last price: ${ticker['last']:,.2f}\n"
        result_str += f"24h high: ${ticker['high']:,.2f}\n"
        result_str += f"24h low: ${ticker['low']:,.2f}\n"
        result_str += f"24h volume: {ticker['baseVolume']:,.2f} {symbol.split('/')[0]}\n"
        result_str += f"24h change: {ticker['percentage']:.2f}%\n"
        
        if ticker.get('bid'):
            result_str += f"Best bid: ${ticker['bid']:,.2f}\n"
        if ticker.get('ask'):
            result_str += f"Best ask: ${ticker['ask']:,.2f}\n"
        
        return result_str
    except Exception as e:
        return f"Error fetching market info: {str(e)}"
