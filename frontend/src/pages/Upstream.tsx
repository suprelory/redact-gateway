export default function Upstream() {
  return (
    <div className="space-y-8">
      <div>
        <h2 className="text-3xl font-semibold mb-2">上游配置</h2>
        <p className="text-muted-foreground">配置上游 API 端点和协议映射</p>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {MOCK_UPSTREAMS.map((upstream) => (
          <UpstreamCard key={upstream.id} upstream={upstream} />
        ))}
      </div>
    </div>
  )
}

interface Upstream {
  id: string
  name: string
  protocol: string
  baseUrl: string
  enabled: boolean
  requestCount: number
}

const MOCK_UPSTREAMS: Upstream[] = [
  { id: '1', name: 'OpenAI Official', protocol: 'OpenAI Chat', baseUrl: 'https://api.openai.com', enabled: true, requestCount: 1234 },
  { id: '2', name: 'Anthropic Official', protocol: 'Anthropic Messages', baseUrl: 'https://api.anthropic.com', enabled: true, requestCount: 567 },
  { id: '3', name: 'Custom Relay', protocol: 'OpenAI Chat', baseUrl: 'https://relay.example.com', enabled: false, requestCount: 0 },
]

function UpstreamCard({ upstream }: { upstream: Upstream }) {
  return (
    <div className="bg-card border border-border rounded-lg p-6">
      <div className="flex items-start justify-between mb-4">
        <div>
          <h3 className="text-lg font-semibold mb-1">{upstream.name}</h3>
          <p className="text-sm text-muted-foreground">{upstream.protocol}</p>
        </div>
        <label className="relative inline-flex items-center cursor-pointer">
          <input
            type="checkbox"
            checked={upstream.enabled}
            className="sr-only peer"
            readOnly
          />
          <div className={`w-11 h-6 rounded-full transition-colors ${
            upstream.enabled ? 'bg-primary' : 'bg-muted'
          }`}>
            <div className={`w-5 h-5 bg-white rounded-full shadow-md transition-transform ${
              upstream.enabled ? 'translate-x-5' : 'translate-x-0.5'
            } mt-0.5`}></div>
          </div>
        </label>
      </div>

      <div className="space-y-3">
        <div>
          <p className="text-xs text-muted-foreground mb-1">Base URL</p>
          <code className="text-sm bg-muted px-2 py-1 rounded">{upstream.baseUrl}</code>
        </div>

        <div className="flex items-center justify-between text-sm">
          <span className="text-muted-foreground">请求次数</span>
          <span className="font-semibold">{upstream.requestCount.toLocaleString()}</span>
        </div>

        <div className="flex items-center gap-2 pt-2">
          <button className="flex-1 px-3 py-2 text-sm border border-border rounded-lg hover:bg-muted transition-colors">
            测试连接
          </button>
          <button className="flex-1 px-3 py-2 text-sm bg-primary text-primary-foreground rounded-lg hover:bg-primary/90 transition-colors">
            编辑
          </button>
        </div>
      </div>
    </div>
  )
}
