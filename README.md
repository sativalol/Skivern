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

* **`Tab 0` Dashboard** – Active bot instances, live hardware usage (CPU/RAM), and real-time command stats.
* **`Tab 1` Settings** – Global naming, prefixes, embed structures, and theme setups.
* **`Tab 2` Palantir** – Log filters and tracking configurations.

> **Controls:** Press **`E`** to edit configurations within any tab. Use **`Tab`** or **`Enter`** to switch inputs, and **`Esc`** to discard changes.

---

## Features & Commands

Skyvern handles core bot operations through isolated modules:

* **Moderation** – `ban`, `warn`, `slowmode`, `temproles`, cleanups, plus more management features.
* **Utility** – `starboard`, `autoresponder`, `snipes`, and custom tags.
* **General & Fun** – `whois`, `birthdays`, `quotes`, MyAnimeList lookups, and lyrics tracking.

*Note: Administrative or guild management permissions are required for all configuration commands.*

---

## Palantir Logging

Monitors and saves system events (message updates/deletions, member changes, role updates, voice activity) into a local `palantir.db` file.

### Performance Layout

* **Asynchronous Batching:** Event logs stream to a buffered channel and commit to SQLite in batches of 100 (or every 500ms) to keep the Discord gateway loop unblocked.
* **In-Memory Cache:** Prefixes, active filters, and anti-spam limits reside in memory to drop unnecessary database reads on incoming messages.

### TUI Filters (`Tab 2`)

* **Palantir Enabled** – Global logging toggle.
* **Blocked Servers** – Target server IDs to ignore.
* **Blocked Channels** – Target channel IDs to ignore.
* **Blocked Users** – Target user IDs to ignore.
* **Blocked Events** – Specific categories to drop (`messages`, `members`, `roles`, `channels`, `invites`, `emojis`, `voice`, `server`).