"""
加密货币交易图 - 专为 BTC 等加密货币设计的多智能体框架
"""
import os
from pathlib import Path
import json
from datetime import date
from typing import Dict, Any, Tuple, List, Optional

from langchain_openai import ChatOpenAI
from langchain_anthropic import ChatAnthropic
from langchain_google_genai import ChatGoogleGenerativeAI

from langgraph.prebuilt import ToolNode

from tradingagents.agents import *
from tradingagents.crypto_config import get_crypto_config
from tradingagents.agents.utils.memory import FinancialSituationMemory
from tradingagents.agents.utils.agent_states import (
    AgentState,
    InvestDebateState,
    RiskDebateState,
)
from tradingagents.dataflows.config import set_config

# 导入加密货币工具
from tradingagents.agents.utils.agent_utils import (
    get_crypto_data,
    get_crypto_indicators,
    get_crypto_funding_rate,
    get_crypto_order_book,
    get_crypto_market_info,
    get_news,
    get_global_news,
)

from tradingagents.graph.conditional_logic import ConditionalLogic
from tradingagents.graph.setup import GraphSetup
from tradingagents.graph.propagation import Propagator
from tradingagents.graph.reflection import Reflector
from tradingagents.graph.signal_processing import SignalProcessor

# 导入交易执行器
from tradingagents.executors.binance_executor import BinanceExecutor


