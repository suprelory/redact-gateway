import { act, cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import App from './App'
import { api, saveToken } from './api'
import type { GatewayEvent } from './types'

const sampleEvent: GatewayEvent = {
  id: 1, request_id: 'request-one', timestamp: '2026-09-13T12:00:00Z',
  method: 'POST', protocol: 'openai_chat', upstream_scheme: 'https',
  upstream_host: 'example.com', upstream_path: '/v1/chat/completions',
  flags: 'EG', streaming: true, status: 200, duration_ms: 123,
  request_bytes: 256, response_bytes: 128, redaction_count: 2, restore_count: 2,
  restore_unique_count: 1, restore_unresolved_count: 0, restore_degraded_count: 0, restore_status: 'restored',
  rule_hits: { EMAIL: 2 },
  redaction_fields: ['$.messages[0].content', '$.tool.arguments'],
}

const clients: QueryClient[] = []

function renderLogs(events: GatewayEvent[], total = events.length) {
  saveToken('test-admin-token')
  vi.spyOn(api, 'status').mockResolvedValue({
    service: 'redact-gateway', version: 'test', proxy_addr: '127.0.0.1:8787',
    admin_addr: '127.0.0.1:8788', started_at: sampleEvent.timestamp, uptime_seconds: 60,
    in_flight: 0, allowed_hosts: 1, allow_private_upstreams: false, max_body_bytes: 1024,
  })
  vi.spyOn(api, 'events').mockResolvedValue({ events, total, page: 1, limit: 50 })
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: Infinity } } })
  clients.push(client)
  render(<QueryClientProvider client={client}><MemoryRouter initialEntries={['/logs']}><App /></MemoryRouter></QueryClientProvider>)
  return client
}

const pagedEvents = Array.from({ length: 105 }, (_, index) => ({
  ...sampleEvent, id: index + 1, request_id: `request-${index + 1}`,
}))

function renderPagedLogs() {
  renderLogs(pagedEvents.slice(0, 50), pagedEvents.length)
  vi.mocked(api.events).mockImplementation(async (limit = 50, page = 1) => ({
    events: pagedEvents.slice((page - 1) * limit, page * limit),
    total: pagedEvents.length, page, limit,
  }))
}

afterEach(() => {
  vi.useRealTimers()
  cleanup()
  clients.splice(0).forEach((client) => client.clear())
  vi.restoreAllMocks()
})

