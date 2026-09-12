export default function Traffic() {
  return (
    <div className="space-y-8">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-3xl font-semibold mb-2">实时流量</h2>
          <p className="text-muted-foreground">查看请求日志和脱敏详情</p>
        </div>
        <div className="flex items-center gap-2">
          <input
            type="search"
            placeholder="搜索..."
            className="px-4 py-2 bg-card border border-border rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-primary"
          />
        </div>
      </div>

      {/* Traffic Table */}
      <div className="bg-card border border-border rounded-lg overflow-hidden">
        <table className="w-full">
          <thead className="bg-muted/50">
            <tr className="text-left text-sm text-muted-foreground">
              <th className="px-6 py-3 font-medium">时间</th>
              <th className="px-6 py-3 font-medium">方法</th>
              <th className="px-6 py-3 font-medium">路径</th>
              <th className="px-6 py-3 font-medium">状态</th>
              <th className="px-6 py-3 font-medium">协议</th>
              <th className="px-6 py-3 font-medium">命中规则</th>
              <th className="px-6 py-3 font-medium">耗时</th>
              <th className="px-6 py-3 font-medium">操作</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-border">
            {MOCK_LOGS.map((log) => (
              <tr key={log.id} className="hover:bg-muted/30 transition-colors">
                <td className="px-6 py-4 text-sm text-muted-foreground">
                  {log.timestamp}
                </td>
                <td className="px-6 py-4">
                  <span className="px-2 py-1 text-xs font-semibold bg-primary/10 text-primary rounded">
                    {log.method}
                  </span>
                </td>
                <td className="px-6 py-4 text-sm font-mono">{log.path}</td>
                <td className="px-6 py-4">
                  <span className={`px-2 py-1 text-xs font-semibold rounded ${
                    log.status === 200
                      ? 'bg-green-500/10 text-green-500'
                      : 'bg-red-500/10 text-red-500'
                  }`}>
                    {log.status}
                  </span>
                </td>
                <td className="px-6 py-4 text-sm">{log.protocol}</td>
                <td className="px-6 py-4 text-sm">{log.hitRules} 条</td>
                <td className="px-6 py-4 text-sm text-muted-foreground">{log.duration}ms</td>
                <td className="px-6 py-4">
                  <button className="text-sm text-primary hover:underline">
                    详情
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}

const MOCK_LOGS = [
  { id: '1', timestamp: '14:23:45', method: 'POST', path: '/v1/chat/completions', status: 200, protocol: 'OpenAI', hitRules: 3, duration: 1245 },
  { id: '2', timestamp: '14:23:12', method: 'POST', path: '/v1/messages', status: 200, protocol: 'Anthropic', hitRules: 2, duration: 987 },
  { id: '3', timestamp: '14:22:56', method: 'POST', path: '/v1/chat/completions', status: 200, protocol: 'OpenAI', hitRules: 1, duration: 1567 },
  { id: '4', timestamp: '14:22:34', method: 'POST', path: '/v1/responses', status: 200, protocol: 'OpenAI', hitRules: 0, duration: 2134 },
  { id: '5', timestamp: '14:22:01', method: 'POST', path: '/v1/messages', status: 500, protocol: 'Anthropic', hitRules: 5, duration: 245 },
]
