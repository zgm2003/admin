import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'

import { CanvasRouter } from '@/router'
import '@/styles/index.css'

createRoot(document.getElementById('root') as HTMLElement).render(<StrictMode><CanvasRouter /></StrictMode>)
