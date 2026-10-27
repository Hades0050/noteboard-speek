import { expect, test } from '@playwright/test'
import { create, toBinary } from '@bufbuild/protobuf'
import { GetBoardResponseSchema, GetNoteResponseSchema } from '../app/gen/noteboard/v1/noteboard_pb'

for (const linked of [false, true]) {
  test(`a click opens a spacious ${linked ? 'note' : 'card'} modal`, async ({ page }) => {
    await page.setViewportSize({ width: 1440, height: 900 })
    const title = linked ? 'План релиза' : 'Проверить макеты'
    const board = create(GetBoardResponseSchema, {
      columns: [{ id: 1n, title: 'Идеи', cards: [{ id: 10n, columnId: 1n, title, noteId: linked ? 1n : undefined }] }],
    })
    const note = create(GetNoteResponseSchema, { note: { id: 1n, title, body: '<p>Описание задачи</p>', audioPath: 'voice.wav' } })
    let saves = 0
    await page.route('**/api/rpc/**', async route => {
      const method = route.request().url().split('/').pop()
      if (method === 'Reorder') saves++
      await route.fulfill({
        contentType: 'application/proto',
        body: method === 'GetBoard' ? Buffer.from(toBinary(GetBoardResponseSchema, board))
          : method === 'GetNote' ? Buffer.from(toBinary(GetNoteResponseSchema, note)) : Buffer.alloc(0),
      })
    })
    await page.goto('/')
    await page.locator('.kcard').click()
    const dialog = page.getByRole('dialog')
    await expect(dialog).toBeVisible()
    await expect(dialog.getByPlaceholder('Заголовок', { exact: true })).toHaveValue(title)
    const box = await dialog.boundingBox()
    expect(box!.width).toBeGreaterThanOrEqual(1000)
    expect(box!.height).toBeGreaterThanOrEqual(650)
    expect(saves).toBe(0)
    if (linked) {
      const header = dialog.locator('[data-slot="header"]')
      for (const name of ['На доску', 'Закрепить', 'Удалить']) {
        await expect(header.getByRole('button', { name, exact: true })).toBeVisible()
      }
      const save = await dialog.getByRole('button', { name: 'Сохранить', exact: true }).boundingBox()
      const record = await dialog.getByRole('button', { name: 'Перезаписать голос', exact: true }).boundingBox()
      expect(save!.x + save!.width).toBeGreaterThan(box!.x + box!.width - 40)
      expect(record!.x + record!.width).toBeLessThan(save!.x)
      expect(Math.abs(record!.y - save!.y)).toBeLessThan(2)
    }
    await page.keyboard.press('Escape')
    await expect(dialog).toHaveCount(0)

    for (const width of [390, 320]) {
      await page.setViewportSize({ width, height: 844 })
      await page.locator('.kcard').click()
      await expect(dialog).toBeVisible()
      await expect.poll(async () => Math.round((await dialog.boundingBox())!.width)).toBe(width)
      await expect.poll(async () => Math.round((await dialog.boundingBox())!.height)).toBe(844)
      await expect(dialog.getByRole('button', { name: 'Сохранить', exact: true })).toBeInViewport()
      if (linked) {
        for (const name of ['На доску', 'Закрепить', 'Удалить', 'Перезаписать голос']) {
          await expect(dialog.getByRole('button', { name, exact: true })).toBeInViewport()
        }
        const save = await dialog.getByRole('button', { name: 'Сохранить', exact: true }).boundingBox()
        const record = await dialog.getByRole('button', { name: 'Перезаписать голос', exact: true }).boundingBox()
        expect(record!.x + record!.width).toBeLessThan(save!.x)
        expect(Math.abs(record!.y - save!.y)).toBeLessThan(2)
      }
      await page.keyboard.press('Escape')
      await expect(dialog).toHaveCount(0)
    }
  })
}
