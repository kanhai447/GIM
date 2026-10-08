import { createApp } from 'vue'
import { createPinia } from 'pinia'
import {
  Alert,
  Avatar,
  Button,
  Card,
  Empty,
  Form,
  FormItem,
  Input,
  InputPassword,
  Layout,
  LayoutContent,
  LayoutHeader,
  LayoutSider,
  Menu,
  MenuItem,
  Result,
  Space,
  Tag,
} from '@arco-design/web-vue'
import '@arco-design/web-vue/dist/arco.css'

import App from '@/App.vue'
import router from '@/router'
import '@/styles/base.css'

const app = createApp(App)
for (const component of [
  Alert,
  Avatar,
  Button,
  Card,
  Empty,
  Form,
  FormItem,
  Input,
  InputPassword,
  Layout,
  LayoutContent,
  LayoutHeader,
  LayoutSider,
  Menu,
  MenuItem,
  Result,
  Space,
  Tag,
]) {
  app.component(component.name!, component)
}

app.use(createPinia()).use(router).mount('#app')
