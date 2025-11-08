"""
Web 监控服务器 - 实时展示交易信息和 LLM 决策
"""
from flask import Flask, render_template, jsonify, request
from datetime import datetime
import json
import os
from pathlib import Path
import threading


class TradingMonitor:
    """交易监控数据管理"""
    
    def __init__(self):
        self.trading_logs = []  # 交易日志
        self.max_logs = 100  # 最多保存100条记录
        self.current_status = {
            'running': False,
            'last_update': None,
            'next_run': None,
            'run_count': 0
        }
    
    def add_log(self, log_entry):
        """添加日志"""
        log_entry['timestamp'] = datetime.now().strftime('%Y-%m-%d %H:%M:%S')
        self.trading_logs.insert(0, log_entry)  # 新的在前面
        
        # 限制日志数量
        if len(self.trading_logs) > self.max_logs:
            self.trading_logs = self.trading_logs[:self.max_logs]
    
    def update_status(self, status_update):
        """更新状态"""
        self.current_status.update(status_update)
        self.current_status['last_update'] = datetime.now().strftime('%Y-%m-%d %H:%M:%S')
    
    def get_latest_logs(self, count=20):
        """获取最新的日志"""
        return self.trading_logs[:count]
    
    def get_status(self):
        """获取当前状态"""
        return self.current_status


# 全局监控实例
monitor = TradingMonitor()

# Flask 应用
app = Flask(__name__, 
            template_folder=os.path.join(os.path.dirname(__file__), 'templates'),
            static_folder=os.path.join(os.path.dirname(__file__), 'static'))


@app.route('/')
def index():
    """主页"""
    return render_template('monitor.html')


@app.route('/api/status')
def get_status():
    """获取系统状态"""
    return jsonify(monitor.get_status())


@app.route('/api/logs')
def get_logs():
    """获取交易日志"""
    count = int(request.args.get('count', 20))
    return jsonify({
        'logs': monitor.get_latest_logs(count),
        'total': len(monitor.trading_logs)
    })


@app.route('/api/latest')
def get_latest():
    """获取最新的一条记录（用于实时更新）"""
    logs = monitor.get_latest_logs(1)
    return jsonify({
        'status': monitor.get_status(),
        'latest_log': logs[0] if logs else None
    })


def start_monitor_server(host='0.0.0.0', port=5000):
    """启动监控服务器"""
    app.run(host=host, port=port, debug=False, threaded=True)


def start_monitor_thread(host='0.0.0.0', port=5000):
    """在后台线程启动监控服务器"""
    thread = threading.Thread(
        target=start_monitor_server,
        args=(host, port),
        daemon=True
    )
    thread.start()
    return thread


if __name__ == '__main__':
    # 测试运行
    print("启动监控服务器...")
    print("访问: http://localhost:5000")
    start_monitor_server(host='0.0.0.0', port=5000)

