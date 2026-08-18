<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { PieChart } from 'echarts/charts'
import { TooltipComponent } from 'echarts/components'
import * as echarts from 'echarts/core'
import type { ECharts } from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'

type StatusState = 'checking' | 'up' | 'error'

const props = defineProps<{
  api: StatusState
  postgresql: StatusState
  redis: StatusState
}>()

echarts.use([PieChart, TooltipComponent, CanvasRenderer])

const chartElement = ref<HTMLDivElement>()
let chart: ECharts | undefined
let themeObserver: MutationObserver | undefined

function themeColor(name: string, fallback: string): string {
  const value = getComputedStyle(document.documentElement).getPropertyValue(name).trim()
  return value === '' ? fallback : value
}

function statusColor(status: StatusState): string {
  if (status === 'up') return themeColor('--el-color-success', '#67c23a')
  if (status === 'error') return themeColor('--el-color-danger', '#f56c6c')
  return themeColor('--el-color-warning', '#e6a23c')
}

function renderChart(): void {
  if (!chartElement.value) return
  chart ??= echarts.init(chartElement.value)
  const compact = chartElement.value.clientWidth <= 320
  const animationEnabled = !window.matchMedia?.('(prefers-reduced-motion: reduce)').matches
  chart.setOption({
    animation: animationEnabled,
    animationDuration: animationEnabled ? 300 : 0,
    tooltip: { trigger: 'item' },
    series: [
      {
        type: 'pie',
        radius: compact ? ['52%', '74%'] : ['58%', '82%'],
        avoidLabelOverlap: true,
        itemStyle: { borderColor: themeColor('--el-bg-color', '#ffffff'), borderWidth: 3 },
        label: {
          color: themeColor('--el-text-color-regular', '#606266'),
          fontSize: 12,
          formatter: compact ? ({ name }: { name: string }) => (name === 'PostgreSQL' ? 'PG' : name) : undefined,
        },
        labelLine: compact ? { length: 8, length2: 4 } : undefined,
        data: [
          { value: 1, name: 'API', itemStyle: { color: statusColor(props.api) } },
          { value: 1, name: 'PostgreSQL', itemStyle: { color: statusColor(props.postgresql) } },
          { value: 1, name: 'Redis', itemStyle: { color: statusColor(props.redis) } },
        ],
      },
    ],
  })
}

function resizeChart(): void {
  chart?.resize()
  renderChart()
}

watch(() => [props.api, props.postgresql, props.redis], renderChart)

onMounted(() => {
  renderChart()
  themeObserver = new MutationObserver(renderChart)
  themeObserver.observe(document.documentElement, { attributes: true, attributeFilter: ['class'] })
  window.addEventListener('resize', resizeChart)
})

onBeforeUnmount(() => {
  window.removeEventListener('resize', resizeChart)
  themeObserver?.disconnect()
  chart?.dispose()
})
</script>

<template>
  <div
    ref="chartElement"
    class="readiness-chart"
    role="img"
    aria-label="API、PostgreSQL 和 Redis 运行状态"
  />
</template>

<style scoped>
.readiness-chart {
  width: 100%;
  height: 220px;
}
</style>
