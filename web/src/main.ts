import { createApp } from 'vue'
import { createPinia } from 'pinia'
import {
  ElAside,
  ElAvatar,
  ElButton,
  ElCard,
  ElContainer,
  ElDescriptions,
  ElDescriptionsItem,
  ElEmpty,
  ElForm,
  ElFormItem,
  ElHeader,
  ElInput,
  ElMain,
  ElMenu,
  ElMenuItem,
  ElTag,
} from 'element-plus'
import 'element-plus/dist/index.css'

import App from '@/App.vue'
import router from '@/router'
import '@/styles/base.css'

const app = createApp(App)
for (const component of [
  ElAside,
  ElAvatar,
  ElButton,
  ElCard,
  ElContainer,
  ElDescriptions,
  ElDescriptionsItem,
  ElEmpty,
  ElForm,
  ElFormItem,
  ElHeader,
  ElInput,
  ElMain,
  ElMenu,
  ElMenuItem,
  ElTag,
]) {
  app.component(component.name!, component)
}

app.use(createPinia()).use(router).mount('#app')
