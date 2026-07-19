import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { RouterProvider } from 'react-router-dom'
import { router } from './router/route'
import { ThemeProvider } from './components/theme-provider'
import { ServerProvider } from './contexts/ServerContext'
import './index.css'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <ThemeProvider defaultTheme="dark" storageKey="torrentstream-theme">
      <ServerProvider>
        <RouterProvider router={router} />
      </ServerProvider>
    </ThemeProvider>
  </StrictMode>,
)
