// @vitest-environment happy-dom

import { mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const harness = vi.hoisted(() => ({ download: vi.fn() }))

vi.mock('@/api/file', () => ({
  fileApi: { download: harness.download },
}))

import MessageBubble from '@/components/chat/MessageBubble.vue'

describe('MessageBubble', () => {
  beforeEach(() => {
    harness.download.mockReset()
    Object.defineProperty(URL, 'createObjectURL', { value: vi.fn(() => 'blob:gim-image'), configurable: true })
    Object.defineProperty(URL, 'revokeObjectURL', { value: vi.fn(), configurable: true })
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('renders text literally without HTML injection', () => {
    const wrapper = mount(MessageBubble, { props: { msg: { type: 1, textMsg: { content: '<b>hello</b>' } } } })
    expect(wrapper.text()).toBe('<b>hello</b>')
    expect(wrapper.find('b').exists()).toBe(false)
  })

  it('downloads an image with the authenticated API and opens a preview on click', async () => {
    harness.download.mockResolvedValue(new Blob(['image'], { type: 'image/png' }))
    const wrapper = mount(MessageBubble, {
      props: { msg: { type: 2, imageMsg: { fileID: 9, mimeType: 'image/png' } } },
    })
    await vi.waitFor(() => expect(wrapper.find('[data-testid="image-thumb"]').exists()).toBe(true))
    expect(harness.download).toHaveBeenCalledWith(9)
    expect(wrapper.get('[data-testid="image-thumb"]').attributes('src')).toBe('blob:gim-image')

    await wrapper.get('[data-testid="image-thumb"]').trigger('click')
    expect(wrapper.find('[data-testid="image-preview"]').exists()).toBe(true)
    await wrapper.get('.preview-close').trigger('click')
    expect(wrapper.find('[data-testid="image-preview"]').exists()).toBe(false)
  })

  it('shows a retry control when the download fails, then recovers', async () => {
    harness.download.mockRejectedValueOnce(new Error('network')).mockResolvedValueOnce(new Blob(['retry']))
    const wrapper = mount(MessageBubble, {
      props: { msg: { type: 2, imageMsg: { fileID: 10 } } },
    })
    await vi.waitFor(() => expect(wrapper.text()).toContain('图片加载失败'))
    await wrapper.get('.image-failed').trigger('click')
    await vi.waitFor(() => expect(wrapper.find('[data-testid="image-thumb"]').exists()).toBe(true))
    expect(harness.download).toHaveBeenCalledTimes(2)
  })

  it('renders an unsupported placeholder for unknown message types', () => {
    const wrapper = mount(MessageBubble, { props: { msg: { type: 3 } } })
    expect(wrapper.text()).toContain('暂不支持的消息')
  })
})