describe('App', () => {
  beforeEach(() => {
    sessionStorage.clear()
  })

  it('requires a control plane token before loading the console', () => {
    render(<App />)
    expect(screen.getByRole('heading', { name: 'Redact Gateway' })).toBeInTheDocument()
    expect(screen.getByLabelText('管理令牌')).toBeInTheDocument()
  })

  it('opens the selected request with actual rule hits and field paths', async () => {
    renderLogs([sampleEvent])
    fireEvent.click(await screen.findByRole('button', { name: '查看请求 request-one 的详情' }))
    const dialog = within(screen.getByRole('dialog', { name: '请求详情' }))
    expect(dialog.getByText('request-one')).toBeInTheDocument()
    expect(dialog.getByText('邮箱')).toBeInTheDocument()
    expect(dialog.getByText('EMAIL')).toBeInTheDocument()
    expect(dialog.getByText('2 次')).toBeInTheDocument()
    expect(dialog.getByText('$.messages[0].content')).toBeInTheDocument()
    expect(dialog.getByText('$.tool.arguments')).toBeInTheDocument()
    expect(dialog.queryByText('GitHub Token')).not.toBeInTheDocument()
    expect(dialog.getByText('详情仅展示统计、规则和字段路径，不包含请求或响应中的敏感原文。')).toBeInTheDocument()
  })

  it('distinguishes zero restorations from unresolved placeholders', async () => {
    renderLogs([
      { ...sampleEvent, restore_count: 0, restore_unique_count: 0, restore_status: 'no_placeholders' },
      { ...sampleEvent, id: 2, request_id: 'unresolved', restore_count: 0, restore_unique_count: 0, restore_unresolved_count: 3, restore_status: 'unresolved' },
    ])
    expect(await screen.findByText('无需还原')).toBeInTheDocument()
    expect(screen.getByText('存在未还原')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: '查看请求 unresolved 的详情' }))
    const dialog = within(screen.getByRole('dialog'))
    expect(dialog.getByText('响应中存在无法匹配原文的占位符。')).toBeInTheDocument()
    expect(within(dialog.getByText('未还原次数').parentElement!).getByText('3')).toBeInTheDocument()
  })

  it('shows unique and degraded counts separately from occurrence counts', async () => {
    renderLogs([{ ...sampleEvent, restore_count: 5, restore_unique_count: 2, restore_degraded_count: 1 }])
    fireEvent.click(await screen.findByRole('button', { name: '查看请求 request-one 的详情' }))
    const dialog = within(screen.getByRole('dialog'))
    expect(within(dialog.getByText('还原次数').parentElement!).getByText('5')).toBeInTheDocument()
    expect(within(dialog.getByText('唯一还原数').parentElement!).getByText('2')).toBeInTheDocument()
    expect(within(dialog.getByText('容错还原次数').parentElement!).getByText('1')).toBeInTheDocument()
  })

  it('does not invent diagnostic zeros for historical records', async () => {
    renderLogs([{ ...sampleEvent, restore_status: 'unavailable', restore_unique_count: 0, restore_unresolved_count: 0 }])
    fireEvent.click(await screen.findByRole('button', { name: '查看请求 request-one 的详情' }))
    const dialog = within(screen.getByRole('dialog'))
    expect(dialog.getByText('此记录未保存还原诊断信息，无法补录历史统计。')).toBeInTheDocument()
    expect(within(dialog.getByText('唯一还原数').parentElement!).getByText('—')).toBeInTheDocument()
    expect(within(dialog.getByText('未还原次数').parentElement!).getByText('—')).toBeInTheDocument()
  })

  it('keeps keyboard focus in the dialog and restores it after Escape', async () => {
    renderLogs([sampleEvent])
    const trigger = await screen.findByRole('button', { name: '查看请求 request-one 的详情' })
    trigger.focus()
    fireEvent.click(trigger)
    const close = screen.getByRole('button', { name: '关闭请求详情' })
    expect(close).toHaveFocus()
    fireEvent.keyDown(close, { key: 'Tab' })
    expect(close).toHaveFocus()
    fireEvent.keyDown(close, { key: 'Tab', shiftKey: true })
    expect(close).toHaveFocus()
    fireEvent.keyDown(close, { key: 'Escape' })
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(trigger).toHaveFocus()
  })

  it('closes via button or backdrop, but not when clicking inside the dialog', async () => {
    renderLogs([sampleEvent])
    const trigger = await screen.findByRole('button', { name: '查看请求 request-one 的详情' })
    fireEvent.click(trigger)
    fireEvent.mouseDown(screen.getByRole('dialog'))
    expect(screen.getByRole('dialog')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: '关闭请求详情' }))
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    fireEvent.click(trigger)
    fireEvent.mouseDown(screen.getByRole('presentation'))
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('shows empty details and errors for a request without redactions', async () => {
    renderLogs([{ ...sampleEvent, rule_hits: {}, redaction_fields: [], redaction_count: 0, error_class: 'upstream_blocked', status: 403 }])
    fireEvent.click(await screen.findByRole('button', { name: '查看请求 request-one 的详情' }))
    const dialog = within(screen.getByRole('dialog'))
    expect(dialog.getByText('本次请求未命中脱敏规则')).toBeInTheDocument()
    expect(dialog.getByText('本次请求未发现可记录的脱敏字段')).toBeInTheDocument()
    expect(dialog.getByText('upstream_blocked')).toBeInTheDocument()
  })

  it('explains missing field paths on legacy records', async () => {
    const legacy = { ...sampleEvent }
    Reflect.deleteProperty(legacy, 'redaction_fields')
    renderLogs([legacy])
    fireEvent.click(await screen.findByRole('button', { name: '查看请求 request-one 的详情' }))
    expect(screen.getByText('此记录未保存脱敏字段路径（可能来自旧版本）')).toBeInTheDocument()
  })

  it('keeps the selected details open when refreshed results change', async () => {
    const client = renderLogs([sampleEvent])
    fireEvent.click(await screen.findByRole('button', { name: '查看请求 request-one 的详情' }))
    act(() => { client.setQueryData(['events', 50, 1, ''], { events: [], total: 0, page: 1, limit: 50 }) })
    expect(within(screen.getByRole('dialog')).getByText('request-one')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: '关闭请求详情' }))
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(screen.getByText('暂无数据')).toBeInTheDocument()
  })

  it('navigates server pages and opens details for a record on a later page', async () => {
    renderPagedLogs()
    expect(await screen.findByText('第 1–50 条，共 105 条')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '首页' })).toBeDisabled()
    expect(screen.getByRole('button', { name: '上一页' })).toBeDisabled()
    fireEvent.click(screen.getByRole('button', { name: '下一页' }))
    expect(await screen.findByText('第 51–100 条，共 105 条')).toBeInTheDocument()
    expect(api.events).toHaveBeenLastCalledWith(50, 2, '')
    expect(screen.queryByRole('button', { name: '查看请求 request-1 的详情' })).not.toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: '查看请求 request-51 的详情' }))
    expect(within(screen.getByRole('dialog')).getByText('request-51')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: '关闭请求详情' }))
    fireEvent.click(screen.getByRole('button', { name: '末页' }))
    expect(await screen.findByText('第 101–105 条，共 105 条')).toBeInTheDocument()
    expect(api.events).toHaveBeenLastCalledWith(50, 3, '')
    expect(screen.getByRole('button', { name: '下一页' })).toBeDisabled()
    expect(screen.getByRole('button', { name: '末页' })).toBeDisabled()
    fireEvent.click(screen.getByRole('button', { name: '上一页' }))
    expect(await screen.findByText('第 51–100 条，共 105 条')).toBeInTheDocument()
    await waitFor(() => expect(screen.getByRole('button', { name: '首页' })).toBeEnabled())
    fireEvent.click(screen.getByRole('button', { name: '首页' }))
    expect(await screen.findByText('第 1–50 条，共 105 条')).toBeInTheDocument()
  })

  it('returns to the first page when changing the page size', async () => {
    renderPagedLogs()
    await screen.findByText('第 1–50 条，共 105 条')
    fireEvent.click(screen.getByRole('button', { name: '下一页' }))
    await screen.findByText('第 51–100 条，共 105 条')
    fireEvent.change(screen.getByRole('combobox', { name: '每页日志条数' }), { target: { value: '20' } })
    expect(await screen.findByText('第 1–20 条，共 105 条')).toBeInTheDocument()
    expect(screen.getByText('第 1 / 6 页')).toBeInTheDocument()
    expect(api.events).toHaveBeenLastCalledWith(20, 1, '')
  })

  it('searches all stored logs and resets to the first matching page', async () => {
    renderPagedLogs()
    await screen.findByText('第 1–50 条，共 105 条')
    fireEvent.click(screen.getByRole('button', { name: '下一页' }))
    await screen.findByText('第 51–100 条，共 105 条')
    vi.mocked(api.events).mockResolvedValue({
      events: [{ ...sampleEvent, request_id: 'archived-request', upstream_host: 'archive.example.com' }],
      total: 1, page: 1, limit: 50,
    })
    fireEvent.change(screen.getByRole('textbox', { name: '搜索请求日志' }), { target: { value: '  ARCHIVE  ' } })
    expect(await screen.findByRole('button', { name: '查看请求 archived-request 的详情' })).toBeInTheDocument()
    expect(api.events).toHaveBeenLastCalledWith(50, 1, 'ARCHIVE')
    expect(screen.getByText('第 1–1 条，共 1 条')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '下一页' })).toBeDisabled()
  })

  it('refreshes the current page manually and automatically', async () => {
    renderPagedLogs()
    await screen.findByText('第 1–50 条，共 105 条')
    fireEvent.click(screen.getByRole('button', { name: '下一页' }))
    await screen.findByText('第 51–100 条，共 105 条')
    const calls = vi.mocked(api.events).mock.calls.length
    fireEvent.click(screen.getByRole('button', { name: '立即刷新请求日志' }))
    await waitFor(() => expect(api.events).toHaveBeenCalledTimes(calls + 1))
    expect(api.events).toHaveBeenLastCalledWith(50, 2, '')
    await waitFor(() => expect(screen.getByRole('button', { name: '立即刷新请求日志' })).toBeEnabled())
    fireEvent.change(screen.getByRole('combobox', { name: '请求日志刷新频率' }), { target: { value: 'manual' } })
    vi.useFakeTimers()
    vi.mocked(api.events).mockClear()
    fireEvent.change(screen.getByRole('combobox', { name: '请求日志刷新频率' }), { target: { value: '10000' } })
    await act(async () => { await vi.advanceTimersByTimeAsync(10000) })
    expect(api.events).toHaveBeenCalledWith(50, 2, '')
    expect(screen.getByText('第 2 / 3 页')).toBeInTheDocument()
  })

  it('accepts the server page correction when no matching records remain', async () => {
    renderPagedLogs()
    await screen.findByText('第 1–50 条，共 105 条')
    vi.mocked(api.events).mockResolvedValue({ events: [], total: 0, page: 1, limit: 50 })
    fireEvent.click(screen.getByRole('button', { name: '末页' }))
    await waitFor(() => expect(api.events).toHaveBeenLastCalledWith(50, 1, ''))
    expect(await screen.findByText('第 0–0 条，共 0 条')).toBeInTheDocument()
    expect(screen.getByText('第 1 / 1 页')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '上一页' })).toBeDisabled()
    expect(screen.getByRole('button', { name: '下一页' })).toBeDisabled()
  })

  it('shows a page load error and retries the requested page', async () => {
    renderPagedLogs()
    await screen.findByText('第 1–50 条，共 105 条')
    vi.mocked(api.events).mockRejectedValueOnce(new Error('offline'))
    fireEvent.click(screen.getByRole('button', { name: '下一页' }))
    expect(await screen.findByText('无法读取请求日志，请点击立即刷新重试。')).toBeInTheDocument()
    expect(screen.getByText('第 2 页')).toBeInTheDocument()
    expect(screen.queryByText('暂无数据')).not.toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: '立即刷新请求日志' }))
    expect(await screen.findByText('第 51–100 条，共 105 条')).toBeInTheDocument()
    expect(api.events).toHaveBeenLastCalledWith(50, 2, '')
  })
})
