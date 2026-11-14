# Citadel Sentry (CitadelSeeder) — Agent Instructions

## Mandatory Architectural Decision (ADR 0001)

All AI agents (Codex, Antigravity, Gemini, Claude) working on Citadel Sentry must strictly obey the accepted architectural decision in `docs/adr/0001-squad-connection-rejected-methods.md`.

### 1. Strictly Prohibited Approaches (DO NOT PROPOSE OR INVESTIGATE)

- **Console command `connect IP:Port`**: The command does not exist in release/shipping builds of Squad 10.6+ (`Not a Valid command (connect)!`).
- **SendInput / Keyboard Emulation**: Simulating `~` console opening, text pasting, or `Enter` for direct connect is prohibited.
- **`steam://connect/IP:Port` / Steam Server Browser**: Ignored by Squad 10.6+.
- **`steam://run/393380//...` CLI Arguments**: Squad binary has no command-line join flags.
- **Synthetic Steam Invites**: Disproven as an end-to-end join solution. The delegate `OnSessionAccepted` is orphaned with 0 listeners and does not trigger `ClientTravel`.

Do **NOT** reopen investigation into these rejected approaches without verifiable proof and explicit authorization from the project owner.

### 2. The Sole Confirmed Architectural Route

Every joining workflow must align with the proven Squad 10.6.3 pipeline:
`Server Browser → FSQServerItem → QueueForServer → Beacon → JoinSession → ClientTravel`

Focus areas:
1. Native integration with Squad Server Browser / Favorites (`GameUserSettings.ini` / EOS cache).
2. Toggling or bypassing the "Hide Empty Servers" filter so 0-player Citadel servers are immediately discoverable and selectable.
3. Invoking the confirmed `QueueForServer` flow through legitimate UI or state automation.

### 3. Ultimate Goal

**Single button «Начать сидинг» in Citadel Sentry → cold-launch Squad if needed → seamless join to CITADEL server (even with 0 players) with zero manual console input and zero manual server search.**
