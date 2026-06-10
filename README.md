# Skyvern `v0.1.0-alpha`

A multi-instance Discord bot runner and moderation tool built in Go, managed directly from a terminal user interface (TUI).

> **Warning:** Skyvern is currently in **alpha**. Features are subject to change, and bugs may occur.

---

## Getting Started

### Prerequisites

* Go 1.21+

### Build and Run

```powershell
# Build the executable
go build -o skyvern.exe main.go

# Launch the TUI
.\skyvern.exe

```

---

## TUI Navigation

Built with Bubble Tea. Use **`Tab`** to cycle through the panels:

* **`Tab 0` Dashboard** – Active bot instances, hardware usage (CPU/RAM), etc.
* **`Tab 1` Settings** – Naming, prefixes, embed structures, and theme setups.
* **`Tab 2` Palantir** – Logs and tracking cfg.

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
