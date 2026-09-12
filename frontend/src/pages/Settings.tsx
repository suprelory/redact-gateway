export default function Settings() {
  return (
    <div className="space-y-8">
      <div>
        <h2 className="text-3xl font-semibold mb-2">设置</h2>
        <p className="text-muted-foreground">配置网关系统参数</p>
      </div>

      <div className="grid gap-6">
        {/* General Settings */}
        <section className="bg-card border border-border rounded-lg p-6">
          <h3 className="text-lg font-semibold mb-4">常规设置</h3>
          <div className="space-y-4">
            <SettingItem
              label="代理端口"
              description="客户端连接的代理端口"
              value="18787"
            />
            <SettingItem
              label="管理端口"
              description="Web 管理界面端口"
              value="18788"
            />
            <SettingItem
              label="最大请求体大小"
              description="单个请求体的最大大小（MB）"
              value="32"
            />
          </div>
        </section>

        {/* Security Settings */}
        <section className="bg-card border border-border rounded-lg p-6">
          <h3 className="text-lg font-semibold mb-4">安全设置</h3>
          <div className="space-y-4">
            <SettingItem
              label="映射过期时间"
              description="映射表条目的过期时间（天）"
              value="7"
            />
            <SettingItem
              label="最大脱敏条目"
              description="单个请求允许的最大脱敏条目数"
              value="16384"
            />
            <div>
              <label className="text-sm font-medium mb-2 block">主密钥状态</label>
              <div className="flex items-center gap-2">
                <div className="w-2 h-2 rounded-full bg-green-500"></div>
                <span className="text-sm text-muted-foreground">已配置</span>
                <button className="ml-auto text-sm text-primary hover:underline">
                  轮换密钥
                </button>
              </div>
            </div>
          </div>
        </section>

        {/* Advanced Settings */}
        <section className="bg-card border border-border rounded-lg p-6">
          <h3 className="text-lg font-semibold mb-4">高级设置</h3>
          <div className="space-y-4">
            <div>
              <label className="text-sm font-medium mb-2 block">允许的上游主机</label>
              <textarea
                className="w-full px-4 py-2 bg-background border border-border rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-primary"
                rows={3}
                placeholder="api.openai.com&#10;api.anthropic.com&#10;留空允许所有主机"
                defaultValue="api.openai.com&#10;api.anthropic.com"
              />
              <p className="text-xs text-muted-foreground mt-1">每行一个主机名，留空允许所有主机</p>
            </div>
            <SettingItem
              label="日志级别"
              description="系统日志的详细程度"
              value="info"
            />
          </div>
        </section>

        {/* Danger Zone */}
        <section className="bg-card border border-destructive rounded-lg p-6">
          <h3 className="text-lg font-semibold text-destructive mb-4">危险区域</h3>
          <div className="space-y-4">
            <div>
              <p className="text-sm mb-2">清空所有映射数据</p>
              <button className="px-4 py-2 text-sm bg-destructive text-destructive-foreground rounded-lg hover:bg-destructive/90 transition-colors">
                清空映射表
              </button>
            </div>
          </div>
        </section>
      </div>
    </div>
  )
}

interface SettingItemProps {
  label: string
  description: string
  value: string
}

function SettingItem({ label, description, value }: SettingItemProps) {
  return (
    <div className="flex items-center justify-between">
      <div>
        <p className="text-sm font-medium">{label}</p>
        <p className="text-xs text-muted-foreground">{description}</p>
      </div>
      <input
        type="text"
        defaultValue={value}
        className="w-32 px-3 py-2 bg-background border border-border rounded-lg text-sm text-right focus:outline-none focus:ring-2 focus:ring-primary"
      />
    </div>
  )
}
