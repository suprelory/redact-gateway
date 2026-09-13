import { act, cleanup, fireEvent, render, screen, within } from '@testing-library/react'
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
  rule_hits: { EMAIL: 2 },
  redaction_fields: ['$.messages[0].content', '$.tool.arguments'],
}

const clients: QueryClient[] = []

function renderLogs(events: GatewayEvent[]) {
  saveToken('test-admin-token')
  vi.spyOn(api, 'status').mockResolvedValue({
    service: 'redact-gateway', version: 'test', proxy_addr: '127.0.0.1:8787',
    admin_addr: '127.0.0.1:8788', started_at: sampleEvent.timestamp, uptime_seconds: 60,
    in_flight: 0, allowed_hosts: 1, allow_private_upstreams: false, max_body_bytes: 1024,
  })
  vi.spyOn(api, 'events').mockResolvedValue({ events })
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: Infinity } } })
  clients.push(client)
  render(<QueryClientProvider client={client}><MemoryRouter initialEntries={['/logs']}><App /></MemoryRouter></QueryClientProvider>)
  return client
}

afterEach(() => {
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
    expect(dialog.getByText('详情仅展示规则和字段路径，不包含请求或响应中的敏感原文。')).toBeInTheDocument()
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
    act(() => { client.setQueryData(['events', 200], { events: [] }) })
    expect(within(screen.getByRole('dialog')).getByText('request-one')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: '关闭请求详情' }))
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(screen.getByText('暂无数据')).toBeInTheDocument()
  })
})
