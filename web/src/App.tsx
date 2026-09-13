import { FormEvent, useEffect, useMemo, useState } from 'react'
import { Navigate, NavLink, Route, Routes } from 'react-router-dom'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  Activity,
  BarChart3,
  Check,
  Clipboard,
  FileClock,
  Gauge,
  KeyRound,
  LogOut,
  Moon,
	RefreshCw,
	Save,
	Search,
  Settings,
  ShieldCheck,
  Sun,
  TerminalSquare,
} from 'lucide-react'
import { api, clearToken, readToken, saveToken } from './api'
import type { GatewayEvent, GatewayStatus } from './types'

const navItems = [
  { to: '/', label: '控制台', icon: Gauge },
  { to: '/logs', label: '请求日志', icon: FileClock },
  { to: '/rules', label: '脱敏规则', icon: ShieldCheck },
  { to: '/settings', label: '运行设置', icon: Settings },
]

export default function App() {
  const [token, setToken] = useState(readToken())
  if (!token) {
    return <TokenGate onAuthenticated={setToken} />
  }
  return <Shell onLogout={() => { clearToken(); setToken('') }} />
}

function TokenGate({ onAuthenticated }: { onAuthenticated: (token: string) => void }) {
  const [value, setValue] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const submit = async (event: FormEvent) => {
    event.preventDefault()
    const token = value.trim()
    if (!token) return
    setBusy(true)
    setError('')
    try {
      await api.status(token)
      saveToken(token)
      onAuthenticated(token)
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : '验证失败')
    } finally {
      setBusy(false)
    }
  }
  return (
    <main className="auth-screen">
      <form className="auth-panel" onSubmit={submit}>
        <div className="brand-mark"><ShieldCheck /></div>
        <div>
          <h1>Redact Gateway</h1>
          <p>控制面身份验证</p>
        </div>
        <label htmlFor="admin-token">管理令牌</label>
        <div className="input-with-icon">
          <KeyRound />
          <input id="admin-token" type="password" value={value} onChange={(event) => setValue(event.target.value)} autoFocus />
        </div>
        {error && <div className="inline-error">{error}</div>}
        <button className="primary-button" type="submit" disabled={busy}>{busy ? '验证中' : '进入控制台'}</button>
      </form>
    </main>
  )
}

function Shell({ onLogout }: { onLogout: () => void }) {
  const [dark, setDark] = useState(document.documentElement.dataset.theme === 'dark')
  const statusQuery = useQuery({ queryKey: ['status'], queryFn: () => api.status(), refetchInterval: 5_000 })
  const status = statusQuery.data
  const toggleTheme = () => {
    const next = !dark
    setDark(next)
    document.documentElement.dataset.theme = next ? 'dark' : 'light'
    localStorage.setItem('redact_gateway_theme', next ? 'dark' : 'light')
  }
  return (
    <div className="app-shell">
      <aside className="sidebar">
        <div className="brand">
          <div className="brand-mark"><ShieldCheck /></div>
          <div className="brand-copy"><strong>Redact Gateway</strong><span>Local privacy relay</span></div>
        </div>
        <nav>
          {navItems.map((item) => (
            <NavLink key={item.to} to={item.to} end={item.to === '/'} title={item.label}>
              <item.icon /><span>{item.label}</span>
            </NavLink>
          ))}
        </nav>
        <div className="sidebar-foot">
          <span>v{status?.version ?? 'dev'}</span>
        </div>
      </aside>
      <div className="main-column">
        <header className="topbar">
          <div className="runtime-state">
            <span className={`status-dot ${statusQuery.isError ? 'error' : 'ok'}`} />
            <span>{statusQuery.isError ? '控制面不可用' : '网关运行中'}</span>
            {status && <code>{status.proxy_addr}</code>}
          </div>
          <div className="top-actions">
            <button className="icon-button" onClick={toggleTheme} title={dark ? '切换浅色主题' : '切换深色主题'}>
              {dark ? <Sun /> : <Moon />}
            </button>
            <button className="icon-button" onClick={onLogout} title="退出控制台"><LogOut /></button>
          </div>
        </header>
        <main className="content">
          {statusQuery.isError && <ErrorBanner message="无法连接控制面，请确认服务状态和管理令牌。" />}
          <Routes>
            <Route path="/" element={<Dashboard status={status} />} />
            <Route path="/logs" element={<Logs />} />
            <Route path="/rules" element={<Rules />} />
            <Route path="/settings" element={<RuntimeSettings status={status} />} />
            <Route path="*" element={<Navigate to="/" replace />} />
          </Routes>
        </main>
      </div>
    </div>
  )
}

