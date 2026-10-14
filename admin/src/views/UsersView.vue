<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { Message, Modal } from '@arco-design/web-vue'
import { adminApi, UserStatus, type AdminUserRow } from '@/api/admin'
import { PublicApiError } from '@/api/client'
import { formatDate } from '@/utils/format'
import { applyUserStatusAction, userStatusActionText, userStatusText } from '@/views/userStatusAction'

const rows=ref<AdminUserRow[]>([]),loading=ref(false),hasMore=ref(false),nextBeforeId=ref(0)
const statusUpdating=reactive(new Set<number>())
const filters=reactive({search:'',status:0})
function report(error:unknown){Message.error(error instanceof PublicApiError?error.message:'请求失败')}
async function load(append=false){loading.value=true;try{const page=await adminApi.users({beforeId:append?nextBeforeId.value:undefined,limit:50,search:filters.search,status:filters.status});rows.value=append?[...rows.value,...page.list]:page.list;hasMore.value=page.hasMore;nextBeforeId.value=page.nextBeforeId}catch(error){report(error)}finally{loading.value=false}}
async function save(row:AdminUserRow){try{await adminApi.updateRestrictions({userId:row.id,curtailChat:row.curtailChat,curtailAddUser:row.curtailAddUser,curtailCreateGroup:row.curtailCreateGroup,curtailInGroupChat:row.curtailInGroupChat});Message.success('限制策略已更新')}catch(error){report(error);await load()}}
function changeStatus(row:AdminUserRow){const action=userStatusActionText(row.status);Modal.warning({title:`${action}用户`,content:`确认${action} ${row.nickname}（${row.account}）？${row.status===UserStatus.Active?'历史消息与审计记录将保留。':''}`,hideCancel:false,onOk:async()=>{statusUpdating.add(row.id);try{const status=await applyUserStatusAction(row,adminApi);Message.success(status===UserStatus.Active?'用户已启用':'用户已禁用')}catch(error){report(error)}finally{statusUpdating.delete(row.id)}}})}
onMounted(()=>load())
</script>
<template>
  <section class="page-stack">
    <a-card :bordered="false"><div class="toolbar"><div><h2>用户管理</h2><p>账号、在线状态、消息/群统计与用户限制策略。</p></div><a-space><a-input v-model="filters.search" allow-clear placeholder="账号或昵称" @press-enter="load()"/><a-select v-model="filters.status" style="width:130px"><a-option :value="0">全部状态</a-option><a-option :value="UserStatus.Active">启用</a-option><a-option :value="UserStatus.Disabled">禁用</a-option></a-select><a-button type="primary" @click="load()">查询</a-button></a-space></div></a-card>
    <a-card :bordered="false"><a-table row-key="id" :loading="loading" :data="rows" :pagination="false" :scroll="{x:1500}">
      <template #columns><a-table-column title="用户" :width="210"><template #cell="{record}"><div class="identity"><a-avatar :image-url="record.avatar||undefined">{{record.nickname.slice(0,1)}}</a-avatar><div><strong>{{record.nickname}}</strong><span>#{{record.id}} · {{record.account}}</span></div></div></template></a-table-column><a-table-column title="状态" :width="105"><template #cell="{record}"><a-tag :color="record.status===UserStatus.Active?'green':'gray'">{{userStatusText(record.status)}}</a-tag><a-badge :status="record.isOnline?'success':'normal'" :text="record.isOnline?'在线':'离线'"/></template></a-table-column><a-table-column title="消息 / 群" :width="140"><template #cell="{record}">{{record.sendMessageCount}} 条<br/>{{record.groupCount}} 群 / 管理 {{record.groupAdminCount}}</template></a-table-column><a-table-column title="限制策略" :width="470"><template #cell="{record}"><a-space wrap><a-checkbox v-model="record.curtailChat">聊天</a-checkbox><a-checkbox v-model="record.curtailAddUser">加好友</a-checkbox><a-checkbox v-model="record.curtailCreateGroup">建群</a-checkbox><a-checkbox v-model="record.curtailInGroupChat">进群</a-checkbox><a-button size="mini" @click="save(record)">保存</a-button></a-space></template></a-table-column><a-table-column title="注册时间" :width="180"><template #cell="{record}">{{formatDate(record.createdAt)}}</template></a-table-column><a-table-column title="操作" :width="100" fixed="right"><template #cell="{record}"><a-button :status="record.status===UserStatus.Active?'danger':'normal'" size="small" :loading="statusUpdating.has(record.id)" @click="changeStatus(record)">{{userStatusActionText(record.status)}}</a-button></template></a-table-column></template>
    </a-table><div class="load-more"><a-button v-if="hasMore" :loading="loading" @click="load(true)">加载更多</a-button></div></a-card>
  </section>
</template>
