import React from 'react'
import ReactDOM from 'react-dom/client'
import { ThemeProvider } from 'next-themes'
import { HashRouter } from 'react-router'
import { TooltipProvider } from '@/components/ui/tooltip'
import { i18nReady } from '@/i18n'
import App from './App'
import './index.css'

void i18nReady.then(() => {
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
})
