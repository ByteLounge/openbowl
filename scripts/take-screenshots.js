const { chromium } = require("playwright");
const { exec, spawn } = require("child_process");
const path = require("path");
const fs = require("fs");

const rootDir = path.resolve(__dirname, "..");
const webDistDir = path.join(rootDir, "apps/web/dist");
const imagesDir = path.join(rootDir, "docs/images");

// Ensure images directory exists
if (!fs.existsSync(imagesDir)) {
  fs.mkdirSync(imagesDir, { recursive: true });
}

async function run() {
  console.log("Starting local preview server of the web dashboard...");

  // Start vite preview on port 3000
  const previewProcess = spawn(
    "npm",
    ["run", "preview", "--prefix", "apps/web", "--", "--port", "3000"],
    {
      cwd: rootDir,
      shell: true,
    },
  );

  // Wait 3 seconds for the server to spin up
  await new Promise((r) => setTimeout(r, 3000));

  console.log("Launching Playwright browser...");
  const browser = await chromium.launch({ headless: true });
  const context = await browser.newContext({
    viewport: { width: 1280, height: 800 },
  });
  const page = await context.newPage();

  try {
    // 1. Screenshot Webapp Dashboard
    console.log(
      "Navigating to local Webapp Dashboard at http://localhost:3000...",
    );
    await page.goto("http://localhost:3000", { waitUntil: "networkidle" });
    // Wait for elements to render
    await page.waitForSelector(".app-container", { timeout: 5000 });

    // Inject a delay to make sure UI is fully painted
    await page.waitForTimeout(2000);

    console.log("Capturing Webapp Dashboard screenshot...");
    await page.screenshot({
      path: path.join(imagesDir, "webapp_dashboard.jpg"),
      type: "jpeg",
      quality: 90,
    });
    console.log("Webapp Dashboard screenshot saved successfully!");

    // 2. Screenshot Extension Popup HTML
    console.log("Navigating to Extension Popup file...");
    const popupFilePath =
      "file:///" +
      path.join(rootDir, "apps/extension/popup.html").replace(/\\/g, "/");

    // Set smaller viewport size matching actual extension dimensions
    await page.setViewportSize({ width: 360, height: 480 });
    await page.goto(popupFilePath, { waitUntil: "load" });
    await page.waitForTimeout(1000);

    console.log("Capturing Extension Popup screenshot...");
    await page.screenshot({
      path: path.join(imagesDir, "extension_popup.jpg"),
      type: "jpeg",
      quality: 90,
    });
    console.log("Extension Popup screenshot saved successfully!");

    // 3. Screenshot Context Viewer Section
    console.log("Capturing Context Viewer Section...");
    // Restore dashboard size and navigate to dashboard
    await page.setViewportSize({ width: 1280, height: 800 });
    await page.goto("http://localhost:3000", { waitUntil: "networkidle" });
    await page.waitForTimeout(1000);

    // Crop/Screenshot the sidebar context memories list area specifically
    const sidebar = await page.$(".sidebar");
    if (sidebar) {
      console.log("Capturing Context Viewer sidebar clip...");
      await sidebar.screenshot({
        path: path.join(imagesDir, "context_viewer.jpg"),
        type: "jpeg",
        quality: 90,
      });
      console.log("Context Viewer screenshot saved successfully!");
    }

    // 4. Screenshot Demo Transition Layout HTML
    console.log("Navigating to Demo Transition Layout file...");
    const demoFilePath =
      "file:///" +
      path.join(rootDir, "scripts/demo_layout.html").replace(/\\/g, "/");

    await page.setViewportSize({ width: 1040, height: 640 });
    await page.goto(demoFilePath, { waitUntil: "load" });
    await page.waitForTimeout(1000);

    console.log("Capturing Demo Transition screenshot...");
    await page.screenshot({
      path: path.join(imagesDir, "demo_transition.jpg"),
      type: "jpeg",
      quality: 95,
    });
    console.log("Demo Transition screenshot saved successfully!");
  } catch (error) {
    console.error("Error during screenshot capture:", error);
  } finally {
    console.log("Closing browser and stopping preview server...");
    await browser.close();
    previewProcess.kill();
    // Force kill node preview processes if left hanging
    if (process.platform === "win32") {
      exec("taskkill /f /im node.exe", () => {});
    } else {
      exec('pkill -f "vite preview"', () => {});
    }
    console.log("Done!");
  }
}

run();
