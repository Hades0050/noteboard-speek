import { expect, test } from '@playwright/test'
import { create, fromBinary, toBinary } from '@bufbuild/protobuf'
import { GetBoardResponseSchema, ReorderRequestSchema, type ReorderRequest } from '../app/gen/noteboard/v1/noteboard_pb'

test('a drag across columns saves only after release and never duplicates the card', async ({ page }) => {
  let board = create(GetBoardResponseSchema, {
    columns: [
      { id: 1n, title: 'Ideas', cards: [{ id: 10n, columnId: 1n, title: 'Move me' }] },
      { id: 2n, title: 'Working' },
      { id: 3n, title: 'Done' },
    ],
  })
  const orders: ReorderRequest[] = []
  const errors: string[] = []
  page.on('pageerror', error => errors.push(error.message))
  page.on('console', message => { if (message.type() === 'error') errors.push(message.text()) })
  await page.route('**/api/rpc/**', async route => {
    const method = route.request().url().split('/').pop()
    if (method === 'Reorder') {
      const order = fromBinary(ReorderRequestSchema, route.request().postDataBuffer()!)
      orders.push(order)
      const cards = board.columns.flatMap(column => column.cards)
      board = create(GetBoardResponseSchema, {
        columns: order.columns.map(column => ({
          id: column.columnId,
          title: board.columns.find(c => c.id === column.columnId)!.title,
          cards: column.cardIds.map(id => ({ ...cards.find(card => card.id === id)!, columnId: column.columnId })),
        })),
      })
    }
    await route.fulfill({
      contentType: 'application/proto',
      body: method === 'GetBoard' ? Buffer.from(toBinary(GetBoardResponseSchema, board)) : Buffer.alloc(0),
    })
  })
  await page.goto('/')
  const columns = page.locator('section').filter({ has: page.locator('.dropzone') })
  await expect(columns).toHaveCount(3)
  const card = await page.getByText('Move me', { exact: true }).boundingBox()
  const working = await columns.nth(1).locator('.dropzone').boundingBox()
  const done = await columns.nth(2).locator('.dropzone').boundingBox()
  await page.mouse.move(card!.x + card!.width / 2, card!.y + card!.height / 2)
  await page.mouse.down()
  await page.mouse.move(working!.x + 80, working!.y + 35, { steps: 15 })
  await expect(columns.nth(1).locator('.kcard')).toHaveCount(1)
  // Pausing while holding the card must not reload its data underneath the drag library.
  await page.waitForTimeout(600)
  expect(orders).toHaveLength(0)
  await expect(page.getByRole('dialog')).toHaveCount(0)
  await page.mouse.move(done!.x + 80, done!.y + 35, { steps: 15 })
  await expect(columns.nth(2).locator('.kcard')).toHaveCount(1)
  await page.mouse.up()
  await expect.poll(() => orders.length).toBe(1)
  await expect(page.locator('.kcard')).toHaveCount(1)
  await expect(columns.nth(0).locator('.kcard')).toHaveCount(0)
  await expect(columns.nth(1).locator('.kcard')).toHaveCount(0)
  expect(orders[0]!.columns.map(column => [String(column.columnId), column.cardIds.map(String)]))
    .toEqual([['1', []], ['2', []], ['3', ['10']]])
  await page.reload()
  await expect(columns.nth(2).locator('.kcard').getByText('Move me', { exact: true })).toBeVisible()
  await expect(page.locator('.kcard')).toHaveCount(1)
  await expect(page.getByRole('dialog')).toHaveCount(0)
  await page.getByText('Move me', { exact: true }).click()
  await expect(page.getByRole('dialog')).toBeVisible()
  expect(errors).toEqual([])
})
