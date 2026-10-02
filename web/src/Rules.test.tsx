import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import App from './App'
import { api, saveToken } from './api'
import type { GatewayRules, GatewaySettings } from './types'

const allFlags = 'HPSIBEG'
const names: Record<string, string> = {
  H: '高熵字符串', P: '电话号码', S: 'sk- 密钥', I: '身份证', B: '银行卡', E: '邮箱', G: '凭据规则包',
}
const clients: QueryClient[] = []

function ruleResponse(enabled: string): GatewayRules {
  return {
    all_flags: allFlags,
    enabled_rules: enabled,
    rules: allFlags.split('').map((flag) => ({
      flag, name: names[flag], description: `${names[flag]}检测`, default: true, enabled: enabled.includes(flag),
    })),
  }
}

function renderRules(enabled = allFlags, firstLoad?: () => Promise<GatewayRules>) {
  saveToken('test-admin-token')
  let settings: GatewaySettings = {
    allowed_hosts: ['api.example.com'], gateway_url: 'https://gateway.example.com', enabled_rules: enabled,
  }
  vi.spyOn(api, 'status').mockResolvedValue({
    service: 'redact-gateway', version: 'test', proxy_addr: '127.0.0.1:8787', gateway_url: settings.gateway_url,
    admin_addr: '127.0.0.1:8788', started_at: '2026-09-13T12:00:00Z', uptime_seconds: 60,
    in_flight: 0, allowed_hosts: 1, allow_private_upstreams: false, max_body_bytes: 1024,
  })
  const rules = vi.spyOn(api, 'rules').mockImplementation(async () => ruleResponse(settings.enabled_rules))
  if (firstLoad) rules.mockImplementationOnce(firstLoad)
  vi.spyOn(api, 'updateSettings').mockImplementation(async (changes) => {
    settings = { ...settings, ...changes }
    return settings
  })
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: Infinity } } })
  clients.push(client)
  render(<QueryClientProvider client={client}><MemoryRouter initialEntries={['/rules']}><App /></MemoryRouter></QueryClientProvider>)
  return client
}

beforeEach(() => sessionStorage.clear())
afterEach(() => {
  cleanup()
  clients.splice(0).forEach((client) => client.clear())
  vi.restoreAllMocks()
})

