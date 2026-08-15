import React from 'react'
import ReactDOM from 'react-dom/client'
import { ThemeProvider } from 'next-themes'
import { HashRouter } from 'react-router'
import { TooltipProvider } from '@/components/ui/tooltip'
import { initializeI18n } from '@/i18n'
import App from './App'
import './index.css'

async function bootstrap() {
  await initializeI18n()

  ReactDOM.createRoot(document.getElementById('root') as HTMLElement).render(
    <React.StrictMode>
      <ThemeProvider attribute="class" defaultTheme="system" enableSystem>
        <HashRouter>
          <TooltipProvider>
            <App />
          </TooltipProvider>
        </HashRouter>
      </ThemeProvider>
    </React.StrictMode>,
  )
}

void bootstrap().catch((error: unknown) => {
  console.error('[i18n] 初始化失败', error)
})
