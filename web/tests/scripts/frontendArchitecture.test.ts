// @vitest-environment node
import { spawnSync } from 'node:child_process'
import { cpSync, mkdirSync, mkdtempSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { dirname, join, resolve } from 'node:path'
import { afterEach, describe, expect, it } from 'vitest'

const fixtures: string[] = []

afterEach(() => {
  for (const fixture of fixtures.splice(0)) rmSync(fixture, { recursive: true, force: true })
})

function check(files: Readonly<Record<string, string>>) {
  const fixture = mkdtempSync(join(tmpdir(), 'admin-frontend-architecture-'))
  fixtures.push(fixture)
  mkdirSync(join(fixture, 'scripts'), { recursive: true })
  cpSync(resolve('scripts/check-frontend-architecture.mjs'), join(fixture, 'scripts/check.mjs'))
  writeFileSync(
    join(fixture, 'scripts/frontend-architecture-baseline.mjs'),
    'export const architectureBaseline = Object.freeze([])\n',
  )
  for (const [path, content] of Object.entries(files)) {
    const file = join(fixture, path)
    mkdirSync(dirname(file), { recursive: true })
    writeFileSync(file, content)
  }
  const result = spawnSync(process.execPath, [join(fixture, 'scripts/check.mjs')], {
    encoding: 'utf8',
  })
  if (result.error) throw result.error
  return { status: result.status, output: result.stdout + result.stderr }
}

describe('frontend architecture CLI', () => {
  it('accepts pages above 500 lines and components above 400 lines without forced splitting', () => {
    const sfc = `<template><section class="fixture" /></template>\n${'\n'.repeat(510)}<style lang="scss" scoped>.fixture { color: var(--el-text-color-primary); }</style>`
    expect(
      check({ 'src/views/fixture/index.vue': sfc, 'src/components/Fixture/index.vue': sfc }),
    ).toMatchObject({ status: 0 })
  })

  it('accepts ordinary SCSS without artificial Sass features', () => {
    expect(
      check({
        'src/components/Fixture/index.vue':
          '<template><section /></template><style scoped lang="scss">section { display: flex; }</style>',
        'src/styles/fixture.scss': ':root { --fixture-color: red; }',
      }),
    ).toMatchObject({ status: 0 })
  })

  it('does not require runtime business DTO parsing for typed API calls', () => {
    expect(
      check({ 'src/api/fixture.ts': "export const get = () => request<Fixture>('/fixture')" }),
    ).toMatchObject({ status: 0 })
  })

  it('rejects owned CSS files', () => {
    const result = check({ 'src/styles/fixture.css': '.fixture { display: flex; }' })
    expect(result.status).toBe(1)
    expect(result.output).toContain('NEW owned-css src/styles/fixture.css')
  })

  it.each([
    '<style scoped>.fixture { display: flex; }</style>',
    '<style lang="css" scoped>.fixture { display: flex; }</style>',
    '<style src="./fixture.scss" scoped></style>',
    '<style lang="scss" src="./fixture.css" scoped></style>',
  ])('rejects an owned Vue style block without SCSS: %s', (style) => {
    const result = check({ 'src/components/Fixture/index.vue': style })
    expect(result.status).toBe(1)
    expect(result.output).toContain('NEW vue-style-scss src/components/Fixture/index.vue')
  })

  it('accepts SCSS inline and external styles while leaving third-party CSS imports alone', () => {
    expect(
      check({
        'src/main.ts': "import 'element-plus/theme-chalk/dark/css-vars.css'",
        'src/components/Fixture/index.vue':
          '<style scoped lang=\'scss\'>.fixture { display: flex; }</style><style scoped lang="scss" src="./fixture.scss"></style>',
        'src/components/Fixture/fixture.scss': '.fixture { display: flex; }',
      }),
    ).toMatchObject({ status: 0 })
  })

  it.each([
    ['src/components/Fixture/index.vue', '<el-select></el-select>', 'raw-el-select'],
    ['src/api/fixture.ts', ['export const value = input as', 'any'].join(' '), 'unsafe-any'],
    ['src/api/fixture.ts', 'export const items = input.items ?? []', 'required-array-fallback'],
    ['src/router/index.ts', "import.meta.glob('/src/views/**/index.vue')", 'broad-view-glob'],
    ['src/router/index.ts', 'const staticPageBinding = {}', 'static-business-route'],
    ['src/router/index.ts', "const path = 'account/profile'", 'page-path-contract'],
    ['src/utils/request.ts', 'export const request = {}', 'request-error-notification-owner'],
  ])('preserves the %s boundary (%s)', (file, content, rule) => {
    const result = check({ [file]: content })
    expect(result.status).toBe(1)
    expect(result.output).toContain(`NEW ${rule} ${file}`)
  })
})
