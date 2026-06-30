const { test: base, expect, chromium } = require("@playwright/test");
const path = require("path");
const fs = require("fs");

// Extend standard Playwright test to automatically launch Chromium with the unpacked extension loaded
const test = base.extend({
  context: async ({}, use) => {
    const pathToExtension = path.resolve(__dirname, "..");

    // Create a temporary user data directory to isolate extension storage and settings
    const userDataDir = path.join(
      __dirname,
      `../.temp-user-data-${Math.random().toString(36).substring(7)}`,
    );

    const context = await chromium.launchPersistentContext(userDataDir, {
      headless: false,
      args: [
        `--disable-extensions-except=${pathToExtension}`,
        `--load-extension=${pathToExtension}`,
      ],
    });

    await use(context);

    await context.close();

    // Clean up temporary user data directory after test completion
    if (fs.existsSync(userDataDir)) {
      try {
        fs.rmSync(userDataDir, { recursive: true, force: true });
      } catch (e) {
        // Ignore cleanup errors
      }
    }
  },

  extensionId: async ({ context }, use) => {
    // Navigate to chrome://extensions to find the dynamically assigned extension ID
    const page = await context.newPage();
    await page.goto("chrome://extensions/");

    const extensionId = await page.evaluate(() => {
      const manager = document.querySelector("extensions-manager");
      const itemList = manager.shadowRoot.querySelector("extensions-item-list");
      const items = itemList.shadowRoot.querySelectorAll("extensions-item");
      for (const item of items) {
        const name = item.shadowRoot.querySelector("#name").textContent;
        if (name.includes("OpenBowl")) {
          return item.id;
        }
      }
      return null;
    });

    await page.close();

    if (!extensionId) {
      throw new Error("OpenBowl extension not found in chrome://extensions");
    }

    await use(extensionId);
  },
});