describe('Rule settings', () => {
  it('loads the saved state of each rule', async () => {
    renderRules('PE')
    expect(await screen.findByRole('switch', { name: '邮箱' })).toBeChecked()
    expect(screen.getByRole('switch', { name: '电话号码' })).toBeChecked()
    expect(screen.getByRole('switch', { name: '高熵字符串' })).not.toBeChecked()
    expect(screen.getByText('已启用 2 / 7 项')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '保存规则' })).toBeDisabled()
    expect(api.updateSettings).not.toHaveBeenCalled()
  })

  it('saves only the rule selection and keeps it after refreshing', async () => {
    const client = renderRules('PE')
    fireEvent.click(await screen.findByRole('switch', { name: '邮箱' }))
    expect(screen.getByRole('switch', { name: '邮箱' })).not.toBeChecked()
    expect(screen.getByText('有未保存的更改')).toBeInTheDocument()
    expect(api.updateSettings).not.toHaveBeenCalled()
    fireEvent.click(screen.getByRole('button', { name: '保存规则' }))
    expect(await screen.findByText('已保存并立即生效')).toBeInTheDocument()
    expect(api.updateSettings).toHaveBeenCalledWith({ enabled_rules: 'P' })
    expect(client.getQueryData<GatewaySettings>(['settings'])).toEqual({
      allowed_hosts: ['api.example.com'], gateway_url: 'https://gateway.example.com', enabled_rules: 'P',
    })
    fireEvent.click(screen.getByRole('button', { name: '刷新脱敏规则' }))
    await waitFor(() => expect(api.rules).toHaveBeenCalledTimes(2))
    expect(screen.getByRole('switch', { name: '邮箱' })).not.toBeChecked()
    expect(screen.getByRole('switch', { name: '电话号码' })).toBeChecked()
    expect(screen.getByText('已启用 1 / 7 项')).toBeInTheDocument()
  })

  it('supports saving all disabled and enabling all again', async () => {
    renderRules('PE')
    await screen.findByRole('switch', { name: '邮箱' })
    fireEvent.click(screen.getByRole('button', { name: '全部关闭' }))
    screen.getAllByRole('switch').forEach((toggle) => expect(toggle).not.toBeChecked())
    fireEvent.click(screen.getByRole('button', { name: '保存规则' }))
    await screen.findByText('已保存并立即生效')
    expect(api.updateSettings).toHaveBeenLastCalledWith({ enabled_rules: '' })
    expect(screen.getByText('已启用 0 / 7 项')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: '刷新脱敏规则' }))
    await waitFor(() => expect(api.rules).toHaveBeenCalledTimes(2))
    screen.getAllByRole('switch').forEach((toggle) => expect(toggle).not.toBeChecked())
    fireEvent.click(screen.getByRole('button', { name: '全部开启' }))
    screen.getAllByRole('switch').forEach((toggle) => expect(toggle).toBeChecked())
    fireEvent.click(screen.getByRole('button', { name: '保存规则' }))
    await screen.findByText('已保存并立即生效')
    expect(api.updateSettings).toHaveBeenLastCalledWith({ enabled_rules: allFlags })
    expect(screen.getByText('已启用 7 / 7 项')).toBeInTheDocument()
  })

  it('keeps the saved state and draft separate when saving fails, then allows retry', async () => {
    const client = renderRules('PE')
    vi.mocked(api.updateSettings).mockRejectedValueOnce(new Error('保存脱敏规则失败'))
    fireEvent.click(await screen.findByRole('switch', { name: '电话号码' }))
    fireEvent.click(screen.getByRole('button', { name: '保存规则' }))
    expect(await screen.findByText('保存脱敏规则失败')).toBeInTheDocument()
    expect(screen.getByRole('switch', { name: '电话号码' })).not.toBeChecked()
    expect(screen.getByText('已启用 2 / 7 项')).toBeInTheDocument()
    expect(client.getQueryData<GatewayRules>(['rules'])?.enabled_rules).toBe('PE')
    expect(screen.queryByText('已保存并立即生效')).not.toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: '保存规则' }))
    await screen.findByText('已保存并立即生效')
    expect(screen.getByText('已启用 1 / 7 项')).toBeInTheDocument()
    expect(api.updateSettings).toHaveBeenLastCalledWith({ enabled_rules: 'E' })
  })

  it('disables changes until settings have loaded and allows a failed load to be retried', async () => {
    renderRules('PE', async () => { throw new Error('offline') })
    expect(screen.getByRole('button', { name: '全部关闭' })).toBeDisabled()
    expect(await screen.findByText('无法读取脱敏规则，请点击刷新重试。')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '全部开启' })).toBeDisabled()
    expect(screen.getByRole('button', { name: '保存规则' })).toBeDisabled()
    expect(api.updateSettings).not.toHaveBeenCalled()
    fireEvent.click(screen.getByRole('button', { name: '刷新脱敏规则' }))
    expect(await screen.findByRole('switch', { name: '邮箱' })).toBeChecked()
    expect(screen.getByRole('button', { name: '全部关闭' })).toBeEnabled()
  })

  it('preserves unsaved switches when the rule list is refreshed', async () => {
    renderRules('PE')
    fireEvent.click(await screen.findByRole('switch', { name: '邮箱' }))
    fireEvent.click(screen.getByRole('button', { name: '刷新脱敏规则' }))
    await waitFor(() => expect(api.rules).toHaveBeenCalledTimes(2))
    expect(screen.getByRole('switch', { name: '邮箱' })).not.toBeChecked()
    expect(screen.getByText('有未保存的更改')).toBeInTheDocument()
    expect(screen.getByText('已启用 2 / 7 项')).toBeInTheDocument()
  })

  it('disables switches while saving and reports success only after the server responds', async () => {
    renderRules('PE')
    let finishSave!: (settings: GatewaySettings) => void
    vi.mocked(api.updateSettings).mockImplementationOnce(() => new Promise((resolve) => { finishSave = resolve }))
    fireEvent.click(await screen.findByRole('switch', { name: '邮箱' }))
    fireEvent.click(screen.getByRole('button', { name: '保存规则' }))
    expect(await screen.findByRole('button', { name: '保存中' })).toBeDisabled()
    screen.getAllByRole('switch').forEach((toggle) => expect(toggle).toBeDisabled())
    expect(screen.getByRole('button', { name: '全部关闭' })).toBeDisabled()
    expect(screen.queryByText('已保存并立即生效')).not.toBeInTheDocument()
    await act(async () => {
      finishSave({ allowed_hosts: ['api.example.com'], gateway_url: 'https://gateway.example.com', enabled_rules: 'P' })
    })
    expect(await screen.findByText('已保存并立即生效')).toBeInTheDocument()
    expect(screen.getByRole('switch', { name: '电话号码' })).toBeEnabled()
    expect(screen.getByText('已启用 1 / 7 项')).toBeInTheDocument()
  })
})
