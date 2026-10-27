import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import '@fontsource-variable/jost'
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './i18n'
import { App } from './ui/App'
import './ui/styles.css'

const queryClient = new QueryClient()

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <App />
    </QueryClientProvider>
  </StrictMode>,
)
