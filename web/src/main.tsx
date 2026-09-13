import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { HashRouter } from 'react-router-dom'
import App from './App'
import './styles.css'

const storedTheme = localStorage.getItem('redact_gateway_theme')
document.documentElement.dataset.theme = storedTheme === 'dark' ? 'dark' : 'light'

const queryClient = new QueryClient({
  defaultOptions: {
    queries: { retry: 1, staleTime: 2_000 },
  },
})

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <HashRouter>
        <App />
      </HashRouter>
    </QueryClientProvider>
  </StrictMode>,
)
