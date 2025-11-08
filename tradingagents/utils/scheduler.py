"""
交易调度器 - 根据K线周期智能调度运行时间
"""
from datetime import datetime, timedelta
import time


class TradingScheduler:
    """交易调度器"""
    
    # K线周期映射（转换为分钟数）
    TIMEFRAME_MINUTES = {
        '1m': 1,
        '3m': 3,
        '5m': 5,
        '15m': 15,
        '30m': 30,
        '1h': 60,
        '2h': 120,
        '4h': 240,
        '6h': 360,
        '12h': 720,
        '1d': 1440,
    }
    
    @classmethod
    def parse_timeframe(cls, timeframe: str) -> int:
        """
        解析时间周期，返回分钟数
        
        Args:
            timeframe: K线周期，如 '15m', '1h'
            
        Returns:
            分钟数
        """
        if timeframe in cls.TIMEFRAME_MINUTES:
            return cls.TIMEFRAME_MINUTES[timeframe]
        
        # 尝试解析其他格式
        if timeframe.endswith('m'):
            return int(timeframe[:-1])
        elif timeframe.endswith('h'):
            return int(timeframe[:-1]) * 60
        elif timeframe.endswith('d'):
            return int(timeframe[:-1]) * 1440
        else:
            raise ValueError(f"不支持的时间周期格式: {timeframe}")
    
    @classmethod
    def get_next_timeframe_time(cls, timeframe: str) -> datetime:
        """
        获取下一个K线周期的开始时间
        
        Args:
            timeframe: K线周期
            
        Returns:
            下一个周期的开始时间
        """
        minutes = cls.parse_timeframe(timeframe)
        now = datetime.now()
        
        # 计算当前时间是第几个周期
        current_minute = now.hour * 60 + now.minute
        
        # 计算下一个周期点
        next_period = ((current_minute // minutes) + 1) * minutes
        
        # 处理跨天的情况
        if next_period >= 1440:  # 24小时 = 1440分钟
            next_day = now.replace(hour=0, minute=0, second=0, microsecond=0) + timedelta(days=1)
            next_period_minutes = next_period - 1440
            next_time = next_day + timedelta(minutes=next_period_minutes)
        else:
            next_time = now.replace(hour=0, minute=0, second=0, microsecond=0) + timedelta(minutes=next_period)
        
        return next_time
    
    @classmethod
    def wait_for_next_timeframe(cls, timeframe: str, verbose: bool = True):
        """
        等待到下一个K线周期的开始时间
        
        Args:
            timeframe: K线周期
            verbose: 是否显示等待信息
        """
        next_time = cls.get_next_timeframe_time(timeframe)
        now = datetime.now()
        wait_seconds = (next_time - now).total_seconds()
        
        if verbose:
            print(f"⏰ 当前时间: {now.strftime('%Y-%m-%d %H:%M:%S')}")
            print(f"⏳ 下一个 {timeframe} K线周期: {next_time.strftime('%Y-%m-%d %H:%M:%S')}")
            print(f"⌛ 需要等待: {int(wait_seconds // 60)} 分 {int(wait_seconds % 60)} 秒")
            print()
        
        if wait_seconds > 0:
            # 倒计时显示
            if verbose:
                import sys
                while wait_seconds > 0:
                    mins = int(wait_seconds // 60)
                    secs = int(wait_seconds % 60)
                    sys.stdout.write(f"\r⏳ 倒计时: {mins:02d}:{secs:02d} ")
                    sys.stdout.flush()
                    time.sleep(1)
                    wait_seconds -= 1
                print("\n")
            else:
                time.sleep(wait_seconds)
    
    @classmethod
    def is_on_timeframe(cls, timeframe: str) -> bool:
        """
        检查当前时间是否刚好在K线周期点上
        
        Args:
            timeframe: K线周期
            
        Returns:
            是否在周期点上（允许60秒误差）
        """
        minutes = cls.parse_timeframe(timeframe)
        now = datetime.now()
        current_minute = now.hour * 60 + now.minute
        
        # 检查是否是周期的整数倍（允许1分钟误差）
        return current_minute % minutes == 0 and now.second < 60
    
    @classmethod
    def get_aligned_intervals(cls, timeframe: str) -> list:
        """
        获取一天中所有对齐的时间点
        
        Args:
            timeframe: K线周期
            
        Returns:
            时间点列表，如 ['00:00', '00:15', '00:30', ...]
        """
        minutes = cls.parse_timeframe(timeframe)
        intervals = []
        
        total_minutes = 0
        while total_minutes < 1440:  # 24小时
            hour = total_minutes // 60
            minute = total_minutes % 60
            intervals.append(f"{hour:02d}:{minute:02d}")
            total_minutes += minutes
        
        return intervals


if __name__ == "__main__":
    # 测试
    print("="*80)
    print("交易调度器测试")
    print("="*80)
    
    timeframes = ['15m', '1h', '4h']
    
    for tf in timeframes:
        print(f"\n周期: {tf}")
        print(f"  分钟数: {TradingScheduler.parse_timeframe(tf)}")
        print(f"  下一个周期: {TradingScheduler.get_next_timeframe_time(tf)}")
        print(f"  是否在周期点: {TradingScheduler.is_on_timeframe(tf)}")
        intervals = TradingScheduler.get_aligned_intervals(tf)
        print(f"  一天的运行时间点: {intervals[:5]}... (共 {len(intervals)} 个)")

