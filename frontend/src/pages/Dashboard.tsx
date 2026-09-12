export default function Dashboard() {
  return (
    <div className="space-y-8">
      <div>
        <h2 className="text-3xl font-semibold mb-2">仪表盘</h2>
        <p className="text-muted-foreground">实时监控网关运行状态与统计数据</p>
      </div>

      {/* Stats Grid */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
        <StatCard
          label="今日请求"
          value="1,234"
          change="+12.3%"
          trend="up"
        />
        <StatCard
          label="脱敏命中"
          value="856"
          change="+8.7%"
          trend="up"
        />
        <StatCard
          label="还原次数"
          value="798"
          change="-2.4%"
          trend="down"
        />
        <StatCard
          label="活跃规则"
          value="18"
          change="0%"
          trend="neutral"
        />
      </div>

      {/* Charts */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <div className="bg-card border border-border rounded-lg p-6">
          <h3 className="text-lg font-semibold mb-4">请求趋势</h3>
          <div className="h-64 flex items-center justify-center text-muted-foreground">
            图表占位
          </div>
        </div>

        <div className="bg-card border border-border rounded-lg p-6">
          <h3 className="text-lg font-semibold mb-4">规则命中分布</h3>
          <div className="h-64 flex items-center justify-center text-muted-foreground">
            图表占位
          </div>
        </div>
      </div>

      {/* Recent Activity */}
      <div className="bg-card border border-border rounded-lg p-6">
        <h3 className="text-lg font-semibold mb-4">最近活动</h3>
        <div className="space-y-3">
          {[1, 2, 3, 4, 5].map((i) => (
            <div
              key={i}
              className="flex items-center justify-between py-3 border-b border-border last:border-0"
            >
              <div className="flex items-center gap-3">
                <div className="w-2 h-2 rounded-full bg-primary"></div>
                <div>
                  <p className="text-sm font-medium">POST /v1/chat/completions</p>
                  <p className="text-xs text-muted-foreground">命中 3 条规则</p>
                </div>
              </div>
              <div className="text-xs text-muted-foreground">2 分钟前</div>
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}

interface StatCardProps {
  label: string
  value: string
  change: string
  trend: 'up' | 'down' | 'neutral'
}

function StatCard({ label, value, change, trend }: StatCardProps) {
  const trendColor = {
    up: 'text-green-500',
    down: 'text-red-500',
    neutral: 'text-muted-foreground',
  }[trend]

  return (
    <div className="bg-card border border-border rounded-lg p-6">
      <p className="text-sm text-muted-foreground mb-2">{label}</p>
      <div className="flex items-end justify-between">
        <p className="text-3xl font-semibold">{value}</p>
        <p className={`text-sm font-medium ${trendColor}`}>{change}</p>
      </div>
    </div>
  )
}
