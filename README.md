# Skyvern `v0.1.0-alpha`

A multi-instance Discord bot runner and moderation tool built in Go, managed directly from a terminal user interface (TUI).

> **Warning:** Skyvern is currently in **alpha**. Features are subject to change, and bugs may occur.

---

## Getting Started

### Prerequisites

* Go 1.21+

### Build and Run

You can build the executable directly, or use our compiler script:

#### Builder
```bash
# Run the compiler
go run build.go
```
This prints a menu to let you choose your target platform (Windows, macOS, Linux, Android) and automatically builds the binary with correct env configuration.

---

## TUI Navigation

Built with Bubble Tea. Use **`Tab`** to cycle through the panels:

* **`Tab 0` Dashboard** – Active bot instances, hardware usage (CPU/RAM), etc.
* **`Tab 1` Settings** – Naming, prefixes, embed structures, and theme setups.
* **`Tab 2` Palantir** – Logs and tracking cfg.
* **`Tab 3` Lavalink** - See lavalink logs.

> **Controls:** Press **`E`** to edit configurations within any tab. Use **`Tab`** or **`Enter`** to switch inputs, and **`Esc`** to discard changes.

---

## Features

* **Moderation** – `ban`, `warn`, `slowmode`, `temproles`, cleanups, plus more management features.
* **Utility** – `starboard`, `autoresponder`, `snipes`, and custom tags.
* **General & Fun** – `whois`, `birthdays`, `quotes`, MyAnimeList lookups, and lyrics tracking.

- Note, some haven't been tested fully.
---

## Palantir Logging

Saves every event (message updates/deletions, member changes, role updates, voice activity) into a `palantir.db` file.

### Layout

* **Batching:** Event logs stream to a buffered channel and commit to SQLite in batches of 100 (or every 500ms) to keep the Discord gateway loop unblocked.
* **Cache:** Prefixes, active filters, and anti-spam limits reside in memory to drop unnecessary database reads on incoming messages.

### TUI Filters (`Tab 2`)

* **Palantir Enabled** – Global logging toggle.
* **Blocked Servers** – server IDs to ignore.
* **Blocked Channels** – channel IDs to ignore.
* **Blocked Users** – user IDs to ignore.
* **Blocked Events** – Specific categories to drop (`messages`, `members`, `roles`, `channels`, `invites`, `emojis`, `voice`, `server`).

---

## Database Encryption

All sensitive credentials (such as Discord Bot Tokens and AI API keys) are stored encrypted at rest inside BoltDB (`bots.db`) using authenticated **AES-256-GCM**.

### Key Configuration

The 32-byte encryption key is derived using SHA-256 from your master key string. You can configure this key in one of two ways:

1. **Local Configuration File (Recommended for Local Use)**:
   Add a `crypt_key` field to your `tui_config.json` next to the binary:
   ```json
   {
     "storage_location": "local",
     "crypt_key": "your-secret-master-key-here"
   }
   ```
2. **Environment Variable (Recommended for Production)**:
   Set the `SKYVERN_CRYPT_KEY` environment variable in your operating system. This is safer because it prevents database decryption even if your application directory is leaked or stolen.

If neither is set, Skyvern will print a warning on boot and use a fallback default key. Plaintext credentials from older database versions are automatically migrated to encrypted form on their next write transaction.

---

## Plugins

Skyvern uses an in-tree plugin system to keep the manager clean. Plugins are given direct access to the database and session manager, meaning they can register commands, attach custom event handlers, or spin up workers.

### How to Add a Plugin

1. Create a new package under `internal/plugins/` (e.g., `internal/plugins/economy/`).
2. Implement the `plugins.Plugin` interface.
3. Call `plugins.Register()` inside your package's `init()` function.
4. Import the package anonymously in `main.go` so it compiles into the binary:
   ```go
   import _ "skyvern/internal/plugins/economy"
   ```
5. Rebuild the bot.
