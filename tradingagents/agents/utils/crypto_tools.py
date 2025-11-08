"""
加密货币专属工具 - 用于加密货币分析师
"""
from langchain_core.tools import tool
from typing import Annotated
from tradingagents.dataflows.interface import route_tool_call
from tradingagents.dataflows.config import get_config


@tool
def get_crypto_funding_rate(
    symbol: Annotated[str, "交易对符号，如 BTC/USDT"]
):
    """
    获取永续合约的资金费率历史
    
    资金费率是加密货币期货特有的指标，反映市场做多/做空情绪：
    - 正费率：多头支付空头，市场看多
    - 负费率：空头支付多头，市场看空
    - 持续高资金费率可能预示回调
    
    Args:
        symbol: 交易对符号，如 BTC/USDT
    
    Returns:
        资金费率历史数据和市场情绪分析
    """
    return route_tool_call("get_crypto_funding_rate", symbol=symbol)


@tool
def get_crypto_order_book(
    symbol: Annotated[str, "交易对符号，如 BTC/USDT"],
    limit: Annotated[int, "深度档位数量，默认20"] = 20
):
    """
    获取订单簿数据（买卖盘深度）
    
    订单簿显示当前市场的买卖挂单情况：
    - 买盘(Bids)：显示支撑位
    - 卖盘(Asks)：显示阻力位
    - 买卖比例：反映市场压力方向
    
    Args:
        symbol: 交易对符号，如 BTC/USDT
        limit: 要获取的档位数量
    
    Returns:
        订单簿数据和市场压力分析
    """
    return route_tool_call("get_crypto_order_book", symbol=symbol, limit=limit)


@tool
def get_crypto_market_info(
    symbol: Annotated[str, "交易对符号，如 BTC/USDT"]
):
    """
    获取24小时市场统计信息
    
    包括：
    - 最新价格
    - 24小时最高/最低价
    - 24小时成交量
    - 24小时涨跌幅
    - 最佳买卖价
    
    Args:
        symbol: 交易对符号，如 BTC/USDT
    
    Returns:
        24小时市场统计数据
    """
    return route_tool_call("get_crypto_market_info", symbol=symbol)


@tool
def get_crypto_data(
    symbol: Annotated[str, "交易对符号，如 BTC/USDT"],
    start_date: Annotated[str, "开始日期 yyyy-mm-dd"],
    end_date: Annotated[str, "结束日期 yyyy-mm-dd"],
    timeframe: Annotated[str, "时间周期，如 1h, 15m, 1d"] = "1h"
):
    """
    获取加密货币的历史K线数据（OHLCV）
    
    Args:
        symbol: 交易对符号，如 BTC/USDT
        start_date: 开始日期
        end_date: 结束日期
        timeframe: K线周期（默认从配置文件读取）
    
    Returns:
        CSV格式的OHLCV数据
    """
    from datetime import datetime, timedelta
    
    config = get_config()
    
    # 如果使用默认值，从配置中读取用户设置的 K 线周期
    if timeframe == "1h":
        timeframe = config.get("crypto_timeframe", "1h")
    
    # 智能限制数据范围，避免 context 超限
    try:
        end_dt = datetime.strptime(end_date, "%Y-%m-%d")
        start_dt = datetime.strptime(start_date, "%Y-%m-%d")
        requested_days = (end_dt - start_dt).days
        
        # 获取推荐的回看天数
        lookback_days = config.get("crypto_lookback_days", None)
        if lookback_days is None:
            lookback_days = _get_recommended_lookback_days(timeframe)
        
        # 如果请求的天数超过推荐值，自动调整 start_date
        if requested_days > lookback_days:
            start_dt = end_dt - timedelta(days=lookback_days)
            start_date = start_dt.strftime("%Y-%m-%d")
            # 可选：记录日志
            # print(f"ℹ️  数据范围已优化: {requested_days}天 → {lookback_days}天 (避免 context 超限)")
    except Exception:
        # 如果日期解析失败，使用原始值
        pass
    
    return route_tool_call(
        "get_crypto_data",
        symbol=symbol,
        start_date=start_date,
        end_date=end_date,
        timeframe=timeframe
    )


@tool  
def get_crypto_indicators(
    symbol: Annotated[str, "交易对符号，如 BTC/USDT"],
    indicator: Annotated[str, "技术指标名称"],
    curr_date: Annotated[str, "当前交易日期 yyyy-mm-dd"],
    look_back_days: Annotated[int, "回看天数"] = 30,
    timeframe: Annotated[str, "时间周期"] = "1h"
):
    """
    计算加密货币的技术指标
    
    支持的指标包括：
    - 移动平均线：close_50_sma, close_200_sma, close_10_ema
    - MACD相关：macd, macds, macdh
    - 动量指标：rsi
    - 波动性指标：boll, boll_ub, boll_lb, atr
    - 成交量指标：vwma, mfi
    
    Args:
        symbol: 交易对符号
        indicator: 技术指标名称
        curr_date: 当前日期
        look_back_days: 回看天数（默认从配置文件读取，并根据时间周期智能调整）
        timeframe: 时间周期（默认从配置文件读取）
    
    Returns:
        指标计算结果
    """
    config = get_config()
    
    # 如果使用默认值，从配置中读取用户设置的 K 线周期
    if timeframe == "1h":
        timeframe = config.get("crypto_timeframe", "1h")
    
    # 如果使用默认值30，则从配置读取或根据 timeframe 智能调整
    if look_back_days == 30:
        # 优先使用用户配置
        look_back_days = config.get("crypto_lookback_days", None)
        
        # 如果没有配置，根据 timeframe 智能推荐
        if look_back_days is None:
            look_back_days = _get_recommended_lookback_days(timeframe)
    
    return route_tool_call(
        "get_crypto_indicators",
        symbol=symbol,
        indicator=indicator,
        curr_date=curr_date,
        look_back_days=look_back_days,
        timeframe=timeframe
    )


def _get_recommended_lookback_days(timeframe: str) -> int:
    """
    根据 K 线周期推荐合适的回看天数
    
    较小的周期需要更少的天数以避免数据量过大：
    - 1m/5m: 2-3 天足够（2880-4320 根 K 线）
    - 15m: 3-5 天（288-480 根 K 线）
    - 30m: 5-7 天（240-336 根 K 线）
    - 1h: 7-10 天（168-240 根 K 线）
    - 4h: 14-20 天（84-120 根 K 线）
    - 1d: 30-60 天（30-60 根 K 线）
    
    Args:
        timeframe: K 线周期字符串
        
    Returns:
        推荐的回看天数
    """
    timeframe_lower = timeframe.lower()
    
    # 分钟级别
    if timeframe_lower.endswith('m'):
        minutes = int(timeframe_lower[:-1])
        if minutes <= 5:
            return 3  # 1m/5m → 3天
        elif minutes <= 15:
            return 5  # 15m → 5天
        elif minutes <= 30:
            return 7  # 30m → 7天
        else:
            return 10  # 其他分钟 → 10天
    
    # 小时级别
    elif timeframe_lower.endswith('h'):
        hours = int(timeframe_lower[:-1])
        if hours == 1:
            return 10  # 1h → 10天
        elif hours <= 4:
            return 15  # 2h/4h → 15天
        else:
            return 20  # 其他小时 → 20天
    
    # 日级别
    elif timeframe_lower.endswith('d'):
        days = int(timeframe_lower[:-1])
        if days == 1:
            return 60  # 1d → 60天
        else:
            return 90  # 其他天 → 90天
    
    # 周级别
    elif timeframe_lower.endswith('w'):
        return 180  # 1w → 180天
    
    # 默认
    return 10

