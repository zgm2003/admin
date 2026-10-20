// From web, with a Vite dev server: playwright-cli -s=dialog-qa run-code --filename=tests/browser/appDialogGeometry.mjs
/* global window, requestAnimationFrame, innerWidth, innerHeight, getComputedStyle */
// eslint-disable-next-line @typescript-eslint/no-unused-vars -- playwright-cli evaluates this function directly.
async function appDialogGeometry(page) {
  await page.goto('http://localhost:16300/tests/browser/fixtures/appDialog.html')
  await page.locator('.app-dialog').waitFor()
  await page.locator('.dialog-fade-enter-active').waitFor({ state: 'detached' })
  const results = []
  for (const [width, height] of [
    [1280, 720],
    [1920, 1080],
    [390, 844],
    [320, 568],
    [1280, 420],
  ]) {
    await page.setViewportSize({ width, height })
    for (const count of [2, 70]) {
      await page.evaluate(
        async ({ count }) => {
          window.dialogQA.count.value = count
          await new Promise(requestAnimationFrame)
        },
        { count },
      )
      const result = await page.locator('.app-dialog').evaluate((dialog) => {
        const rect = dialog.getBoundingClientRect()
        const scroll = dialog.querySelector('.el-scrollbar__wrap')
        const content = dialog.querySelector('.app-dialog__content').getBoundingClientRect()
        return {
          viewport: [innerWidth, innerHeight],
          height: rect.height,
          centerX: rect.x + rect.width / 2,
          centerY: rect.y + rect.height / 2,
          scrollHeight: scroll.scrollHeight,
          clientHeight: scroll.clientHeight,
          footerBottom: dialog.querySelector('.el-dialog__footer').getBoundingClientRect().bottom,
          contentRight: content.right,
          dialogRight: rect.right,
        }
      })
      if (
        Math.abs(result.centerX - width / 2) > 1 ||
        Math.abs(result.centerY - height / 2) > 1 ||
        result.height > height * 0.9 + 1 ||
        result.footerBottom > height ||
        result.contentRight > result.dialogRight ||
        (count === 70 && result.scrollHeight <= result.clientHeight) ||
        (count === 2 && result.height > 300)
      )
        throw new Error(JSON.stringify(result))
      results.push(result)
    }
  }
  await page.evaluate(async () => {
    window.dialogQA.header.value = false
    window.dialogQA.footer.value = false
    await new Promise(requestAnimationFrame)
  })
  const hidden = await page.locator('.app-dialog').evaluate((dialog) => ({
    header: getComputedStyle(dialog.querySelector('.el-dialog__header')).display,
    footer: Boolean(dialog.querySelector('.el-dialog__footer')),
    height: dialog.getBoundingClientRect().height,
  }))
  if (hidden.header !== 'none' || hidden.footer || hidden.height > 378)
    throw new Error(JSON.stringify(hidden))
  await page.evaluate(async () => {
    window.dialogQA.count.value = 2
    window.dialogQA.center.value = false
    await new Promise(requestAnimationFrame)
  })
  const top = await page
    .locator('.app-dialog')
    .evaluate((dialog) => dialog.getBoundingClientRect().top)
  if (Math.abs(top - 21) > 1) throw new Error(`uncentered top=${top}`)
  return { results, hidden, uncenteredTop: top }
}
