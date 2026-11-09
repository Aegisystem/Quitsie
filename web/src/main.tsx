import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import '@fontsource-variable/jost'
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { followPageLife } from './diagnostics'
import './i18n'
import { startServiceWorker } from './pwa'
import { App } from './ui/App'
import './ui/styles.css'
import { followOtherTabs } from './ui/theme'

followOtherTabs()
followPageLife()
startServiceWorker()

const queryClient = new QueryClient()

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <App />
    </QueryClientProvider>
  </StrictMode>,
)
