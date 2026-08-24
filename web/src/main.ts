import { createApp } from 'vue'
import ElementPlus from 'element-plus'
import 'element-plus/dist/index.css'
import 'element-plus/theme-chalk/dark/css-vars.css'

import App from './App.vue'
import { appI18n, initializeLocale } from './i18n'
import { router } from './router'
import { pinia } from './store'
import { installPermissionGuard } from './permission'
import './styles/index.scss'

initializeLocale()
installPermissionGuard(router)

createApp(App).use(pinia).use(router).use(ElementPlus).use(appI18n).mount('#app')
