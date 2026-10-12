// @vitest-environment happy-dom

import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import GroupMessageList from '@/components/group/GroupMessageList.vue'
import GroupSidebar from '@/components/group/GroupSidebar.vue'
import type { GroupMessageViewItem, GroupSession } from '@/types/group'

function item(status: GroupMessageViewItem['status'], id: number | undefined, content = 'hello'): GroupMessageViewItem {
  return {
    localID: id ? `message:${id}` : 'pending:c1',
    messageId: id,
    groupID: 5,
    sendUserID: 1,
    clientMsgId: id ? `c${id}` : 'c1',
    msg: { type: 1, textMsg: { content } },
    createdAt: '2026-10-13T00:00:00Z',
    status,
    retryable: true,
  }
}

describe('group components', () => {
  it('renders pending, sent and failed states and emits retry', async () => {
    const wrapper = mount(GroupMessageList, {
      props: { messages: [item('pending', undefined), item('sent', 2), item('failed', 3)], currentUserID: 1, hasMore: true, loading: false, senderName: () => '我' },
    })
    expect(wrapper.text()).toContain('发送中')
    expect(wrapper.text()).toContain('已发送')
    expect(wrapper.text()).toContain('发送失败 · 重试')
    await wrapper.get('button.failed').trigger('click')
    expect(wrapper.emitted('retry')).toEqual([['c3']])
  })

  it('renders HTML-looking content as literal text', () => {
    const wrapper = mount(GroupMessageList, {
      props: { messages: [item('sent', 4, '<b>hello group</b>')], currentUserID: 1, hasMore: false, loading: false, senderName: () => '我' },
    })
    expect(wrapper.get('.bubble').text()).toBe('<b>hello group</b>')
    expect(wrapper.find('.bubble b').exists()).toBe(false)
  })

  it('distinguishes sessions from my groups and shows unread/top', async () => {
    const session: GroupSession = {
      group: { groupID: 5, name: 'Session Group', avatar: '' },
      lastMessage: { messageId: 8, groupID: 5, sendUserID: 2, clientMsgId: 'c8', msg: { type: 1, textMsg: { content: 'preview' } }, createdAt: '2026-10-13T00:00:00Z' },
      lastMessageID: 8,
      lastMessageAt: '2026-10-13T00:00:00Z',
      lastReadMessageID: null,
      unreadCount: 3,
      isTop: true,
    }
    const wrapper = mount(GroupSidebar, {
      props: {
        sessions: [session],
        groups: [{ groupId: 6, title: 'My Group', abstract: '', avatar: '', ownerId: 1, status: 1, role: 1, memberCount: 3, isSearch: true, joinPolicy: 2, isProhibition: false, memberLimit: 100 }],
        currentGroupID: null,
        loading: false,
      },
      global: { stubs: { ElAvatar: { template: '<span><slot /></span>' } } },
    })
    expect(wrapper.text()).toContain('群会话')
    expect(wrapper.text()).toContain('我的群')
    expect(wrapper.text()).toContain('preview')
    expect(wrapper.text()).toContain('置顶')
    expect(wrapper.get('.unread').text()).toBe('3')
    await wrapper.get('.list-item').trigger('click')
    expect(wrapper.emitted('select')).toEqual([[5]])
  })
})
