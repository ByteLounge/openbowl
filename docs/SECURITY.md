# OpenBowl Security Policy & Data Protection 🛡️

OpenBowl is designed from the ground up with a **local-first, privacy-respecting threat model**. Your conversation histories, local codebase files, and API keys are stored entirely on your own computer.

---

## 🔒 Threat Model & Data Siloing

In the OpenBowl architecture:

1. **No External Cloud Dependency**: Requests to AI providers (like OpenAI, Anthropic, or Ollama) are made directly from your machine. There are no middleman proxy servers capturing your data.
2. **Same-Origin REST Interfaces**: The local sidecar Gin server binds exclusively to `localhost:3010`. External hosts on the internet cannot access this port unless they are running on the local loopback interface.

---

## 🔑 Secure API Key Storage

Storing raw, plain-text API keys in a local database is a significant vulnerability. OpenBowl secures key storage using two complementary strategies:

### 1. OS-Level Secure Enclaves (Desktop App)

When running within the Tauri desktop container, OpenBowl delegates API key storage to native operating system enclaves:

- **Windows**: Windows Credential Manager
- **macOS**: Apple Keychain Service
- **Linux**: Secret Service API (via D-Bus)

This ensures that other standard user processes cannot query or scrape your keys.

### 2. AES-256-GCM Database Encryption

For local database configurations where OS keyrings are unavailable (such as head-less sidecar mode), OpenBowl encrypts sensitive credential records before writing them to the SQLite file.

- **Algorithm**: AES-256-GCM (Galois/Counter Mode) authenticated encryption.
- **Key Derivation**: PBKDF2 with SHA-256 using a local device salt combined with a system identifier.
- **Format**: Database entries store the base64-encoded payload containing `[nonce] + [ciphertext] + [auth tag]`.

---

## 📁 SQLite File Protection

On start, the OpenBowl backend checks the database file (`openbowl.db`) and enforces secure OS file permissions:

- **Windows**: Restricts file ACL access to the currently logged-on user account.
- **macOS / Linux**: Sets file permissions to `0600` (read/write only by the owner).

---

## 🌐 Browser Extension Security

The OpenBowl Chrome Extension operates with strict Manifest V3 policies:

- **Permissions**: Requires only `activeTab` and `storage` privileges.
- **CORS Safety**: Local backend requests do not expose or share credentials (`AllowCredentials: false`) to prevent external cross-origin scripts from hijacking loopback tokens.
