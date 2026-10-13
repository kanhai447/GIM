// @vitest-environment happy-dom

import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import FileUploadPanel from '@/components/file/FileUploadPanel.vue'
import UserFileList from '@/components/file/UserFileList.vue'
import type { UserFileRecord } from '@/types/file'

const file: UserFileRecord = {
  id: 7, uid: '00000000-0000-4000-8000-000000000007', fileName: '<img src=x onerror=alert(1)>.txt', size: 12,
  mimeType: 'text/plain', sha256: 'a'.repeat(64), url: '/api/file/7', createdAt: '2026-10-13T00:00:00Z',
}

const global = {
  stubs: {
    ElButton: { template: '<button @click="$emit(\'click\')"><slot /></button>' },
    ElProgress: { props: ['percentage', 'status'], template: '<div class="progress">{{ percentage }}</div>' },
  },
}

describe('file components', () => {
  it('renders malicious-looking filenames as literal text and emits owned actions', async () => {
    const wrapper = mount(UserFileList, { props: { files: [file], loading: false, loadingMore: false, hasMore: false }, global })
    expect(wrapper.get('.file-name').text()).toBe(file.fileName)
    expect(wrapper.find('.file-name img').exists()).toBe(false)
    const buttons = wrapper.findAll('button')
    await buttons[0]!.trigger('click')
    await buttons[1]!.trigger('click')
    expect(wrapper.emitted('download')?.[0]).toEqual([file])
    expect(wrapper.emitted('remove')?.[0]).toEqual([file])
  })

  it('shows independent upload task states and accepts multiple selection', () => {
    const wrapper = mount(FileUploadPanel, {
      props: { tasks: [
        { id: '1', fileName: 'sending.bin', size: 10, progress: 55, status: 'uploading' },
        { id: '2', fileName: 'failed.bin', size: 20, progress: 0, status: 'failed', errorMessage: '上传失败' },
      ] },
      global,
    })
    expect(wrapper.findAll('.upload-task')).toHaveLength(2)
    expect(wrapper.text()).toContain('55')
    expect(wrapper.text()).toContain('上传失败')
    expect(wrapper.get('input[type="file"]').attributes('multiple')).toBeDefined()
  })
})