function Dashboard({ status }: { status?: GatewayStatus }) {
  const statsQuery = useQuery({ queryKey: ['stats', 24], queryFn: () => api.stats(24), refetchInterval: 5_000 })
  const eventsQuery = useQuery({ queryKey: ['events', 12], queryFn: () => api.events(12), refetchInterval: 5_000 })
  const overview = statsQuery.data?.overview
  const cards = [
    { label: '24 小时请求', value: overview?.requests ?? 0, icon: Activity, tone: 'blue' },
    { label: '脱敏命中', value: overview?.redactions ?? 0, icon: ShieldCheck, tone: 'green' },
    { label: '成功还原', value: overview?.restores ?? 0, icon: RefreshCw, tone: 'teal' },
    { label: '异常请求', value: overview?.errors ?? 0, icon: BarChart3, tone: 'amber' },
  ]
  return (
    <section className="page">
      <PageHeader title="控制台" meta={`运行 ${formatUptime(status?.uptime_seconds ?? 0)}`} />
      <div className="status-strip">
        <div><ShieldCheck /><span><strong>脱敏代理在线</strong><small>请求级映射 · SSE 增量还原 · 默认阻断私有上游</small></span></div>
        <div className="status-facts"><span>处理中 <b>{status?.in_flight ?? 0}</b></span><span>上限 <b>{formatBytes(status?.max_body_bytes ?? 0)}</b></span></div>
      </div>
      <div className="metric-grid">
        {cards.map((card) => <MetricCard key={card.label} {...card} />)}
      </div>
      <div className="two-column">
        <section className="panel">
          <PanelTitle title="上游调用排行" />
          <div className="table-wrap compact-table">
            <table>
              <thead><tr><th>上游</th><th>请求</th><th>错误</th><th>平均耗时</th></tr></thead>
              <tbody>
                {(statsQuery.data?.top_upstreams ?? []).map((item) => (
                  <tr key={item.host}><td className="host-cell">{item.host}</td><td>{item.requests}</td><td>{item.errors}</td><td>{Math.round(item.average_ms)} ms</td></tr>
                ))}
                {!statsQuery.data?.top_upstreams.length && <EmptyRow columns={4} />}
              </tbody>
            </table>
          </div>
        </section>
        <section className="panel">
          <PanelTitle title="最近请求" />
          <div className="event-list">
            {(eventsQuery.data?.events ?? []).slice(0, 8).map((event) => (
              <div className="event-row" key={event.id}>
                <StatusCode value={event.status} />
                <span className="event-target"><strong>{event.upstream_host}</strong><small>{event.upstream_path}</small></span>
                <span className="event-count">{event.redaction_count}</span>
                <time>{formatClock(event.timestamp)}</time>
              </div>
            ))}
            {!eventsQuery.data?.events.length && <div className="empty-state">暂无请求记录</div>}
          </div>
        </section>
      </div>
    </section>
  )
}

function Logs() {
	const [search, setSearch] = useState('')
	const [refreshInterval, setRefreshInterval] = useState<number | false>(5_000)
	const eventsQuery = useQuery({ queryKey: ['events', 200], queryFn: () => api.events(200), refetchInterval: refreshInterval })
  const events = useMemo(() => {
    const needle = search.trim().toLowerCase()
    if (!needle) return eventsQuery.data?.events ?? []
    return (eventsQuery.data?.events ?? []).filter((event) =>
      `${event.upstream_host} ${event.upstream_path} ${event.protocol} ${event.flags} ${event.status}`.toLowerCase().includes(needle),
    )
  }, [eventsQuery.data?.events, search])
	return (
		<section className="page full-height-page">
			<PageHeader title="请求日志" meta={`${events.length} 条`} action={
				<div className="refresh-controls">
					<button
						className="icon-button bordered"
						title="立即刷新"
						aria-label="立即刷新请求日志"
						onClick={() => { void eventsQuery.refetch() }}
						disabled={eventsQuery.isFetching}
					>
						<RefreshCw className={eventsQuery.isFetching ? 'spin' : undefined} />
					</button>
					<label className="refresh-select">
						<span className="sr-only">请求日志刷新频率</span>
						<select
							aria-label="请求日志刷新频率"
							value={refreshInterval === false ? 'manual' : String(refreshInterval)}
							onChange={(event) => {
								const value = event.target.value
								setRefreshInterval(value === 'manual' ? false : Number(value))
							}}
						>
							<option value="manual">手动刷新</option>
							<option value="5000">自动刷新 · 5 秒</option>
							<option value="10000">自动刷新 · 10 秒</option>
							<option value="30000">自动刷新 · 30 秒</option>
							<option value="60000">自动刷新 · 60 秒</option>
						</select>
					</label>
				</div>
			} />
      <div className="toolbar">
        <div className="search-field"><Search /><input value={search} onChange={(event) => setSearch(event.target.value)} placeholder="搜索上游、接口、协议或状态" /></div>
      </div>
      <section className="panel log-panel">
        <div className="table-wrap">
          <table>
            <thead><tr><th>时间</th><th>上游</th><th>接口</th><th>协议</th><th>规则</th><th>脱敏/还原</th><th>状态</th><th>耗时</th></tr></thead>
            <tbody>
              {events.map((event) => <LogRow key={event.id} event={event} />)}
              {!events.length && <EmptyRow columns={8} />}
            </tbody>
          </table>
        </div>
      </section>
    </section>
  )
}

