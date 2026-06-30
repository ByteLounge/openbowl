# OpenBowl 🥣 — Universal Context Layer for AI

<img width="891" height="280" alt="openbowl-banner" src="https://github.com/user-attachments/assets/ef194ab1-3a14-4bff-860d-976f436a3024" />

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat-square&logo=go)](https://golang.org/)
[![TypeScript](https://img.shields.io/badge/TypeScript-5.x-3178C6?style=flat-square&logo=typescript)](https://www.typescriptlang.org/)
[![React](https://img.shields.io/badge/React-18.x-61DAFB?style=flat-square&logo=react)](https://react.dev/)
[![Playwright](https://img.shields.io/badge/Playwright-E2E-green?style=flat-square&logo=playwright)](https://playwright.dev/)

> **OpenBowl** is the universal context layer for AI. It allows you to switch between ChatGPT, Claude, Gemini, and local LLMs (like Ollama or LM Studio) **without losing your conversation context, active task list, or architectural decisions**. 
>
> In OpenBowl, **the user owns the memory**, and the **AI models only generate responses**.

---

## 💡 How It Works (In Simple Terms)

1. **Local Sidecar Server**: You run a lightweight backend server on your machine. It stores all your project metadata, tasks, architectural decisions, and active chats in a fast, private local SQLite database.
2. **Chrome Extension**: Injects a floating **🥣 Inject Context** button on ChatGPT and Claude.
3. **Background Auto-Sync**: As you chat with Claude or ChatGPT, the extension silently captures your conversation turns (prompts and answers) and updates your local database.
4. **Seamless Continuity**: When you switch models (e.g., from Claude to ChatGPT) and click **Inject Context**, OpenBowl dynamically inserts your active project tasks, structural memories, and a sliding window of your **last 3 conversation turns** directly into the input box. The new model instantly picks up right where you left off!

---

## ✨ Core Features

* **Real-time Extension Auto-Sync**: Inbuilt DOM MutationObserver captures user and assistant messages dynamically in the background as you chat.
* **Sliding Window Context Buffer**: Automatically limits raw history to the last 6 messages (3 turns). This prevents prompt bloat and keeps context dense, neat, and token-efficient.
* **Zero-config Startup script**: A PowerShell installer sets up the backend server to run silently on Windows login (hidden cmd console).
* **Robust E2E Testing**: A built-in Playwright test suite loads the extension unpacked, mocks LLM frontends and Go REST APIs, and asserts DOM/Storage integrations.

---

## 🛠️ Tech Stack

* **Frontend Client**: React, TypeScript, Vite, Zustand
* **Desktop Container**: Tauri (Rust shell)
* **Local Sidecar Backend**: Go, Gin Gonic, Pure-Go SQLite (`modernc.org/sqlite`)
* **E2E & Integration Testing**: Playwright, Go Test Suite

---

## 🚀 Setup Instructions

### 1. Prerequisites
Make sure you have:
* **Node.js** (v18+)
* **Go** (v1.21+)

---

### 2. Install and Run the Go Backend (Windows Startup Setup)
We have automated the compilation and startup configuration so the Go backend runs silently in the background on every login.

1. Open PowerShell inside the project directory (`D:\Projects\OpenBowl`) and run:
   ```powershell
   powershell -ExecutionPolicy Bypass -File .\scripts\install-startup.ps1
   ```
2. **What this does**:
   * Compiles the Go server into `bin/openbowl-server.exe`.
   * Places a silent background launcher `OpenBowlLauncher.vbs` inside your Windows **Startup folder**.
   * Boots the server immediately on port `3010`.

> [!TIP]
> If you ever need to restart the server manually, press `Win + R`, type `shell:startup`, and double-click `OpenBowlLauncher.vbs`.

---

### 3. Install the Chrome Extension
1. Open Google Chrome and navigate to **`chrome://extensions/`**.
2. Enable **Developer mode** (toggle in the top-right corner).
3. Click **Load unpacked** (top-left corner).
4. Select the **`apps/extension`** folder inside the OpenBowl project directory.

---

## 📖 Usage Guide

### 1. Configure the Project ID
* Click the extension icon in your Chrome toolbar.
* Enter your **Active Project ID** (defaults to `proj-core-default`).
* Click **Save Configurations**.

### 2. Conversational Continuity in Action
1. Open **Claude** (`https://claude.ai/`).
2. Ask: *`What is the weather in Pune?`*
3. Claude replies. The extension automatically syncs this turn to your local database in the background.
4. Open **ChatGPT** (`https://chatgpt.com/`) in a new tab.
5. Click the **🥣 Inject Context** floating button. 
6. Your project roadmap, decisions, and the Pune weather history are instantly written into the ChatGPT prompt box!
7. Type: *`What about Vellore?`* and ChatGPT will continue the chat seamlessly.

---

## 🧪 Running Tests

### Go Backend Tests
Run the Go unit tests across all packages:
```bash
cd packages/core
go test -v ./...
```

### Extension Playwright E2E Tests
Run the Playwright test suite which automatically spins up Chromium, loads the unpacked extension, mocks the APIs/websites, and validates functionality:
```bash
cd apps/extension
npm install
npx playwright install chromium
npm run test
```

---

## 📄 License
This project is licensed under the MIT License - see the `LICENSE` file for details.
