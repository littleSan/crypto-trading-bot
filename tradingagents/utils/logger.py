"""
美化的终端日志输出工具
"""
from datetime import datetime


class ColorLogger:
    """带颜色的日志输出"""
    
    # ANSI 颜色代码
    RESET = '\033[0m'
    BOLD = '\033[1m'
    
    # 前景色
    BLACK = '\033[30m'
    RED = '\033[31m'
    GREEN = '\033[32m'
    YELLOW = '\033[33m'
    BLUE = '\033[34m'
    MAGENTA = '\033[35m'
    CYAN = '\033[36m'
    WHITE = '\033[37m'
    
    # 高亮前景色
    BRIGHT_RED = '\033[91m'
    BRIGHT_GREEN = '\033[92m'
    BRIGHT_YELLOW = '\033[93m'
    BRIGHT_BLUE = '\033[94m'
    BRIGHT_MAGENTA = '\033[95m'
    BRIGHT_CYAN = '\033[96m'
    
    # 背景色
    BG_BLACK = '\033[40m'
    BG_RED = '\033[41m'
    BG_GREEN = '\033[42m'
    BG_YELLOW = '\033[43m'
    BG_BLUE = '\033[44m'
    BG_MAGENTA = '\033[45m'
    BG_CYAN = '\033[46m'
    BG_WHITE = '\033[47m'
    
    @classmethod
    def header(cls, text, char='=', width=80):
        """打印标题"""
        print(f"\n{cls.BOLD}{cls.BRIGHT_CYAN}{char * width}{cls.RESET}")
        print(f"{cls.BOLD}{cls.BRIGHT_CYAN}{text.center(width)}{cls.RESET}")
        print(f"{cls.BOLD}{cls.BRIGHT_CYAN}{char * width}{cls.RESET}\n")
    
    @classmethod
    def subheader(cls, text, char='─', width=80):
        """打印子标题"""
        print(f"\n{cls.BRIGHT_BLUE}{char * width}{cls.RESET}")
        print(f"{cls.BOLD}{cls.BRIGHT_BLUE}{text}{cls.RESET}")
        print(f"{cls.BRIGHT_BLUE}{char * width}{cls.RESET}\n")
    
    @classmethod
    def success(cls, text):
        """成功消息"""
        print(f"{cls.BRIGHT_GREEN}✅ {text}{cls.RESET}")
    
    @classmethod
    def error(cls, text):
        """错误消息"""
        print(f"{cls.BRIGHT_RED}❌ {text}{cls.RESET}")
    
    @classmethod
    def warning(cls, text):
        """警告消息"""
        print(f"{cls.BRIGHT_YELLOW}⚠️  {text}{cls.RESET}")
    
    @classmethod
    def info(cls, text):
        """信息消息"""
        print(f"{cls.CYAN}ℹ️  {text}{cls.RESET}")
    
    @classmethod
    def step(cls, step_num, text):
        """步骤消息"""
        print(f"{cls.BOLD}{cls.BRIGHT_MAGENTA}🔄 [步骤 {step_num}] {text}{cls.RESET}")
    
    @classmethod
    def tool_call(cls, tool_name):
        """工具调用"""
        print(f"{cls.YELLOW}🔧 调用工具: {cls.BOLD}{tool_name}{cls.RESET}")
    
    @classmethod
    def tool_result(cls, tool_name, result, max_lines=50):
        """工具结果"""
        print(f"\n{cls.BOLD}{cls.BG_BLUE}{cls.WHITE} Tool Message: {tool_name} {cls.RESET}")
        print(f"{cls.GREEN}{'─' * 80}{cls.RESET}")
        
        # 限制输出行数（如果太长）
        lines = result.split('\n')
        if len(lines) > max_lines:
            print('\n'.join(lines[:max_lines]))
            print(f"{cls.YELLOW}... (省略 {len(lines) - max_lines} 行){cls.RESET}")
        else:
            print(result)
        
        print(f"{cls.GREEN}{'─' * 80}{cls.RESET}\n")
    
    @classmethod
    def llm_response(cls, agent_name, content, max_lines=100):
        """LLM 响应"""
        print(f"\n{cls.BOLD}{cls.BG_MAGENTA}{cls.WHITE} {agent_name} LLM 响应 {cls.RESET}")
        print(f"{cls.MAGENTA}{'─' * 80}{cls.RESET}")
        
        lines = content.split('\n')
        if len(lines) > max_lines:
            print('\n'.join(lines[:max_lines]))
            print(f"{cls.YELLOW}... (省略 {len(lines) - max_lines} 行){cls.RESET}")
        else:
            print(content)
        
        print(f"{cls.MAGENTA}{'─' * 80}{cls.RESET}\n")
    
    @classmethod
    def position_info(cls, info):
        """持仓信息"""
        print(f"\n{cls.BOLD}{cls.BG_CYAN}{cls.BLACK} 💼 账户和持仓信息 {cls.RESET}")
        print(f"{cls.CYAN}{'─' * 80}{cls.RESET}")
        print(info)
        print(f"{cls.CYAN}{'─' * 80}{cls.RESET}\n")
    
    @classmethod
    def decision(cls, decision_text):
        """最终决策"""
        print(f"\n{cls.BOLD}{cls.BG_GREEN}{cls.BLACK} ✅ 最终交易决策 {cls.RESET}")
        print(f"{cls.GREEN}{'=' * 80}{cls.RESET}")
        print(decision_text)
        print(f"{cls.GREEN}{'=' * 80}{cls.RESET}\n")
    
    @classmethod
    def timestamp(cls):
        """时间戳"""
        return f"{cls.CYAN}[{datetime.now().strftime('%H:%M:%S')}]{cls.RESET}"