class CryptoTradingAgentsGraph:
    """加密货币交易智能体图 - 专为 BTC 等加密货币优化"""
    
    def __init__(
        self,
        selected_analysts=["market", "crypto", "social", "news"],  # 默认不包括 fundamentals
        debug=False,
        config: Dict[str, Any] = None,
        auto_execute=False,  # 是否自动执行交易
    ):
        """
        初始化加密货币交易图
        
        Args:
            selected_analysts: 选择的分析师列表
                - "market": 市场技术分析师
                - "crypto": 加密货币专属分析师（资金费率、订单簿等）
                - "social": 社交媒体情绪分析师
                - "news": 新闻分析师
                注意：不包括 "fundamentals"，因为加密货币没有传统财报
            debug: 是否开启调试模式
            config: 配置字典，如果为 None 则使用默认加密货币配置
            auto_execute: 是否自动执行交易（需谨慎）
        """
        self.debug = debug
        self.config = config or get_crypto_config()
        self.auto_execute = auto_execute
        
        # 更新接口配置
        set_config(self.config)
        
        # 创建必要的目录
        os.makedirs(
            os.path.join(self.config["project_dir"], "dataflows/data_cache"),
            exist_ok=True,
        )
        
        # 初始化 LLM
        self._initialize_llms()
        
        # 初始化记忆系统
        self._initialize_memories()
        
        # 创建工具节点（加密货币专属）
        self.tool_nodes = self._create_tool_nodes()
        
        # 初始化组件
        self.conditional_logic = ConditionalLogic()
        self.graph_setup = GraphSetup(
            self.quick_thinking_llm,
            self.deep_thinking_llm,
            self.tool_nodes,
            self.bull_memory,
            self.bear_memory,
            self.trader_memory,
            self.invest_judge_memory,
            self.risk_manager_memory,
            self.conditional_logic,
            self.config,
        )
        
        self.propagator = Propagator()
        self.reflector = Reflector(self.quick_thinking_llm)
        self.signal_processor = SignalProcessor(self.quick_thinking_llm)
        
        # 初始化交易执行器
        self.executor = BinanceExecutor(
            self.config, 
            test_mode=self.config['binance_test_mode']
        )
        
        # 设置交易所
        if not self.config['binance_test_mode']:
            self.executor.setup_exchange(
                self.config['crypto_symbol'],
                self.config['binance_leverage']
            )
        
        # 状态追踪
        self.curr_state = None
        self.ticker = None
        self.log_states_dict = {}
        
        # 设置图（使用加密货币专属的分析师配置）
        self.graph = self._setup_crypto_graph(selected_analysts)
    
    def _initialize_llms(self):
        """初始化语言模型"""
        provider = self.config["llm_provider"].lower()
        
        if provider in ["openai", "ollama", "openrouter"]:
            self.deep_thinking_llm = ChatOpenAI(
                model=self.config["deep_think_llm"],
                base_url=self.config["backend_url"]
            )
            self.quick_thinking_llm = ChatOpenAI(
                model=self.config["quick_think_llm"],
                base_url=self.config["backend_url"]
            )
        elif provider == "anthropic":
            self.deep_thinking_llm = ChatAnthropic(
                model=self.config["deep_think_llm"],
                base_url=self.config["backend_url"]
            )
            self.quick_thinking_llm = ChatAnthropic(
                model=self.config["quick_think_llm"],
                base_url=self.config["backend_url"]
            )
        elif provider == "google":
            self.deep_thinking_llm = ChatGoogleGenerativeAI(
                model=self.config["deep_think_llm"]
            )
            self.quick_thinking_llm = ChatGoogleGenerativeAI(
                model=self.config["quick_think_llm"]
            )
        else:
            raise ValueError(f"不支持的 LLM 提供商: {provider}")
    
    def _initialize_memories(self):
        """初始化记忆系统"""
        self.bull_memory = FinancialSituationMemory("crypto_bull_memory", self.config)
        self.bear_memory = FinancialSituationMemory("crypto_bear_memory", self.config)
        self.trader_memory = FinancialSituationMemory("crypto_trader_memory", self.config)
        self.invest_judge_memory = FinancialSituationMemory("crypto_invest_judge_memory", self.config)
        self.risk_manager_memory = FinancialSituationMemory("crypto_risk_manager_memory", self.config)
    
    def _create_tool_nodes(self) -> Dict[str, ToolNode]:
        """创建加密货币专属的工具节点"""
        return {
            "market": ToolNode([
                get_crypto_data,
                get_crypto_indicators,
            ]),
            "crypto": ToolNode([
                get_crypto_funding_rate,
                get_crypto_order_book,
                get_crypto_market_info,
            ]),
            "social": ToolNode([
                get_news,
            ]),
            "news": ToolNode([
                get_news,
                get_global_news,
            ]),
        }
    
    def _setup_crypto_graph(self, selected_analysts):
        """
        设置加密货币交易图
        
        将 'crypto' 分析师映射到实际的创建函数
        """
        # 创建分析师节点映射
        from langgraph.graph import END, StateGraph, START
        
        workflow = StateGraph(AgentState)
        
        analyst_nodes = {}
        delete_nodes = {}
        tool_nodes = {}
        
        if "market" in selected_analysts:
            analyst_nodes["market"] = create_market_analyst(self.quick_thinking_llm)
            delete_nodes["market"] = create_msg_delete()
            tool_nodes["market"] = self.tool_nodes["market"]
        
        if "crypto" in selected_analysts:
            analyst_nodes["crypto"] = create_crypto_analyst(self.quick_thinking_llm)
            delete_nodes["crypto"] = create_msg_delete()
            tool_nodes["crypto"] = self.tool_nodes["crypto"]
        
        if "social" in selected_analysts:
            analyst_nodes["social"] = create_social_media_analyst(self.quick_thinking_llm)
            delete_nodes["social"] = create_msg_delete()
            tool_nodes["social"] = self.tool_nodes["social"]
        
        if "news" in selected_analysts:
            analyst_nodes["news"] = create_news_analyst(self.quick_thinking_llm)
            delete_nodes["news"] = create_msg_delete()
            tool_nodes["news"] = self.tool_nodes["news"]
        
        # 创建研究员和管理节点
        bull_researcher_node = create_bull_researcher(
            self.quick_thinking_llm, self.bull_memory, self.config
        )
        bear_researcher_node = create_bear_researcher(
            self.quick_thinking_llm, self.bear_memory, self.config
        )
        research_manager_node = create_research_manager(
            self.deep_thinking_llm, self.invest_judge_memory, self.config
        )
        
        # 使用加密货币交易员
        trader_node = create_crypto_trader(self.quick_thinking_llm, self.trader_memory, self.config)
        
        # 创建风险分析节点
        risky_analyst = create_risky_debator(self.quick_thinking_llm)
        neutral_analyst = create_neutral_debator(self.quick_thinking_llm)
        safe_analyst = create_safe_debator(self.quick_thinking_llm)
        risk_manager_node = create_risk_manager(
            self.deep_thinking_llm, self.risk_manager_memory, self.config
        )
        
        # 添加分析师节点到图
        for analyst_type, node in analyst_nodes.items():
            workflow.add_node(f"{analyst_type.capitalize()} Analyst", node)
            workflow.add_node(
                f"Msg Clear {analyst_type.capitalize()}", delete_nodes[analyst_type]
            )
            workflow.add_node(f"tools_{analyst_type}", tool_nodes[analyst_type])
        
        # 添加其他节点
        workflow.add_node("Bull Researcher", bull_researcher_node)
        workflow.add_node("Bear Researcher", bear_researcher_node)
        workflow.add_node("Research Manager", research_manager_node)
        workflow.add_node("Trader", trader_node)
        workflow.add_node("Risky Analyst", risky_analyst)
        workflow.add_node("Neutral Analyst", neutral_analyst)
        workflow.add_node("Safe Analyst", safe_analyst)
        workflow.add_node("Risk Judge", risk_manager_node)
        
        # 定义边
        first_analyst = selected_analysts[0]
        workflow.add_edge(START, f"{first_analyst.capitalize()} Analyst")
        
        # 连接分析师
        for i, analyst_type in enumerate(selected_analysts):
            current_analyst = f"{analyst_type.capitalize()} Analyst"
            current_tools = f"tools_{analyst_type}"
            current_clear = f"Msg Clear {analyst_type.capitalize()}"
            
            workflow.add_conditional_edges(
                current_analyst,
                getattr(self.conditional_logic, f"should_continue_{analyst_type}"),
                [current_tools, current_clear],
            )
            workflow.add_edge(current_tools, current_analyst)
            
            if i < len(selected_analysts) - 1:
                next_analyst = f"{selected_analysts[i+1].capitalize()} Analyst"
                workflow.add_edge(current_clear, next_analyst)
            else:
                workflow.add_edge(current_clear, "Bull Researcher")
        
        # 添加剩余的边
        workflow.add_conditional_edges(
            "Bull Researcher",
            self.conditional_logic.should_continue_debate,
            {
                "Bear Researcher": "Bear Researcher",
                "Research Manager": "Research Manager",
            },
        )
        workflow.add_conditional_edges(
            "Bear Researcher",
            self.conditional_logic.should_continue_debate,
            {
                "Bull Researcher": "Bull Researcher",
                "Research Manager": "Research Manager",
            },
        )
        workflow.add_edge("Research Manager", "Trader")
        workflow.add_edge("Trader", "Risky Analyst")
        workflow.add_conditional_edges(
            "Risky Analyst",
            self.conditional_logic.should_continue_risk_analysis,
            {
                "Safe Analyst": "Safe Analyst",
                "Risk Judge": "Risk Judge",
            },
        )
        workflow.add_conditional_edges(
            "Safe Analyst",
            self.conditional_logic.should_continue_risk_analysis,
            {
                "Neutral Analyst": "Neutral Analyst",
                "Risk Judge": "Risk Judge",
            },
        )
        workflow.add_conditional_edges(
            "Neutral Analyst",
            self.conditional_logic.should_continue_risk_analysis,
            {
                "Risky Analyst": "Risky Analyst",
                "Risk Judge": "Risk Judge",
            },
        )
        
        workflow.add_edge("Risk Judge", END)
        
        return workflow.compile()
    
    def propagate(self, crypto_symbol, trade_date):
        """
        运行加密货币交易智能体图
        
        Args:
            crypto_symbol: 加密货币交易对，如 "BTC/USDT"
            trade_date: 交易日期
            
        Returns:
            (final_state, decision): 最终状态和处理后的决策
        """
        self.ticker = crypto_symbol
        
        # 初始化状态
        init_agent_state = self.propagator.create_initial_state(
            crypto_symbol, trade_date
        )
        args = self.propagator.get_graph_args()
        
        if self.debug:
            # 调试模式带追踪
            trace = []
            for chunk in self.graph.stream(init_agent_state, **args):
                if len(chunk.get("messages", [])) > 0:
                    chunk["messages"][-1].pretty_print()
                    trace.append(chunk)
            
            final_state = trace[-1] if trace else {}
        else:
            # 标准模式
            final_state = self.graph.invoke(init_agent_state, **args)
        
        # 存储当前状态用于反思
        self.curr_state = final_state
        
        # 记录状态
        self._log_state(trade_date, final_state)
        
        # 处理信号
        decision = self.process_signal(final_state.get("final_trade_decision", ""))
        
        # 如果启用自动执行且不是测试模式
        if self.auto_execute and decision:
            self._execute_trade_decision(decision, crypto_symbol)
        
        return final_state, decision
    
    def _execute_trade_decision(self, decision, symbol):
        """执行交易决策"""
        # 解析决策（需要从决策文本中提取交易参数）
        # 这里简化处理，实际应该用 LLM 或正则表达式解析
        action = "HOLD"
        if "BUY" in decision or "做多" in decision:
            action = "BUY"
        elif "SELL" in decision or "做空" in decision:
            action = "SELL"
        
        if action != "HOLD":
            result = self.executor.execute_trade(
                symbol,
                action,
                self.config['position_size'],
                reason="Multi-agent decision"
            )
            print(f"\n交易执行结果: {result}")
    
    def _log_state(self, trade_date, final_state):
        """记录最终状态到 JSON 文件"""
        self.log_states_dict[str(trade_date)] = {
            "crypto_symbol": final_state.get("company_of_interest"),
            "trade_date": final_state.get("trade_date"),
            "market_report": final_state.get("market_report"),
            "crypto_analysis_report": final_state.get("crypto_analysis_report"),
            "sentiment_report": final_state.get("sentiment_report"),
            "news_report": final_state.get("news_report"),
            "investment_debate_state": {
                "bull_history": final_state.get("investment_debate_state", {}).get("bull_history"),
                "bear_history": final_state.get("investment_debate_state", {}).get("bear_history"),
                "judge_decision": final_state.get("investment_debate_state", {}).get("judge_decision"),
            },
            "trader_investment_decision": final_state.get("trader_investment_plan"),
            "risk_debate_state": {
                "judge_decision": final_state.get("risk_debate_state", {}).get("judge_decision"),
            },
            "final_trade_decision": final_state.get("final_trade_decision"),
        }
        
        # 保存到文件
        directory = Path(f"crypto_results/{self.ticker}/CryptoTradingStrategy_logs/")
        directory.mkdir(parents=True, exist_ok=True)
        
        with open(
            f"crypto_results/{self.ticker}/CryptoTradingStrategy_logs/full_states_log_{trade_date}.json",
            "w",
        ) as f:
            json.dump(self.log_states_dict, f, indent=4, ensure_ascii=False)
    
    def reflect_and_remember(self, returns_losses):
        """基于收益反思并更新记忆"""
        self.reflector.reflect_bull_researcher(
            self.curr_state, returns_losses, self.bull_memory
        )
        self.reflector.reflect_bear_researcher(
            self.curr_state, returns_losses, self.bear_memory
        )
        self.reflector.reflect_trader(
            self.curr_state, returns_losses, self.trader_memory
        )
        self.reflector.reflect_invest_judge(
            self.curr_state, returns_losses, self.invest_judge_memory
        )
        self.reflector.reflect_risk_manager(
            self.curr_state, returns_losses, self.risk_manager_memory
        )
    
    def process_signal(self, full_signal):
        """处理信号以提取核心决策"""
        return self.signal_processor.process_signal(full_signal)
    
    def get_current_position(self):
        """获取当前持仓"""
        return self.executor.get_current_position(self.config['crypto_symbol'])
    
    def close_all_positions(self):
        """平掉所有持仓"""
        self.executor.close_all_positions(self.config['crypto_symbol'])

