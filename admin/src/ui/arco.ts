import type { App, Component } from 'vue'
import {
  Alert,
  Avatar,
  Badge,
  Button,
  Card,
  Checkbox,
  Descriptions,
  DescriptionsItem,
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
  Modal,
  Option,
  Result,
  Select,
  Space,
  Switch,
  Table,
  TableColumn,
  Tag,
  Textarea,
} from '@arco-design/web-vue'

const components: Component[] = [
  Alert,
  Avatar,
  Badge,
  Button,
  Card,
  Checkbox,
  Descriptions,
  DescriptionsItem,
  Empty,
  Form,
  FormItem,
  Input,
  InputPassword,
  Modal,
  Option,
  Select,
  Layout,
  LayoutContent,
  LayoutHeader,
  LayoutSider,
  Menu,
  MenuItem,
  Result,
  Space,
  Switch,
  Table,
  TableColumn,
  Tag,
  Textarea,
]

export function registerArcoComponents(app: App): void {
  for (const component of components) {
    const name = (component as Component & { name?: string }).name
    if (!name) throw new Error('Arco component is missing its runtime name')
    app.component(`A${name}`, component)
  }
}
