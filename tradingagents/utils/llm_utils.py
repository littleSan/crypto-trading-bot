"""
LLM 工具函数 - 提供重试机制和错误处理
"""
import time
from functools import wraps
from typing import Any, Callable
from tradingagents.utils.logger import ColorLogger


def llm_retry(max_retries: int = 3, base_delay: float = 5.0, backoff_factor: float = 2.0):
    """
    LLM 调用重试装饰器
    
    Args:
        max_retries: 最大重试次数
        base_delay: 初始延迟时间（秒）
        backoff_factor: 退避系数（每次重试延迟时间翻倍）
    """
    def decorator(func: Callable) -> Callable:
        @wraps(func)
        def wrapper(*args, **kwargs) -> Any:
            last_exception = None
            
            for attempt in range(max_retries + 1):
                try:
                    return func(*args, **kwargs)
                except Exception as e:
                    last_exception = e
                    error_str = str(e).lower()
                    
                    # 判断是否是可重试的错误
                    is_retryable = (
                        '404' in error_str or
                        'not found' in error_str or
                        '500' in error_str or
                        '502' in error_str or
                        '503' in error_str or
                        '504' in error_str or
                        'timeout' in error_str or
                        'connection' in error_str or
                        'rate limit' in error_str or
                        'too many requests' in error_str
                    )
                    
                    if not is_retryable:
                        # 不可重试的错误，直接抛出
                        raise
                    
                    if attempt < max_retries:
                        delay = base_delay * (backoff_factor ** attempt)
                        ColorLogger.warning(
                            f"LLM 调用失败 (尝试 {attempt + 1}/{max_retries + 1}): {str(e)[:100]}"
                        )
                        ColorLogger.info(f"等待 {delay:.1f} 秒后重试...")
                        time.sleep(delay)
                    else:
                        # 最后一次重试也失败了
                        ColorLogger.error(
                            f"LLM 调用失败，已达最大重试次数 ({max_retries + 1})"
                        )
                        raise last_exception
            
            # 理论上不会到这里
            raise last_exception
        
        return wrapper
    return decorator


def check_llm_health(llm, test_prompt: str = "Hello, are you available?") -> bool:
    """
    检查 LLM 是否可用
    
    Args:
        llm: LLM 实例
        test_prompt: 测试提示词
        
    Returns:
        True if LLM is healthy, False otherwise
    """
    try:
        ColorLogger.info("正在检查 LLM 连接...")
        response = llm.invoke(test_prompt)
        ColorLogger.success("LLM 连接正常 ✓")
        return True
    except Exception as e:
        ColorLogger.error(f"LLM 连接失败: {str(e)[:200]}")
        return False

