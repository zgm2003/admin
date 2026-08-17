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

const colors: Record<StatusState, string> = {
  checking: '#C97822',
  up: '#16845B',
  error: '#C33C3C',
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
        itemStyle: { borderColor: '#ffffff', borderWidth: 3 },
        label: {
          color: '#44515d',
          fontSize: 12,
          formatter: compact ? ({ name }: { name: string }) => (name === 'PostgreSQL' ? 'PG' : name) : undefined,
        },
        labelLine: compact ? { length: 8, length2: 4 } : undefined,
        data: [
          { value: 1, name: 'API', itemStyle: { color: colors[props.api] } },
          { value: 1, name: 'PostgreSQL', itemStyle: { color: colors[props.postgresql] } },
          { value: 1, name: 'Redis', itemStyle: { color: colors[props.redis] } },
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
  window.addEventListener('resize', resizeChart)
})

onBeforeUnmount(() => {
  window.removeEventListener('resize', resizeChart)
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
