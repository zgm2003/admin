<!-- 组件：对话 Markdown 渲染 -->
<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import DOMPurify from 'dompurify'
import MarkdownIt from 'markdown-it'
import 'highlight.js/styles/github-dark.css'
import hljs from 'highlight.js/lib/core'
import bash from 'highlight.js/lib/languages/bash'
import css from 'highlight.js/lib/languages/css'
import go from 'highlight.js/lib/languages/go'
import javascript from 'highlight.js/lib/languages/javascript'
import json from 'highlight.js/lib/languages/json'
import python from 'highlight.js/lib/languages/python'
import sql from 'highlight.js/lib/languages/sql'
import typescript from 'highlight.js/lib/languages/typescript'
import xml from 'highlight.js/lib/languages/xml'

hljs.registerLanguage('bash', bash)
hljs.registerLanguage('shell', bash)
hljs.registerLanguage('css', css)
hljs.registerLanguage('go', go)
hljs.registerLanguage('javascript', javascript)
hljs.registerLanguage('js', javascript)
hljs.registerLanguage('json', json)
hljs.registerLanguage('python', python)
hljs.registerLanguage('sql', sql)
hljs.registerLanguage('typescript', typescript)
hljs.registerLanguage('ts', typescript)
hljs.registerLanguage('html', xml)
hljs.registerLanguage('xml', xml)

const props = defineProps<{ content: string; streaming?: boolean }>()
const root = ref<HTMLElement | null>(null)

const markdown = new MarkdownIt({
  html: false,
  linkify: true,
  breaks: true,
  highlight(code, language) {
    if (language && hljs.getLanguage(language)) {
      try {
        return hljs.highlight(code, { language, ignoreIllegals: true }).value
      } catch {
        return ''
      }
    }
    return ''
  },
})

const rendered = computed(() => {
  const source = props.content || ''
  if (source === '') return ''
  return DOMPurify.sanitize(markdown.render(source), {
    ADD_ATTR: ['target', 'rel'],
  })
})

function placeCursor(): void {
  const host = root.value
  if (host === null) return
  host.querySelector('.markdown-cursor')?.remove()
  if (!props.streaming) return

  let target: HTMLElement = host
  while (target.lastChild !== null) {
    const last = target.lastChild
    if (last.nodeType !== Node.ELEMENT_NODE) break
    const element = last as HTMLElement
    if (element.tagName === 'HR' || element.tagName === 'IMG' || element.tagName === 'BR') break
    target = element
  }
  const cursor = document.createElement('span')
  cursor.className = 'markdown-cursor'
  cursor.setAttribute('aria-hidden', 'true')
  target.appendChild(cursor)
}

watch(
  () => [props.content, props.streaming],
  () => {
    void nextTick(placeCursor)
  },
)
onMounted(() => nextTick(placeCursor))
</script>

<template>
  <div
    ref="root"
    class="markdown-content"
    :class="{ 'markdown-content--streaming': streaming }"
    v-html="rendered"
  />
</template>

<style scoped lang="scss">
.markdown-content {
  color: var(--admin-text);
  font-size: 14px;
  line-height: 1.75;
  overflow-wrap: anywhere;
}

.markdown-content :deep(p) {
  margin: 0 0 10px;
}

.markdown-content :deep(p:last-child) {
  margin-bottom: 0;
}

.markdown-content :deep(h1),
.markdown-content :deep(h2),
.markdown-content :deep(h3),
.markdown-content :deep(h4) {
  margin: 18px 0 8px;
  font-weight: 650;
  line-height: 1.35;
}

.markdown-content :deep(h1) {
  font-size: 20px;
}

.markdown-content :deep(h2) {
  font-size: 18px;
}

.markdown-content :deep(h3) {
  font-size: 16px;
}

.markdown-content :deep(ul),
.markdown-content :deep(ol) {
  margin: 6px 0 10px;
  padding-left: 22px;
}

.markdown-content :deep(li) {
  margin: 3px 0;
}

.markdown-content :deep(code) {
  padding: 2px 5px;
  border-radius: 4px;
  background: var(--admin-surface-soft);
  font-family: 'JetBrains Mono', 'SFMono-Regular', Consolas, monospace;
  font-size: 12.5px;
}

.markdown-content :deep(pre) {
  margin: 10px 0;
  padding: 12px 14px;
  overflow-x: auto;
  border: 1px solid var(--admin-border);
  border-radius: 8px;
  background: #0f172a;
}

.markdown-content :deep(pre code) {
  padding: 0;
  color: #e2e8f0;
  background: transparent;
  font-size: 12.5px;
  line-height: 1.65;
}

.markdown-content :deep(blockquote) {
  margin: 10px 0;
  padding: 8px 12px;
  border-left: 3px solid var(--el-color-primary);
  color: var(--admin-text-soft);
  background: var(--admin-surface-soft);
}

.markdown-content :deep(a) {
  color: var(--el-color-primary);
  text-decoration: none;
}

.markdown-content :deep(a:hover) {
  text-decoration: underline;
}

.markdown-content :deep(table) {
  display: block;
  width: 100%;
  margin: 10px 0;
  overflow-x: auto;
  border-collapse: collapse;
}

.markdown-content :deep(th),
.markdown-content :deep(td) {
  padding: 6px 10px;
  border: 1px solid var(--admin-border);
  text-align: left;
}

.markdown-content :deep(th) {
  background: var(--admin-surface-soft);
}

.markdown-content :deep(hr) {
  margin: 16px 0;
  border: 0;
  border-top: 1px solid var(--admin-border);
}

.markdown-content :deep(.markdown-cursor) {
  display: inline-block;
  width: 6px;
  height: 1em;
  margin-left: 2px;
  border-radius: 1px;
  background: var(--el-color-primary);
  vertical-align: -2px;
  animation: markdown-cursor 1s steps(2, start) infinite;
}

@keyframes markdown-cursor {
  50% {
    opacity: 0;
  }
}
</style>
