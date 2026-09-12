export default function Rules() {
  return (
    <div className="space-y-8">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-3xl font-semibold mb-2">规则管理</h2>
          <p className="text-muted-foreground">配置脱敏规则和测试规则效果</p>
        </div>
        <button className="px-4 py-2 bg-primary text-primary-foreground rounded-lg font-medium hover:bg-primary/90 transition-colors">
          添加规则
        </button>
      </div>

      {/* Rule List */}
      <div className="space-y-4">
        {MOCK_RULES.map((rule) => (
          <RuleCard key={rule.id} rule={rule} />
        ))}
      </div>
    </div>
  )
}

interface Rule {
  id: string
  name: string
  type: string
  enabled: boolean
  builtin: boolean
  priority: number
  hitCount: number
}

const MOCK_RULES: Rule[] = [
  { id: '1', name: 'OpenAI API Key', type: 'API_KEY', enabled: true, builtin: true, priority: 90, hitCount: 234 },
  { id: '2', name: 'GitHub Token', type: 'API_KEY', enabled: true, builtin: true, priority: 90, hitCount: 156 },
  { id: '3', name: '数据库连接串', type: 'CONNSTR', enabled: true, builtin: true, priority: 85, hitCount: 89 },
  { id: '4', name: '中国手机号', type: 'PHONE', enabled: true, builtin: true, priority: 80, hitCount: 445 },
  { id: '5', name: '内网 IP', type: 'IPPRIVATE', enabled: true, builtin: true, priority: 65, hitCount: 332 },
  { id: '6', name: '邮箱地址', type: 'EMAIL', enabled: false, builtin: true, priority: 70, hitCount: 0 },
]

function RuleCard({ rule }: { rule: Rule }) {
  return (
    <div className="bg-card border border-border rounded-lg p-6">
      <div className="flex items-start justify-between">
        <div className="flex items-start gap-4 flex-1">
          <div className={`w-10 h-10 rounded-lg flex items-center justify-center text-sm font-semibold ${
            rule.enabled ? 'bg-primary/10 text-primary' : 'bg-muted text-muted-foreground'
          }`}>
            {rule.priority}
          </div>

          <div className="flex-1">
            <div className="flex items-center gap-2 mb-1">
              <h3 className="text-lg font-semibold">{rule.name}</h3>
              {rule.builtin && (
                <span className="px-2 py-0.5 text-xs bg-muted text-muted-foreground rounded">
                  内置
                </span>
              )}
              <span className="px-2 py-0.5 text-xs bg-secondary text-secondary-foreground rounded font-mono">
                {rule.type}
              </span>
            </div>
            <div className="flex items-center gap-4 text-sm text-muted-foreground">
              <span>优先级: {rule.priority}</span>
              <span>命中: {rule.hitCount.toLocaleString()} 次</span>
            </div>
          </div>
        </div>

        <div className="flex items-center gap-2">
          <button className="px-3 py-1.5 text-sm border border-border rounded-lg hover:bg-muted transition-colors">
            测试
          </button>
          <button className="px-3 py-1.5 text-sm border border-border rounded-lg hover:bg-muted transition-colors">
            编辑
          </button>
          <label className="relative inline-flex items-center cursor-pointer">
            <input
              type="checkbox"
              checked={rule.enabled}
              className="sr-only peer"
              readOnly
            />
            <div className={`w-11 h-6 rounded-full transition-colors ${
              rule.enabled ? 'bg-primary' : 'bg-muted'
            }`}>
              <div className={`w-5 h-5 bg-white rounded-full shadow-md transition-transform ${
                rule.enabled ? 'translate-x-5' : 'translate-x-0.5'
              } mt-0.5`}></div>
            </div>
          </label>
        </div>
      </div>
    </div>
  )
}
