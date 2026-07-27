# Relai

One tray icon for every AI coding plan you pay for.

Relai shows how much of each provider's rate-limit window you have burned — Claude and Codex today — lets you switch Claude accounts, and hands a live coding session off to another tool when a plan runs dry.

> **Status: design only.** The spec is written and every external interface it depends on has been verified against real output. No implementation yet.

## Why

If you work against several plans with rolling limits (Claude 5h/7d, Codex 5h/weekly), three things are missing at the moment you actually need them:

1. **Passive visibility.** You find out you are out of quota by hitting the wall mid-task.
2. **A fast switch.** Rotating to another account means leaving your session.
3. **A way across.** When every Claude window is spent, the work should be able to continue somewhere that still has quota.

Existing tools each solve one of these. Relai puts all three behind one icon.

## Design

Relai **does not reimplement** quota fetching or session parsing. It shells out to the tools that already do it well and parses their JSON:

| Concern | Delegated to |
|---|---|
| Claude quota + account switching | [`claude-swap`](https://github.com/realiti4/claude-swap) (`cswap --json`) |
| Session handoff between coding CLIs | [`continues`](https://github.com/yigitkonur/cli-continues) (`--jsonl`) |
| Codex quota | Codex app-server, `Account/rateLimits/readRequest` |

The full design, including verified schemas for all three, is in
[`docs/superpowers/specs/2026-07-27-relai-design.md`](docs/superpowers/specs/2026-07-27-relai-design.md).

## Platforms

Go 1.26 + [`fyne.io/systray`](https://github.com/fyne-io/systray). Cross-compilation verified from macOS arm64 with no extra toolchain:

| Target | Binary |
|---|---|
| darwin/arm64 | 3.8 MB |
| windows/amd64 | 4.1 MB |
| linux/amd64 | 6.1 MB |

Linux additionally needs a StatusNotifierItem host (appindicator or equivalent) at runtime, or the icon will not appear.

## Prior art

Relai is deliberately duplicative. [`cswap menubar`](https://github.com/realiti4/claude-swap) already gives you a macOS menu bar for Claude accounts, and [CodexBar](https://github.com/steipete/CodexBar) already shows quota across 63+ providers and is far more mature. If you only need one of those halves, use them — they are better at their own job. Relai exists for the case where you want quota, account switching, and session handoff behind a single icon, on all three desktop platforms.