function Rules() {
  const rulesQuery = useQuery({ queryKey: ['rules'], queryFn: api.rules })
  return (
    <section className="page">
      <PageHeader title="脱敏规则" meta={`默认组合 ${rulesQuery.data?.all_flags ?? 'HPSIBEG'}`} />
      <section className="panel rule-panel">
        <div className="rule-grid">
          {(rulesQuery.data?.rules ?? []).map((rule) => (
            <div className="rule-row" key={rule.flag}>
              <span className="flag-box">{rule.flag}</span>
              <span><strong>{rule.name}</strong><small>{rule.description}</small></span>
              <span className="enabled-mark"><Check />已启用</span>
            </div>
          ))}
        </div>
      </section>
    </section>
  )
}

function RuntimeSettings({ status }: { status?: GatewayStatus }) {
	const queryClient = useQueryClient()
	const settingsQuery = useQuery({ queryKey: ['settings'], queryFn: () => api.settings() })
	const [allowedHosts, setAllowedHosts] = useState('')
	const saveMutation = useMutation({
		mutationFn: (hosts: string[]) => api.updateSettings({ allowed_hosts: hosts }),
		onSuccess: (settings) => {
			setAllowedHosts(settings.allowed_hosts.join('\n'))
			queryClient.setQueryData(['settings'], settings)
			queryClient.invalidateQueries({ queryKey: ['status'] })
		},
	})
	useEffect(() => {
		if (settingsQuery.data) setAllowedHosts(settingsQuery.data.allowed_hosts.join('\n'))
	}, [settingsQuery.data])
	const saveAllowedHosts = (event: FormEvent) => {
		event.preventDefault()
		const hosts = allowedHosts.split(/[\r\n,]+/).map((host) => host.trim()).filter(Boolean)
		saveMutation.mutate(hosts)
	}
	const [gatewayBase, setGatewayBase] = useState(() => `http://${window.location.hostname || '127.0.0.1'}:8787`)
	const examples = [
		{ label: 'OpenAI 全规则', value: `${gatewayBase}/$https://api.openai.com/v1` },
		{ label: 'OpenAI 常用规则', value: `${gatewayBase}/HPSE$https://api.openai.com/v1` },
		{ label: 'Anthropic', value: `${gatewayBase}/PSE$https://api.anthropic.com` },
	]
	return (
		<section className="page">
			<PageHeader title="运行设置" meta="运行时信息" />
			<section className="panel settings-band">
				<PanelTitle title="上游白名单" />
				<form className="settings-form" onSubmit={saveAllowedHosts}>
					<label className="field-label" htmlFor="allowed-hosts">允许的上游域名</label>
					<textarea
						id="allowed-hosts"
						className="plain-input settings-textarea"
						value={allowedHosts}
						onChange={(event) => setAllowedHosts(event.target.value)}
						placeholder="api.openai.com\napi.anthropic.com"
						disabled={settingsQuery.isPending || saveMutation.isPending}
					/>
					<p className="field-help">每行一个域名。留空表示允许所有公网 HTTP/HTTPS 上游，私有地址仍受安全策略限制。</p>
					{settingsQuery.isError && <ErrorBanner message="无法读取上游白名单。" />}
					{saveMutation.isError && <ErrorBanner message={saveMutation.error instanceof Error ? saveMutation.error.message : '保存上游白名单失败。'} />}
					<div className="settings-actions">
						<button className="primary-button action-button" type="submit" disabled={settingsQuery.isPending || saveMutation.isPending}>
							<Save />{saveMutation.isPending ? '保存中' : '保存白名单'}
						</button>
						{saveMutation.isSuccess && <span className="success-message"><Check />已保存并立即生效</span>}
					</div>
				</form>
			</section>
			<section className="panel settings-band">
				<PanelTitle title="网关地址" />
				<label className="field-label" htmlFor="gateway-base">数据面基础地址</label>
				<input id="gateway-base" className="plain-input" value={gatewayBase} onChange={(event) => setGatewayBase(event.target.value.replace(/\/$/, ''))} />
				<div className="copy-list">
					{examples.map((example) => <CopyField key={example.label} {...example} />)}
				</div>
			</section>
			<section className="panel settings-band">
				<PanelTitle title="安全边界" />
				<dl className="definition-grid">
					<div><dt>请求体上限</dt><dd>{formatBytes(status?.max_body_bytes ?? 0)}</dd></div>
					<div><dt>上游白名单</dt><dd>{status?.allowed_hosts ? `${status.allowed_hosts} 个域名` : '未配置'}</dd></div>
					<div><dt>私有网络上游</dt><dd>{status?.allow_private_upstreams ? '允许' : '阻断'}</dd></div>
					<div><dt>代理监听</dt><dd><code>{status?.proxy_addr ?? '127.0.0.1:8787'}</code></dd></div>
				</dl>
			</section>
		</section>
	)
}

