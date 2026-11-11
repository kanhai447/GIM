// @vitest-environment happy-dom

import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import FriendList from '@/components/friend/FriendList.vue'
import FriendRequestList from '@/components/friend/FriendRequestList.vue'
import type { FriendRequest, FriendUser } from '@/types/friend'

const global = {
  stubs: {
    ElButton: { template: '<button @click="$emit(\'click\', $event)"><slot /></button>' },
  },
}

const hostileName = '<img src=x onerror=alert(1)>'
const friend: FriendUser = {
  userID: 8, account: 'alice', nickname: hostileName, avatar: '', abstract: '',
  remark: '', isOnline: true, createdAt: '2026-11-11T00:00:00Z',
}

const request: FriendRequest = {
  id: 5, requester: { userID: 9, nickname: hostileName, avatar: '' }, receiverID: 1, status: 0,
  message: '<script>alert(1)</script>', createdAt: '2026-11-11T00:00:00Z', updatedAt: '2026-11-11T00:00:00Z',
}

describe('friend components', () => {
  it('renders hostile nicknames as literal text and reflects presence', () => {
    const wrapper = mount(FriendList, { props: { friends: [friend], loading: false, selectedID: null }, global })
    expect(wrapper.get('.friend-name').text()).toBe(hostileName)
    expect(wrapper.find('.friend-name img').exists()).toBe(false)
    expect(wrapper.get('.online-dot').classes()).toContain('online')
    expect(wrapper.text()).toContain('在线')

    const offline = mount(FriendList, { props: { friends: [{ ...friend, isOnline: false }], loading: false, selectedID: 8 }, global })
    expect(offline.get('.online-dot').classes()).not.toContain('online')
    expect(offline.get('.friend-row').classes()).toContain('selected')
  })

  it('emits select, chat and remove from the friend row', async () => {
    const wrapper = mount(FriendList, { props: { friends: [friend], loading: false, selectedID: null }, global })
    await wrapper.get('.friend-row').trigger('click')
    expect(wrapper.emitted('select')?.[0]).toEqual([friend])

    const buttons = wrapper.findAll('button')
    await buttons[0]!.trigger('click')
    await buttons[1]!.trigger('click')
    expect(wrapper.emitted('chat')?.[0]).toEqual([friend])
    expect(wrapper.emitted('remove')?.[0]).toEqual([friend])
  })

  it('shows the empty state before any friend exists', () => {
    const wrapper = mount(FriendList, { props: { friends: [], loading: false, selectedID: null }, global })
    expect(wrapper.text()).toContain('还没有好友')
  })

  it('renders requests safely and emits accept/reject', async () => {
    const wrapper = mount(FriendRequestList, { props: { requests: [request], loading: false }, global })
    expect(wrapper.get('.request-name').text()).toBe(hostileName)
    expect(wrapper.find('.request-name img').exists()).toBe(false)
    expect(wrapper.get('.request-message').text()).toBe('<script>alert(1)</script>')
    expect(wrapper.find('.request-message script').exists()).toBe(false)

    const buttons = wrapper.findAll('button')
    await buttons[0]!.trigger('click')
    await buttons[1]!.trigger('click')
    expect(wrapper.emitted('accept')?.[0]).toEqual([5])
    expect(wrapper.emitted('reject')?.[0]).toEqual([5])
  })

  it('shows the empty state when no request is pending', () => {
    const wrapper = mount(FriendRequestList, { props: { requests: [], loading: false }, global })
    expect(wrapper.text()).toContain('暂无待处理申请')
  })
})
