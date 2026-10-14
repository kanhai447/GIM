import type { App, Component } from 'vue'
import { describe, expect, it, vi } from 'vitest'

import { registerArcoComponents } from '@/ui/arco'

describe('Arco component registration', () => {
  it('registers the A-prefixed names used by Vue templates', () => {
    const registered = new Map<string, Component>()
    const component = vi.fn((name: string, value: Component) => {
      registered.set(name, value)
      return {} as App
    })

    registerArcoComponents({ component } as unknown as App)

    expect(registered).toEqual(expect.objectContaining({}))
    expect([...registered.keys()]).toEqual(expect.arrayContaining([
      'ACard',
      'AForm',
      'AFormItem',
      'AInput',
      'AInputPassword',
      'AButton',
    ]))
    expect(component).not.toHaveBeenCalledWith('Form', expect.anything())
  })
})
