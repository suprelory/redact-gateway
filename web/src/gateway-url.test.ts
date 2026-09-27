import { describe, expect, it } from 'vitest'
import { defaultGatewayURL } from './gateway-url'

describe('defaultGatewayURL', () => {
  it.each([
    ['https://gateway.example.com', '0.0.0.0:8787', 'https://gateway.example.com'],
    ['http://gateway.example.com', '0.0.0.0:8787', 'http://gateway.example.com'],
    ['https://gateway.example.com:8443', '0.0.0.0:8787', 'https://gateway.example.com:8443'],
    ['http://localhost:8788', '127.0.0.1:9000', 'http://localhost:9000'],
    ['http://192.168.1.10:8788', '0.0.0.0:9000', 'http://192.168.1.10:9000'],
    ['http://[::1]:8788', '[::]:9000', 'http://[::1]:9000'],
    ['http://127.0.0.1:8788', '0.0.0.0:80', 'http://127.0.0.1'],
    ['http://localhost:5173', undefined, 'http://localhost:8787'],
  ])('infers %s using the actual proxy port only for direct hosts', (page, proxyAddr, expected) => {
    expect(defaultGatewayURL(proxyAddr, new URL(page))).toBe(expected)
  })
})
