import { defineConfig } from '@playwright/test'

export default defineConfig({
  testDir: './tests',
  expect: { timeout: 15000 },
  use: { baseURL: 'http://127.0.0.1:3000' },
  webServer: {
    command: 'bun run dev --host 127.0.0.1',
    url: 'http://127.0.0.1:3000',
    reuseExistingServer: !process.env.CI,
  },
})
