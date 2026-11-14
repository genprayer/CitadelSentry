# Testing and acceptance gates

Run the narrowest applicable checks after each change, then run the complete release gate before publishing.

## Source and static checks

```bash
gofmt -w $(find internal -type f -name '*.go') app.go main.go
go vet ./...
go test -race ./...
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o /tmp/citadel-sentry-windows-check.exe .
npm --prefix frontend run typecheck
npm --prefix frontend run lint
npm --prefix frontend run generate
```

The Wails Linux build is an additional packaging check:

```bash
wails build -tags webkit2_41 -platform linux/amd64 -s
```

Windows interactive acceptance is separate from a cross-compile. Run it on a real Windows desktop with Steam and Squad installed; a successful build alone does not prove URI handling, process termination, WebView2 rendering, or power commands.

For Potato Mode recovery, run `go test -race ./internal/game ./internal/fileutil`.
The regression tests remove previously existing `Engine.ini` and `GameUserSettings.ini` after applying Potato Mode, recreate the manager, and verify exact restoration and a successful next session. They also verify that corrupt backups remain blocked and competing file creators cannot overwrite one another. In a Windows smoke test, reproduce missing-file recovery using disposable config copies and confirm that existing game settings remain intact.

Additional recovery cases preserve originals under null/missing/duplicate transaction fields, directory aliases, file symlinks and shared hard links. New transaction tests verify recorded permissions after reopening the manager, recreation of missing files and external permission changes. Windows read-only and hard-link behavior still require native acceptance in addition to compilation.

Status regressions cover unreadable settings, orphaned original/session files after manifest removal, FPS in comments/unrelated sections, fractional FPS and ambiguous supported values. Logger regressions preserve scoped attributes/group boundaries, shared concurrent writes, `LogValue` representations and bounded recursive groups. Their local race checks are distinct from native Windows filesystem/UI acceptance.

Run `npm --prefix frontend test` for desktop/browser adapter regressions. These execute the actual TypeScript source with isolated bridge and API fixtures; they are not Windows or in-game acceptance. Tests cover backend errors, absent native operations, immutable preview defaults, authoritative empty fleets, unready server states and saved seeding targets.

Process-inspection regressions in `app_test.go` and `internal/platform` distinguish a confirmed stopped game from unavailable OS state. Seed tests verify that close/exit failure prevents power commands; they use explicit adapters and never shut down or suspend the test host. Recovery tests recreate the manager to prove that interrupted preparation remains recoverable with automatic restoration disabled, while a recorded finished session preserves that preference. Frontend tests cover unknown process status, late status responses and listener ownership across remounts.

Power-duration regression tests distinguish late Squad observation from launch waiting. Frontend cancellation tests require the native bridge and retain native cancellation errors; no fixture performs a real shutdown or suspend.

Priority tests use process-handle fixtures to verify distinct observed originals, preflight/read/backup failures, partial write rollback, persistent retry, process-ID reuse, external changes, ambiguous JSON and corrupt checksums. Application tests confirm that native-setting failure aborts the identified mission and retains graphics recovery. File-lock tests exercise the local OS primitive; the Windows primitive is cross-compiled and requires separate native acceptance. Do not infer that Windows permissions or anti-cheat access have passed from these fixtures.

Config validation tests preserve invalid files/cached values, explicit endpoints and current actions during legacy migration. JSON tests reject duplicate/case-folded keys and trailing values. Application tests recover interrupted graphics without readable Sentry preferences and restore preflight-only preparation when no game was observed. Linux opener tests use task-owned copies of harmless local command primitives and prove cancellation/reaping; they do not open Steam or a real browser.

For opt-in local Linux logind acceptance, run `CITADEL_TEST_LOGIND=1 go test -race ./internal/platform -run '^TestNativeLogindInhibitorLifecycle$' -count=1 -v` through the resource guard. It acquires one owned idle/sleep blocker, verifies its exact PID/reason, releases it and confirms the owned count returns to zero. It performs no suspend, shutdown or game launch. This gate is separate from ordinary fixture tests and Windows acceptance.

## Functional smoke matrix

Use a test account and explicitly authorized test servers. Do not use production Servers 1–4 for exploratory tests, and do not restart a server as part of a client smoke test.

1. Load the visible fleet and refresh it. Confirm the list comes from `GET /api/servers/sentry` and that offline entries are not queued.
2. Click Join. Confirm the browser/desktop bridge reports an error when unavailable and that a successful request opens the website's Steam lobby flow.
3. Start one seed with a short test duration. Confirm the elapsed clock updates every second, the remaining timer reaches zero, and Stop returns the session to a terminal state.
4. Start a two-server queue with a test target. Confirm the first server enters `active`, then the engine waits for Squad exit before the second Steam lobby is launched.
5. Exercise `abort`, `notify_only`, `continue_next`, and `retry`. For `retry`, verify that the current server is retried until Stop or target completion.
6. Exercise `close_squad`, `sleep`, and `shutdown` only in a controlled test window. For multiple servers verify the action runs after the queue, not after each server.
7. Disconnect Steam/API access and confirm the UI shows an honest error or stale/offline state instead of inventing servers.

## Memory, race, and crash checks

`go test -race ./...` is the required concurrency gate. Repeat a short start/stop loop while watching the process RSS with the operating system's process monitor. A rising RSS across stable iterations is a release blocker and must be reproduced with a profile or minimal test before claiming a leak fix. The project does not claim a formal leak-free guarantee from compilation alone.

For a crash or destructive-action report, preserve the exact version, OS, logs, server ID, action, and reproduction steps. Stop the session, disable automatic startup, and do not ask the user to retry a power action until the failure is understood.

Local release regression fixtures: `PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s scripts/tests -p test_release.py -v`. These use copied metadata, simulated build outputs and task-owned child processes. They verify rejected missing/inconsistent versions, exact rollback, checksum/file-identity guards, concurrent-launch refusal, real SIGTERM/SIGKILL recovery, no-clobber publication, stale-output rejection and preservation of unknown retention entries. Native Wails packaging and installed-copy acceptance remain separate from this fixture gate.