function MetricCard({ label, value, icon: Icon, tone }: { label: string; value: number; icon: typeof Activity; tone: string }) {
  return <div className="metric-card"><div className={`metric-icon ${tone}`}><Icon /></div><span>{label}</span><strong>{value.toLocaleString()}</strong></div>
}

function PageHeader({ title, meta, action }: { title: string; meta?: string; action?: React.ReactNode }) {
  return <header className="page-header"><div><h1>{title}</h1>{meta && <span>{meta}</span>}</div>{action}</header>
}

function PanelTitle({ title }: { title: string }) {
  return <div className="panel-title"><h2>{title}</h2></div>
}

function LogRow({ event }: { event: GatewayEvent }) {
  return (
    <tr>
      <td><time>{formatDateTime(event.timestamp)}</time></td>
      <td className="host-cell">{event.upstream_host}</td>
      <td><code className="path-code">{event.upstream_path}</code></td>
      <td><Protocol value={event.protocol} streaming={event.streaming} /></td>
      <td><code>{event.flags}</code></td>
      <td><span className="count-pair"><b>{event.redaction_count}</b><span>/</span>{event.restore_count}</span></td>
      <td><StatusCode value={event.status} /></td>
      <td>{event.duration_ms} ms</td>
    </tr>
  )
}

function Protocol({ value, streaming }: { value: string; streaming: boolean }) {
  const labels: Record<string, string> = { openai_chat: 'OpenAI Chat', openai_responses: 'Responses', anthropic_messages: 'Anthropic', generic: 'Generic' }
  return <span className="protocol"><span>{labels[value] ?? value}</span>{streaming && <em>SSE</em>}</span>
}

function StatusCode({ value }: { value: number }) {
  const tone = value >= 500 ? 'bad' : value >= 400 ? 'warn' : value >= 200 && value < 300 ? 'good' : 'neutral'
  return <span className={`status-code ${tone}`}>{value || '—'}</span>
}

function CopyField({ label, value }: { label: string; value: string }) {
  const [copied, setCopied] = useState(false)
  const copy = async () => {
    await navigator.clipboard.writeText(value)
    setCopied(true)
    window.setTimeout(() => setCopied(false), 1_500)
  }
  return <div className="copy-field"><span>{label}</span><code>{value}</code><button className="icon-button" title="复制" onClick={copy}>{copied ? <Check /> : <Clipboard />}</button></div>
}

function EmptyRow({ columns }: { columns: number }) {
  return <tr><td className="empty-cell" colSpan={columns}>暂无数据</td></tr>
}

function ErrorBanner({ message }: { message: string }) {
  return <div className="error-banner">{message}</div>
}

function formatClock(value: string): string {
  return new Date(value).toLocaleTimeString('zh-CN', { hour12: false, hour: '2-digit', minute: '2-digit', second: '2-digit' })
}

function formatDateTime(value: string): string {
  return new Date(value).toLocaleString('zh-CN', { hour12: false })
}

function formatBytes(value: number): string {
  if (!value) return '—'
  if (value >= 1024 * 1024) return `${Math.round(value / 1024 / 1024)} MiB`
  return `${Math.round(value / 1024)} KiB`
}

function formatUptime(seconds: number): string {
  if (seconds < 60) return `${seconds} 秒`
  if (seconds < 3600) return `${Math.floor(seconds / 60)} 分钟`
  const hours = Math.floor(seconds / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)
  return `${hours} 小时 ${minutes} 分钟`
}
