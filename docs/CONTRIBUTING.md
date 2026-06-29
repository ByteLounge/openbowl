# Contributing to OpenBowl

Thank you for your interest in contributing to OpenBowl! We are building the universal context layer for AI, and we welcome contributions from developers, architects, and designers alike.

---

## 1. Prerequisites

To build and run OpenBowl locally, you need the following tools installed on your development machine:

* **Node.js** (v18.x or later)
* **pnpm** (v8.x or later) - Monorepo package manager
* **Go** (v1.21 or later) - Local backend engine
* **Rust & Cargo** (Latest stable) - Needed for compiling Tauri desktop bindings
* **Protobuf Compiler** (`protoc`) - Optional, required if you modify gRPC definitions

---

## 2. Directory Layout

Our workspace is structured as a monorepo to separate concerns and reuse components:

```text
openbowl/
├── apps/
│   ├── desktop/           # Tauri Desktop shell (Rust config & window logic)
│   ├── web/               # Vite + React Web interface
│   └── cli/               # Go CLI management tool
├── packages/
│   ├── core/              # Go sidecar implementation (engines, routing, adapters)
│   ├── ui/                # React Shared UI components (built on shadcn primitives)
│   ├── types/             # Shared TypeScript models and interfaces
│   └── shared/            # Common utility logic
├── docs/                  # Architecture, PRD, and API specifications
└── server/                # Go Cloud synchronization backend service
```

---

## 3. Local Development Setup

Follow these steps to set up the development environment:

### Step 1: Install frontend dependencies
```bash
pnpm install
```

### Step 2: Set up local backend configurations
Navigate to `packages/core/` and set up the default local environment:
```bash
cd packages/core
cp .env.example .env
```

### Step 3: Run the application in development mode
Run the development command from the root of the project. This starts the React dev server and triggers Tauri to compile and run the Go backend sidecar:
```bash
pnpm dev
```

---

## 4. Code & Quality Standards

* **Strict TypeScript**: We do not allow `any` typings. Ensure all interfaces, props, and states are strictly typed.
* **Go Coding Style**: All Go files must pass `golangci-lint` checkups. Ensure error propagation is handled cleanly (never suppress errors).
* **Interface-Driven Design**: Before writing concrete database models or API clients, declare their interfaces in domain packages. Use dependency injection to pass concrete implementations.
* **Test Requirements**:
  * Unit tests are required for any modifications in `packages/core` or `packages/types`.
  * Run tests before submitting a Pull Request:
    ```bash
    pnpm test
    ```

---

## 5. Branching & PR Guidelines

1. **Branch Names**: Use descriptive prefixes:
   * `feat/` for new features
   * `fix/` for bug fixes
   * `docs/` for documentation updates
   * `refactor/` for code restructuring
2. **Commit Messages**: Use structured commit names (e.g., `feat(context-engine): optimize budget allocation`).
3. **Pull Requests**: Every PR must reference an existing Issue and pass all automated CI pipeline checks (linting, tests, build compilation) before review.
