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
    expect(harness.download).not.toHaveBeenCalled()
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

  it('renders a file card and downloads through the authenticated API', async () => {
    harness.download.mockResolvedValue(new Blob(['pdf'], { type: 'application/pdf' }))
    const click = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {})
    const wrapper = mount(MessageBubble, {
      props: { msg: { type: 3, fileMsg: { fileID: 11, fileName: 'report.pdf', size: 2048, mimeType: 'application/pdf' } } },
    })
    expect(wrapper.text()).toContain('report.pdf')
    expect(wrapper.text()).toContain('2.0 KB')
    await wrapper.get('.file-download').trigger('click')
    await vi.waitFor(() => expect(click).toHaveBeenCalledOnce())
    expect(harness.download).toHaveBeenCalledWith(11)
    click.mockRestore()
  })

  it('shows retry when a file download fails', async () => {
    harness.download.mockRejectedValueOnce(new Error('network')).mockResolvedValueOnce(new Blob(['retry']))
    vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {})
    const wrapper = mount(MessageBubble, {
      props: { msg: { type: 3, fileMsg: { fileID: 12, fileName: 'notes.txt', size: 128 } } },
    })
    await wrapper.get('.file-download').trigger('click')
    await vi.waitFor(() => expect(wrapper.text()).toContain('重试下载'))
    await wrapper.get('.file-download').trigger('click')
    await vi.waitFor(() => expect(wrapper.text()).toContain('下载'))
    expect(harness.download).toHaveBeenCalledTimes(2)
  })

  it('renders an unsupported placeholder for unknown message types', () => {
    const wrapper = mount(MessageBubble, { props: { msg: { type: 4 } } })
    expect(wrapper.text()).toContain('暂不支持的消息')
  })
})
