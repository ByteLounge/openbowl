const { defineConfig } = require('@playwright/test');

module.exports = defineConfig({
  testDir: './tests',
  timeout: 30000,
  expect: {
    timeout: 5000,
  },
  use: {
    // Playwright supports testing extensions only when running in non-headless mode or modern headless mode
    // We run headed to let users see the extension popup and content script in action!
    headless: false,
  },
  projects: [
    {
      name: 'chromium',
      use: {
        browserName: 'chromium',
      },
    },
  ],
});
