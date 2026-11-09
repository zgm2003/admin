// With Vite running: playwright-cli -s=dashboard-qa run-code --filename=tests/browser/dashboardOverview.mjs
/* global window, document, innerWidth, getComputedStyle, requestAnimationFrame */
// eslint-disable-next-line @typescript-eslint/no-unused-vars -- evaluated by playwright-cli.
async function dashboardOverview(page) {
  const errors = []
  page.on('pageerror', (error) => errors.push(error.message))
  let failed = false
  const requests = []
  await page.unroute('**/api/**')
  await page.unroute('http://localhost:16300/api/**')
  await page.route('http://localhost:16300/api/**', async (route) => {
    const path = route
      .request()
      .url()
      .replace(/^https?:\/\/[^/]+/, '')
      .split('?')[0]
    requests.push(path)
    let data
    if (path === '/api/health') data = { status: 'up' }
    else if (path === '/api/ready') data = { postgresql: 'up', redis: 'up' }
    else if (path === '/api/admin/v1/system/server')
      data = {
        sampledAt: '2026-11-06T10:00:00Z',
        hostname: 'instance-qa',
        os: 'linux',
        arch: 'amd64',
        logicalCores: 8,
        cpuPercent: 12.5,
        memory: { totalBytes: 17179869184, availableBytes: 8589934592, usedPercent: 50 },
        disk: { totalBytes: 107374182400, availableBytes: 53687091200, usedPercent: 50 },
        diskPath: '/',
        hostUptimeSeconds: 90061,
        processUptimeSeconds: 3661,
        startedAt: '2026-11-06T08:58:59Z',
        goVersion: 'go1.26.5',
        goroutines: 42,
        heapAllocBytes: 33554432,
      }
    else throw new Error(`unexpected API ${path}`)
    const fail = failed && path.endsWith('/system/server')
    await route.fulfill({
      status: fail ? 503 : 200,
      contentType: 'application/json',
      body: JSON.stringify({
        code: fail ? 10006 : 0,
        data: fail ? null : data,
        message: fail ? 'Metrics unavailable' : 'ok',
      }),
    })
  })
  await page.goto('http://localhost:16300/tests/browser/fixtures/dashboard.html')
  await page.getByTestId('server-metrics').waitFor()
  const results = []
  for (const mode of ['light', 'dark']) {
    await page.evaluate((mode) => {
      window.dashboardQA.applyTheme(mode)
      window.dashboardQA.applyPrimaryColor('#7C3AED')
    }, mode)
    for (const [width, height] of [
      [1440, 1200],
      [768, 1024],
      [390, 844],
      [320, 568],
    ]) {
      await page.setViewportSize({ width, height })
      await page.evaluate(() => new Promise(requestAnimationFrame))
      const result = await page.evaluate(() => ({
        width: innerWidth,
        documentWidth: document.documentElement.scrollWidth,
        appWidth: document.querySelector('#app').scrollWidth,
        surface: getComputedStyle(document.querySelector('.server-panel')).backgroundColor,
        accent: getComputedStyle(document.querySelector('.server-panel__icon')).color,
      }))
      if (result.documentWidth > width || result.appWidth > width)
        throw new Error(`overflow ${JSON.stringify(result)}`)
      results.push({ mode, ...result })
    }
  }
  await page.setViewportSize({ width: 1440, height: 1200 })
  await page.screenshot({ path: '.playwright-cli/dashboard-dark.png', fullPage: true })
  await page.evaluate(() => window.dashboardQA.applyTheme('light'))
  await page.screenshot({ path: '.playwright-cli/dashboard-light.png', fullPage: true })
  await page.locator('summary').click()
  if (!(await page.locator('details').evaluate((node) => node.open)))
    throw new Error('deployment disclosure failed')
  failed = true
  await page.getByTestId('server-refresh').click()
  await page.getByTestId('server-error').waitFor()
  if (await page.getByTestId('server-metrics').count()) throw new Error('stale metrics visible')
  failed = false
  await page.getByTestId('server-refresh').click()
  await page.getByTestId('server-metrics').waitFor()
  await page.evaluate(() => {
    window.dashboardQA.permission.permissionCodes = []
    window.dashboardQA.setLocale('en-US')
  })
  await page.getByTestId('server-metrics').waitFor()
  if (await page.getByTestId('operations-links').count())
    throw new Error('revoked route link remains')
  if (await page.locator('.dashboard-toolbar').count())
    throw new Error('duplicated page header remains')
  if (
    (await page.locator('.dashboard-intro .section-kicker').innerText()) !== 'Operations overview'
  )
    throw new Error('locale not applied')
  if (errors.length) throw new Error(JSON.stringify(errors))
  return {
    results,
    requests,
    errors,
    checks: [
      'responsive',
      'theme',
      'deployment disclosure',
      'failure clears metrics',
      'retry',
      'all signed-in users see metrics; operation links remain permission-scoped',
      'no duplicated page header',
      'English',
    ],
  }
}
