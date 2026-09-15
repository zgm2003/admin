import { createApp } from 'vue'
import 'element-plus/theme-chalk/dark/css-vars.css'
import 'element-plus/theme-chalk/display.css'

import App from './App.vue'
import { appI18n, initializeLocale } from './i18n'
import { router } from './router'
import { installRouteLoadRecovery } from './router/routeLoadRecovery'
import { pinia } from './store'
import { installPermissionGuard } from './permission'
import './styles/index.scss'

initializeLocale()
installPermissionGuard(router)
installRouteLoadRecovery(router)

createApp(App).use(pinia).use(router).use(appI18n).mount('#app')
