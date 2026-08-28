import { createApp } from 'vue'
import ElementPlus from 'element-plus'
import zhCn from 'element-plus/es/locale/lang/zh-cn'
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

createApp(App).use(pinia).use(router).use(ElementPlus, { locale: zhCn }).use(appI18n).mount('#app')
