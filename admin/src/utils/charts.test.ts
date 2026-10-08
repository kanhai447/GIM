import { describe, expect, it } from 'vitest'

import { loadChartRuntime } from '@/utils/charts'

describe('ECharts on-demand runtime', () => {
  it('loads the registered canvas runtime without creating fake chart data', async () => {
    const runtime = await loadChartRuntime()
    expect(runtime.init).toBeTypeOf('function')
  })
})