test.describe("OpenBowl Chrome Extension", () => {
  test("should load popup.html and handle settings persistence", async ({
    context,
    extensionId,
  }) => {
    const page = await context.newPage();

    // 1. Navigate to the extension's popup UI
    await page.goto(`chrome-extension://${extensionId}/popup.html`);

    // 2. Assert basic UI elements are present
    await expect(page.locator("h3")).toContainText("🥣 OpenBowl Context");
    await expect(page.locator("#proj-id")).toBeVisible();
    await expect(page.locator("#save-btn")).toBeVisible();

    // 3. Verify the default project ID is loaded
    await expect(page.locator("#proj-id")).toHaveValue("proj-core-default");

    // 4. Change and save new configurations
    await page.locator("#proj-id").fill("test-custom-workspace-id");
    await page.locator("#save-btn").click();

    // Assert visual feedback "Saved!" is shown
    await expect(page.locator("#save-btn")).toHaveText("Saved!");

    // 5. Reload the popup page and verify persistence in chrome.storage.local
    await page.reload();
    await expect(page.locator("#proj-id")).toHaveValue(
      "test-custom-workspace-id",
    );
  });

  test("should inject floating button on ChatGPT and insert context", async ({
    context,
    extensionId,
  }) => {
    const page = await context.newPage();

    // 1. Intercept network call to chatgpt.com to serve a mock page (prevents calling external network)
    await page.route("https://chatgpt.com/", (route) => {
      route.fulfill({
        status: 200,
        contentType: "text/html",
        body: `
          <!DOCTYPE html>
          <html>
          <head><title>Mock ChatGPT</title></head>
          <body>
            <h1>ChatGPT Workspace</h1>
            <textarea id="prompt-textarea" placeholder="Send a message"></textarea>
          </body>
          </html>
        `,
      });
    });

    // Intercept backend API conversation sync
    await page.route(
      "http://localhost:3010/api/v1/conversations/sync",
      (route) => {
        route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({ status: "synchronized" }),
        });
      },
    );

    // 2. Intercept backend API context retrieval fetch call from content.js
    await page.route(
      "http://localhost:3010/api/v1/projects/proj-core-default/context",
      (route) => {
        route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({
            context_text:
              "### OpenBowl Workspace Memory\n- Active Goals: Run Tests\n- Active Project: Default Core",
          }),
        });
      },
    );

    // 3. Navigate to chatgpt.com
    await page.goto("https://chatgpt.com/");

    // 4. Assert the injected "🥣 Inject Context" button exists on the page
    const injectBtn = page.locator("text=🥣 Inject Context");
    await expect(injectBtn).toBeVisible();

    // 5. Click the inject button and assert the context gets written to prompt textarea
    const promptBox = page.locator("#prompt-textarea");
    await expect(promptBox).toHaveValue("");

    await injectBtn.click();

    await expect(promptBox).toHaveValue(
      "### OpenBowl Workspace Memory\n- Active Goals: Run Tests\n- Active Project: Default Core\n\n",
    );
  });

  test("should inject floating button on Claude and handle contenteditable injection", async ({
    context,
    extensionId,
  }) => {
    const page = await context.newPage();

    // 1. Intercept network call to claude.ai to serve a mock contenteditable-based page
    await page.route("https://claude.ai/", (route) => {
      route.fulfill({
        status: 200,
        contentType: "text/html",
        body: `
          <!DOCTYPE html>
          <html>
          <head><title>Mock Claude</title></head>
          <body>
            <h1>Claude Chat</h1>
            <div contenteditable="true" id="claude-input" style="border:1px solid #ccc; min-height:50px;"></div>
          </body>
          </html>
        `,
      });
    });

    // Intercept backend API conversation sync
    await page.route(
      "http://localhost:3010/api/v1/conversations/sync",
      (route) => {
        route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({ status: "synchronized" }),
        });
      },
    );

    // 2. Intercept backend API context retrieval fetch call
    await page.route(
      "http://localhost:3010/api/v1/projects/proj-core-default/context",
      (route) => {
        route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({
            context_text:
              "### OpenBowl Claude Context\n- Active Task: Claude Test",
          }),
        });
      },
    );

    // 3. Navigate to claude.ai
    await page.goto("https://claude.ai/");

    // 4. Assert floating button is visible
    const injectBtn = page.locator("text=🥣 Inject Context");
    await expect(injectBtn).toBeVisible();

    // 5. Focus the contenteditable and click inject
    const inputDiv = page.locator("#claude-input");
    await inputDiv.focus();
    await injectBtn.click();

    // 6. Assert insertion into contenteditable element
    await expect(inputDiv).toContainText("### OpenBowl Claude Context");
    await expect(inputDiv).toContainText("- Active Task: Claude Test");
  });

  test("should scrape messages from ChatGPT page and sync them to backend", async ({
    context,
    extensionId,
  }) => {
    const page = await context.newPage();
    page.on("console", (msg) => console.log("BROWSER LOG:", msg.text()));

    // 1. Serve a mock ChatGPT page with user and assistant conversation turns
    await page.route("https://chatgpt.com/", (route) => {
      route.fulfill({
        status: 200,
        contentType: "text/html",
        body: `
          <!DOCTYPE html>
          <html>
          <head><title>Mock ChatGPT</title></head>
          <body>
            <div data-message-author-role="user">
              <div class="whitespace-pre-wrap">Hello AI, what is the weather in Goa?</div>
            </div>
            <div data-message-author-role="assistant">
              <div class="markdown">Hello user, it is sunny in Goa.</div>
            </div>
            <textarea id="prompt-textarea"></textarea>
          </body>
          </html>
        `,
      });
    });

    // 2. Capture and mock the conversation sync POST request
    let syncPayload = null;
    await page.route(
      "http://localhost:3010/api/v1/conversations/sync",
      (route) => {
        syncPayload = route.request().postDataJSON();
        route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({ status: "synchronized", message_count: 2 }),
        });
      },
    );

    // 3. Mock the context retrieval API
    await page.route(
      "http://localhost:3010/api/v1/projects/proj-core-default/context",
      (route) => {
        route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({
            context_text: "### OpenBowl Context\n- Project: Core",
          }),
        });
      },
    );

    // 4. Navigate and trigger context injection
    await page.goto("https://chatgpt.com/");
    await page.locator("text=🥣 Inject Context").click();

    // Wait for context injection to complete (resolves async storage.local.get and fetch race condition)
    await expect(page.locator("#prompt-textarea")).toHaveValue(
      "### OpenBowl Context\n- Project: Core\n\n",
    );

    // 5. Assert the sync payload was correct and extracted user/assistant turns
    expect(syncPayload).not.toBeNull();
    expect(syncPayload.project_id).toBe("proj-core-default");
    expect(syncPayload.messages).toEqual([
      { role: "user", content: "Hello AI, what is the weather in Goa?" },
      { role: "assistant", content: "Hello user, it is sunny in Goa." },
    ]);
  });
});
