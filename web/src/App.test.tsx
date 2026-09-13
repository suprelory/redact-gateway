import { render, screen } from '@testing-library/react'
import { beforeEach, describe, expect, it } from 'vitest'
import App from './App'

describe('App', () => {
  beforeEach(() => {
    sessionStorage.clear()
  })

  it('requires a control plane token before loading the console', () => {
    render(<App />)
    expect(screen.getByRole('heading', { name: 'Redact Gateway' })).toBeInTheDocument()
    expect(screen.getByLabelText('管理令牌')).toBeInTheDocument()
  })
})
