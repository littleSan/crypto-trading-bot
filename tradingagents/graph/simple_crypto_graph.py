"""
简化版加密货币交易图 - 只包含核心 3 个 Agent
市场分析师 → 加密货币分析师 → 交易员 → 决策
"""
import os
from typing import Dict, Any, Tuple
from datetime import datetime

from langchain_openai import ChatOpenAI
from langgraph.graph import END, StateGraph, START
from langgraph.prebuilt import ToolNode

from tradingagents.agents.analysts.market_analyst import create_market_analyst
from tradingagents.agents.analysts.crypto_analyst import create_crypto_analyst
from tradingagents.agents.utils.agent_states import AgentState
from tradingagents.crypto_config import get_crypto_config
from tradingagents.dataflows.config import set_config

# 导入工具
from tradingagents.agents.utils.agent_utils import (
    get_crypto_data,
    get_crypto_indicators,
    get_crypto_funding_rate,
    get_crypto_order_book,
    get_crypto_market_info,
)

# 导入交易执行器
from tradingagents.executors.binance_executor import BinanceExecutor

# 导入美化日志工具
from tradingagents.utils.logger import ColorLogger
from tradingagents.utils.llm_utils import llm_retry


class SimpleCryptoTradingGraph:
    """简化版加密货币交易图 - 3 个核心 Agent"""
    
    def __init__(self, debug=False, config: Dict[str, Any] = None, auto_execute=False):
        """
        初始化简化版交易图
        
        流程：市场分析师 → 加密货币分析师 → 交易员 → 输出决策
        """
        self.debug = debug
        self.config = config or get_crypto_config()
        self.auto_execute = auto_execute
        
        # 更新配置
        set_config(self.config)
        
        # 创建必要的目录
        os.makedirs(
            os.path.join(self.config["project_dir"], "dataflows/data_cache"),
            exist_ok=True,
        )
        
        # 初始化 LLM
        self._initialize_llm()
        
        # 构建图
        self.graph = self._build_graph()
        
        # 初始化交易执行器
        self.executor = BinanceExecutor(self.config) if auto_execute else None
        
        if self.debug:
            ColorLogger.success("简化版交易图初始化完成")
            print(f"   {ColorLogger.CYAN}→ 深度思考模型: {self.config['deep_think_llm']}{ColorLogger.RESET}")
            print(f"   {ColorLogger.CYAN}→ 快速思考模型: {self.config['quick_think_llm']}{ColorLogger.RESET}")
            print(f"   {ColorLogger.CYAN}→ 自动执行: {auto_execute}{ColorLogger.RESET}\n")
    
    def _initialize_llm(self):
        """初始化 LLM"""
        provider = self.config.get("llm_provider", "openai")
        
        if provider == "openai":
            self.llm = ChatOpenAI(
                model=self.config["quick_think_llm"],
                base_url=self.config["backend_url"]
            )
        else:
            raise ValueError(f"不支持的 LLM 提供商: {provider}")
    
    def _build_graph(self):
        """构建简化的工作流图"""
        
        # 创建分析师节点
        market_analyst = create_market_analyst(self.llm)
        crypto_analyst = create_crypto_analyst(self.llm)
        
        # 创建工具节点
        market_tools = ToolNode([get_crypto_data, get_crypto_indicators])
        crypto_tools = ToolNode([
            get_crypto_funding_rate,
            get_crypto_order_book,
            get_crypto_market_info,
        ])
        
        # 创建交易员节点
        def create_simple_trader():
            """创建简化版交易员 - 不使用记忆，包含当前仓位信息"""
            def trader_node(state):
                symbol = state["company_of_interest"]
                market_report = state.get("market_report", "未提供市场分析")
                crypto_report = state.get("crypto_analysis_report", "未提供加密货币分析")
                
                # 获取当前仓位和账户信息
                position_info = self._get_position_info(symbol)
                
                if self.debug:
                    ColorLogger.position_info(position_info)
                
                prompt = f"""你是一位专业的加密货币交易员。基于以下分析报告和当前仓位状况，给出明确的交易决策。

**市场技术分析**：
{market_report}

**加密货币专属分析**（资金费率、订单簿等）：
{crypto_report}

**当前账户和仓位信息**：
{position_info}

---

**你的任务**：
1. 综合以上分析和当前持仓情况，给出交易方向：BUY（做多）/ SELL（做空）/ HOLD（观望）/ CLOSE（平仓）
2. 说明你的理由（技术面、链上数据、当前盈亏状况等）
3. 如果是 BUY 或 SELL，建议：
   - 入场价位
   - 止损价位
   - 止盈目标
   - 建议仓位大小（占总资金的百分比）
4. 如果是 CLOSE，说明平仓理由

**重要决策原则**：
- 当前杠杆倍数：{self.config.get('binance_leverage', 10)}x
- 最大风险敞口：单笔交易不超过总资金的 {self.config.get('risk_per_trade', 0.02) * 100}%
- 如果已有持仓且浮亏严重（超过 -5%），考虑止损
- 如果已有持仓且浮盈较好（超过 +3%），考虑止盈或持有
- 如果信号不明确或市场波动过大，应选择 HOLD
- 避免频繁开仓平仓，确保每次交易都有充分理由

请用中文回答，最后必须以以下格式结尾：
**最终决策: BUY** 或 **最终决策: SELL** 或 **最终决策: HOLD** 或 **最终决策: CLOSE**
"""
                
                if self.debug:
                    ColorLogger.subheader("📝 发送给交易员的 Prompt")
                    print(f"{ColorLogger.YELLOW}{prompt}{ColorLogger.RESET}\n")
                    ColorLogger.info("等待 LLM 响应...")
                
                # 添加重试机制防止 404 等临时错误
                @llm_retry(max_retries=3, base_delay=10.0, backoff_factor=2.0)
                def invoke_trader_llm():
                    return self.llm.invoke(prompt)
                
                response = invoke_trader_llm()
                
                if self.debug:
                    ColorLogger.llm_response("交易员", response.content, max_lines=150)
                
                return {
                    "final_trade_decision": response.content,
                    "messages": [response]
                }
            
            return trader_node
        
        trader = create_simple_trader()
        
        # 删除消息节点
        def create_msg_delete():
            def delete_messages(state):
                return {"messages": []}
            return delete_messages
        
        delete_market = create_msg_delete()
        delete_crypto = create_msg_delete()
        
        # 构建工作流
        workflow = StateGraph(AgentState)
        
        # 添加节点
        workflow.add_node("Market Analyst", market_analyst)
        workflow.add_node("tools_market", market_tools)
        workflow.add_node("delete_market", delete_market)
        
        workflow.add_node("Crypto Analyst", crypto_analyst)
        workflow.add_node("tools_crypto", crypto_tools)
        workflow.add_node("delete_crypto", delete_crypto)
        
        workflow.add_node("Trader", trader)
        
        # 定义流程
        workflow.add_edge(START, "Market Analyst")
        
        # 市场分析师 → 工具调用 → 删除消息 → 加密货币分析师
        workflow.add_conditional_edges(
            "Market Analyst",
            self._should_use_tools,
            {
                "tools": "tools_market",
                "continue": "delete_market",
            }
        )
        workflow.add_edge("tools_market", "Market Analyst")
        workflow.add_edge("delete_market", "Crypto Analyst")
        
        # 加密货币分析师 → 工具调用 → 删除消息 → 交易员
        workflow.add_conditional_edges(
            "Crypto Analyst",
            self._should_use_tools,
            {
                "tools": "tools_crypto",
                "continue": "delete_crypto",
            }
        )
        workflow.add_edge("tools_crypto", "Crypto Analyst")
        workflow.add_edge("delete_crypto", "Trader")
        
        # 交易员 → 结束
        workflow.add_edge("Trader", END)
        
        return workflow.compile()
    
    def _get_position_info(self, symbol: str) -> str:
        """获取当前仓位和账户信息"""
        try:
            if not self.executor:
                # 如果没有 executor，尝试创建一个临时的来获取信息
                from tradingagents.executors.binance_executor import BinanceExecutor
                temp_executor = BinanceExecutor(self.config)
                return temp_executor.get_position_summary(symbol)
            else:
                return self.executor.get_position_summary(symbol)
        except Exception as e:
            return f"无法获取仓位信息: {str(e)}\n建议：按新开仓位处理"
    
    def _should_use_tools(self, state):
        """判断是否需要调用工具"""
        messages = state.get("messages", [])
        if not messages:
            return "continue"
        
        last_message = messages[-1]
        
        # 检查是否有工具调用
        if hasattr(last_message, "tool_calls") and last_message.tool_calls:
            if self.debug:
                print(f"🔧 检测到工具调用: {len(last_message.tool_calls)} 个")
            return "tools"
        
        return "continue"
    
    def propagate(self, symbol: str, trade_date: str) -> Tuple[Dict, str]:
        """
        执行交易分析流程
        
        Args:
            symbol: 交易对，如 BTC/USDT
            trade_date: 交易日期
            
        Returns:
            (最终状态, 交易决策)
        """
        # 初始化状态
        init_state = {
            "company_of_interest": symbol,
            "trade_date": trade_date,
            "messages": [],
            "market_report": "",
            "crypto_analysis_report": "",
            "final_trade_decision": "",
        }
        
        if self.debug:
            ColorLogger.header(f"开始分析 {symbol}", '=', 80)
            ColorLogger.info(f"交易日期: {trade_date}")
            ColorLogger.info(f"时间周期: {self.config.get('crypto_timeframe', '1h')}")
            ColorLogger.info(f"杠杆倍数: {self.config.get('binance_leverage', 10)}x")
            print()
        
        # 运行图
        final_state = None
        current_analyst = None  # 跟踪当前分析师
        
        for step, chunk in enumerate(self.graph.stream(init_state), 1):
            node_name = list(chunk.keys())[0]
            node_state = list(chunk.values())[0]
            
            if self.debug:
                ColorLogger.step(step, node_name)
                
                # 跟踪当前分析师（用于显示工具结果）
                if "Analyst" in node_name:
                    current_analyst = node_name
                
                # 显示工具调用结果
                if node_name.startswith("tools_"):
                    messages = node_state.get("messages", [])
                    for msg in messages:
                        # 检查是否是工具消息
                        if hasattr(msg, 'content') and hasattr(msg, 'name'):
                            tool_name = getattr(msg, 'name', 'Unknown Tool')
                            tool_content = msg.content
                            ColorLogger.tool_result(tool_name, tool_content, max_lines=60)
                
                # 显示 LLM 响应（分析师的输出）
                if "Analyst" in node_name:
                    messages = node_state.get("messages", [])
                    if messages:
                        last_msg = messages[-1]
                        if hasattr(last_msg, 'content') and last_msg.content and not hasattr(last_msg, 'tool_calls'):
                            # 这是最终的分析报告（没有工具调用）
                            ColorLogger.llm_response(node_name, last_msg.content, max_lines=80)
                
                # 显示生成的报告摘要
                if "market_report" in node_state and node_state["market_report"]:
                    if current_analyst == "Market Analyst":
                        ColorLogger.success(f"市场分析报告已生成 ({len(node_state['market_report'])} 字符)")
                        # 显示报告内容
                        if node_state["market_report"] and len(node_state["market_report"]) > 100:
                            print(f"\n{ColorLogger.BRIGHT_CYAN}{'─' * 80}{ColorLogger.RESET}")
                            print(f"{ColorLogger.BOLD}{ColorLogger.CYAN}📊 市场技术分析报告{ColorLogger.RESET}")
                            print(f"{ColorLogger.BRIGHT_CYAN}{'─' * 80}{ColorLogger.RESET}")
                            print(node_state["market_report"])
                            print(f"{ColorLogger.BRIGHT_CYAN}{'─' * 80}{ColorLogger.RESET}\n")
                
                if "crypto_analysis_report" in node_state and node_state["crypto_analysis_report"]:
                    if current_analyst == "Crypto Analyst":
                        ColorLogger.success(f"加密货币分析报告已生成 ({len(node_state['crypto_analysis_report'])} 字符)")
                        # 显示报告内容
                        if node_state["crypto_analysis_report"] and len(node_state["crypto_analysis_report"]) > 100:
                            print(f"\n{ColorLogger.BRIGHT_MAGENTA}{'─' * 80}{ColorLogger.RESET}")
                            print(f"{ColorLogger.BOLD}{ColorLogger.MAGENTA}💰 加密货币专属分析报告{ColorLogger.RESET}")
                            print(f"{ColorLogger.BRIGHT_MAGENTA}{'─' * 80}{ColorLogger.RESET}")
                            print(node_state["crypto_analysis_report"])
                            print(f"{ColorLogger.BRIGHT_MAGENTA}{'─' * 80}{ColorLogger.RESET}\n")
                
                if "final_trade_decision" in node_state and node_state["final_trade_decision"]:
                    ColorLogger.success("交易决策已生成")
            
            final_state = chunk
        
        # 提取最终决策
        if final_state:
            last_node_state = list(final_state.values())[0]
            decision = last_node_state.get("final_trade_decision", "无法获取决策")
        else:
            decision = "分析流程未完成"
        
        return final_state, decision
    
    def execute_trade(self, decision: str):
        """执行交易（如果启用自动执行）"""
        if not self.auto_execute:
            print("\n⚠️ 自动执行未启用（AUTO_EXECUTE=false）")
            print("💡 如需自动交易，请在 .env 中设置 AUTO_EXECUTE=true")
            return
        
        if not self.executor:
            print("❌ 交易执行器未初始化")
            return
        
        # 解析决策
        if "**最终决策: BUY**" in decision or "**最终决策: LONG**" in decision:
            action = "BUY"
        elif "**最终决策: SELL**" in decision or "**最终决策: SHORT**" in decision:
            action = "SELL"
        elif "**最终决策: CLOSE**" in decision:
            action = "CLOSE"
        else:
            print("\n📊 决策为 HOLD，不执行交易")
            return
        
        symbol = self.config["crypto_symbol"]
        quantity = self.config["position_size"]
        
        ColorLogger.header("准备执行交易", '=', 80)
        print(f"{ColorLogger.BOLD}交易对:{ColorLogger.RESET} {symbol}")
        print(f"{ColorLogger.BOLD}操作:{ColorLogger.RESET} {ColorLogger.BRIGHT_YELLOW}{action}{ColorLogger.RESET}")
        print(f"{ColorLogger.BOLD}数量:{ColorLogger.RESET} {quantity}")
        print(f"{ColorLogger.BOLD}杠杆:{ColorLogger.RESET} {self.config.get('binance_leverage', 10)}x")
        
        test_mode = self.config.get('binance_test_mode')
        if test_mode:
            print(f"{ColorLogger.BOLD}模式:{ColorLogger.RESET} {ColorLogger.GREEN}测试模式 ✅{ColorLogger.RESET}")
        else:
            print(f"{ColorLogger.BOLD}模式:{ColorLogger.RESET} {ColorLogger.BRIGHT_RED}实盘模式 ⚠️{ColorLogger.RESET}")
        print()
        
        try:
            if action == "BUY":
                ColorLogger.info("📈 正在开多单...")
                result = self.executor.open_long(symbol, quantity)
            elif action == "SELL":
                ColorLogger.info("📉 正在开空单...")
                result = self.executor.open_short(symbol, quantity)
            else:  # CLOSE
                ColorLogger.info("🔄 正在平仓...")
                result = self.executor.close_position(symbol)
            
            ColorLogger.success("交易执行成功!")
            print(f"\n{ColorLogger.GREEN}{'─' * 80}{ColorLogger.RESET}")
            print(f"{ColorLogger.BOLD}执行结果:{ColorLogger.RESET}")
            print(result)
            print(f"{ColorLogger.GREEN}{'─' * 80}{ColorLogger.RESET}\n")
        except Exception as e:
            ColorLogger.error("交易执行失败!")
            print(f"\n{ColorLogger.RED}{'─' * 80}{ColorLogger.RESET}")
            print(f"{ColorLogger.BOLD}错误信息:{ColorLogger.RESET}")
            print(str(e))
            print(f"{ColorLogger.RED}{'─' * 80}{ColorLogger.RESET}\n")

