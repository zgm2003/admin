import React from 'react'
import { createRoot } from 'react-dom/client'
import { RouterProvider } from 'react-router-dom'
import 'antd/dist/reset.css'

import { AppProviders } from '@/components/layout/app-providers'
import '@/i18n'
import { router } from '@/router'
import '@/styles/globals.css'
import '@/styles/index.css'

document.body.style.fontFamily = '"SF Pro Display","SF Pro Text","PingFang SC","Microsoft YaHei","Helvetica Neue",sans-serif'

createRoot(document.getElementById('root')!).render(<React.StrictMode><AppProviders><RouterProvider router={router} /></AppProviders></React.StrictMode>)
