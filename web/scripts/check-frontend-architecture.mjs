import { existsSync, readFileSync, readdirSync } from 'node:fs'
import { join, relative, sep } from 'node:path'
import { fileURLToPath } from 'node:url'

const root = fileURLToPath(new URL('..', import.meta.url))
const src = join(root, 'src')
const tests = join(root, 'tests')
const findings = []

const toProjectPath = (file) => relative(root, file).split(sep).join('/')
const add = (rule, file, detail) => findings.push({ rule, file: toProjectPath(file), detail })
const walk = (dir) => {
  if (!existsSync(dir)) return []
  const result = []
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const file = join(dir, entry.name)
    if (entry.isDirectory()) result.push(...walk(file))
    else result.push(file)
  }
  return result
}

for (const file of [...walk(src), ...walk(tests)]) {
  const projectPath = toProjectPath(file)
  const content = readFileSync(file, 'utf8')
  if (projectPath.startsWith('src/') && file.endsWith('.vue')) {
    const lineCount = content.split(/\r?\n/).length
    const isPage =
      (projectPath.startsWith('src/views/') && !projectPath.includes('/components/')) ||
      projectPath === 'src/layout/index.vue'
    const isComponent =
      projectPath.startsWith('src/components/') ||
      projectPath.startsWith('src/layout/components/') ||
      projectPath.includes('/components/')
    if (!file.endsWith('index.vue') && !projectPath.endsWith('App.vue')) {
      add('vue-component-path', file, 'Vue 文件必须位于目录/index.vue')
    }
    if (projectPath.includes('/src/index.vue')) {
      add('component-src-directory', file, '公共组件不应存在中间 src 目录')
    }
    if (
      content.includes('lang="scss"') &&
      !/\$[A-Za-z_-]+|@mixin|@include|@function|&:/.test(content)
    ) {
      add('unnecessary-scss', file, 'SCSS 未使用变量、mixin、函数或浅层嵌套')
    }
    if (/<el-row\b[\s\S]*?<el-col\b[^>]*:?(?:span|:span)=?["']?24/.test(content)) {
      add('meaningless-grid-wrapper', file, '疑似单列 24 栅格包裹')
    }
    if ((isPage && lineCount > 500) || (isComponent && lineCount > 400)) {
      add('oversized-sfc', file, `SFC 共 ${lineCount} 行，超过强制拆分阈值`)
    }
    if (
      content.includes('<el-table') &&
      projectPath !== 'src/components/AppTable/index.vue' &&
      !content.includes('AppTable exception:')
    ) {
      add('raw-el-table', file, '原生 el-table 必须使用 AppTable 或声明专用组件例外')
    }
  }
  if (/from\s+['"][^'"]*\.ts['"]/.test(content))
    add('explicit-ts-import', file, 'import 不应带 .ts 后缀')
  if (/from\s+['"]\.\.\//.test(content))
    add('cross-module-relative-import', file, '跨模块 import 应使用 @/* 别名')
  if (/\bany\[\]|\bas any\b|Record<[^>]*,\s*any>|@ts-ignore/.test(content)) {
    add('unsafe-any', file, '业务代码存在未约束 any')
  }
  if (projectPath.startsWith('src/api/') && /request<(?!unknown\b)/.test(content)) {
    add('api-unparsed-response', file, 'API 请求应使用 request<unknown>() 并在模块边界解析')
  }
  if (projectPath.startsWith('src/api/') && /\brequest\s*\(/.test(content)) {
    add('api-unparsed-response', file, 'API 请求不得省略 unknown 响应边界')
  }
  if (projectPath.startsWith('src/api/') && /\?\?\s*\[\]/.test(content)) {
    add('required-array-fallback', file, '必填数组不得使用 ?? [] 静默修复')
  }
  if (
    projectPath.startsWith('tests/') &&
    file.endsWith('.test.ts') &&
    (/\breadFileSync\s*\(/.test(content) || /\bsource\.(?:not\.)?toContain\s*\(/.test(content))
  ) {
    add('source-shape-test', file, '测试不得读取源码或断言私有实现形态')
  }
  if (projectPath.startsWith('src/router/') && /views\/\*\*\/index\.vue/.test(content)) {
    add('broad-view-glob', file, '动态路由不得广泛扫描页面私有组件')
  }
  if (projectPath.startsWith('src/router/') && /staticPageBinding/.test(content)) {
    add('static-business-route', file, '业务页面不得保留静态路由绑定特例')
  }
}

const requestFile = join(src, 'utils', 'request.ts')
if (
  existsSync(requestFile) &&
  /element-plus|ElNotification|ElMessage|Notification/.test(readFileSync(requestFile, 'utf8'))
) {
  add('request-ui-side-effect', requestFile, '请求层不得直接拥有 Element Plus 通知副作用')
}

const baselinePath = join(root, 'scripts', 'frontend-architecture-baseline.mjs')
const baselineSource = readFileSync(baselinePath, 'utf8')
const baseline = [...baselineSource.matchAll(/\{\s*rule:\s*'([^']+)',\s*file:\s*'([^']+)'/g)].map(
  (m) => ({ rule: m[1], file: m[2] }),
)
const key = (item) => `${item.rule}|${item.file}`
const actualKeys = new Set(findings.map(key))
const baselineKeys = new Set(baseline.map(key))
const unknown = findings.filter((item) => !baselineKeys.has(key(item)))
const stale = baseline.filter((item) => !actualKeys.has(key(item)))

if (unknown.length || stale.length) {
  for (const item of unknown) console.error(`NEW ${item.rule} ${item.file}: ${item.detail}`)
  for (const item of stale) console.error(`STALE ${item.rule} ${item.file}`)
  process.exitCode = 1
} else {
  console.log(`Architecture check passed (${findings.length} baseline findings)`)
}
