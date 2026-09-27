export function defaultGatewayURL(
  proxyAddr?: string,
  location: Pick<Location, 'protocol' | 'host' | 'hostname'> = window.location,
): string {
  const hostname = location.hostname || '127.0.0.1'
  const directHost = hostname === 'localhost' || hostname.endsWith('.localhost')
    || /^\d{1,3}(?:\.\d{1,3}){3}$/.test(hostname) || hostname.includes(':')
  if (!directHost) return `${location.protocol}//${location.host}`

  const host = hostname.includes(':') && !hostname.startsWith('[') ? `[${hostname}]` : hostname
  const port = proxyAddr?.match(/:(\d+)$/)?.[1] ?? '8787'
  return `http://${host}${port === '80' ? '' : `:${port}`}`
}
