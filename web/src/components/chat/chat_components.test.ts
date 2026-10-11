// @vitest-environment happy-dom

import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import ChatMessageList from '@/components/chat/ChatMessageList.vue'
import ChatSessionList from '@/components/chat/ChatSessionList.vue'
import type { ChatMessageItem, ChatSession } from '@/types/chat'

function message(status: ChatMessageItem['status'], id: string): ChatMessageItem {
  return {
    localID: id,
    messageId: status === 'sent' ? Number(id) : undefined,
    sendUserID: 1,
    revUserID: 2,
    clientMsgId: `client-${id}`,
    msg: { type: 1, textMsg: { content: `message-${id}` } },
    createdAt: '2026-10-11T00:00:00Z',
    status,
    errorMessage: status === 'failed' ? '服务端确认超时，可重试' : undefined,
  }
}

describe('private chat components', () => {
  it('renders pending, sent, failed and load-older controls and emits retry', async () => {
    const wrapper = mount(ChatMessageList, {
      props: {
        messages: [message('pending', '1'), message('sent', '2'), message('failed', '3')],
        currentUserID: 1,
        hasMore: true,
        loading: false,
      },
    })
    expect(wrapper.text()).toContain('发送中')
    expect(wrapper.text()).toContain('已发送')
    expect(wrapper.text()).toContain('发送失败 · 重试')
    expect(wrapper.text()).toContain('加载更早消息')

    await wrapper.get('.failed-label').trigger('click')
    await wrapper.get('.older-row button').trigger('click')
    expect(wrapper.emitted('retry')).toEqual([['client-3']])
    expect(wrapper.emitted('loadOlder')).toHaveLength(1)
  })

  it('renders unread badge, top state and emits session selection', async () => {
    const session: ChatSession = {
      peer: { userID: 2, nickname: 'Peer', avatar: '' },
      lastMessage: {
        messageId: 8,
        sendUserID: 2,
        revUserID: 1,
        clientMsgId: 'remote-8',
        msg: { type: 1, textMsg: { content: 'unread text' } },
        createdAt: '2026-10-11T00:00:00Z',
      },
      lastMessageID: 8,
      lastMessageAt: '2026-10-11T00:00:00Z',
      lastReadMessageID: null,
      unreadCount: 7,
      isTop: true,
    }
    const wrapper = mount(ChatSessionList, {
      props: { sessions: [session], currentPeerID: null, loading: false },
      global: { stubs: { ElAvatar: { template: '<span><slot /></span>' } } },
    })
    expect(wrapper.get('.unread-badge').text()).toBe('7')
    expect(wrapper.text()).toContain('unread text')
    expect(wrapper.text()).toContain('已置顶')
    await wrapper.get('.session-item').trigger('click')
    expect(wrapper.emitted('select')).toEqual([[2]])
  })
})
