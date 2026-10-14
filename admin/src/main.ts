import { createApp } from 'vue'
import { createPinia } from 'pinia'
import '@arco-design/web-vue/dist/arco.css'

import App from '@/App.vue'
import router from '@/router'
import '@/styles/base.css'
import { registerArcoComponents } from '@/ui/arco'

const app = createApp(App)
registerArcoComponents(app)
app.use(createPinia()).use(router).mount('#app')
