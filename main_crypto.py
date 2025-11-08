"""
加密货币交易主程序 - 使用多智能体框架交易 BTC

所有配置都在 .env 文件中设置，无需在代码中修改
如需调整配置，请编辑项目根目录的 .env 文件
"""
from tradingagents.graph.crypto_trading_graph import CryptoTradingAgentsGraph
from tradingagents.crypto_config import get_crypto_config
from dotenv import load_dotenv
from datetime import datetime

# 加载环境变量（从 .env 文件）
load_dotenv()

def main():
    """主函数 - 所有配置从 .env 读取"""
    
    # 从 .env 文件获取所有配置
    config = get_crypto_config()
    
    # 打印配置信息
    print("=" * 80)
    print("🚀 加密货币多智能体交易系统启动")
    print("=" * 80)
    print(f"LLM 提供商: {config['llm_provider']}")
    print(f"深度思考模型: {config['deep_think_llm']}")
    print(f"快速思考模型: {config['quick_think_llm']}")
    print(f"交易对: {config['crypto_symbol']}")
    print(f"K线周期: {config['crypto_timeframe']}")
    print(f"杠杆倍数: {config['binance_leverage']}x")
    print(f"测试模式: {'是 ✅' if config['binance_test_mode'] else '否 ⚠️（实盘）'}")
    print(f"辩论轮数: {config['max_debate_rounds']}")
    print(f"风险讨论轮数: {config['max_risk_discuss_rounds']}")
    print(f"选择的分析师: {', '.join(config['selected_analysts'])}")
    if config['binance_proxy']:
        print(f"代理设置: {config['binance_proxy']}")
    print("=" * 80)
    
    # 初始化交易图
    ta = CryptoTradingAgentsGraph(
        selected_analysts=config['selected_analysts'],
        debug=config['debug_mode'],
        config=config,
        auto_execute=config['auto_execute']
    )
    
    # 获取当前日期
    trade_date = datetime.now().strftime("%Y-%m-%d")
    
    print(f"\n📊 开始分析 {config['crypto_symbol']} (日期: {trade_date})")
    print("=" * 80)
    
    # 执行分析（forward propagate）
    final_state, decision = ta.propagate(config['crypto_symbol'], trade_date)
    
    print("\n" + "=" * 80)
    print("📈 最终交易决策")
    print("=" * 80)
    print(decision)
    print("=" * 80)
    
    # 查看当前持仓（如果有）
    current_position = ta.get_current_position()
    if current_position:
        print("\n💼 当前持仓:")
        print(f"  方向: {current_position['side']}")
        print(f"  数量: {current_position['size']}")
        print(f"  开仓价: {current_position['entry_price']}")
        print(f"  未实现盈亏: {current_position['unrealized_pnl']} USDT")
    else:
        print("\n💼 当前无持仓")
    
    # 可选：基于盈亏反思和学习
    # 如果你有实际的盈亏数据，可以让系统学习
    # returns = 1000  # 盈利1000 USDT
    # ta.reflect_and_remember(returns)
    
    print("\n✅ 分析完成！")
    print(f"详细日志已保存到: crypto_results/{config['crypto_symbol']}/")


if __name__ == "__main__":
    main()

