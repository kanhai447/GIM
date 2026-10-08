import { LineChart } from 'echarts/charts'
import { GridComponent, TooltipComponent } from 'echarts/components'
import { init, use } from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'

export function loadChartRuntime() {
  use([LineChart, GridComponent, TooltipComponent, CanvasRenderer])
  return { init }
}
