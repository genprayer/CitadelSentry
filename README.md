# Citadel Sentry

Citadel Sentry is a Wails desktop application for Citadel/Squad seeding. It loads the server fleet from the Citadel API, coordinates automated one-click joining into the Citadel game servers on Linux or Windows, and manages a bounded queue of seeding sessions.

> **Architectural Decision (ADR 0001)**: The project strictly adheres to [`docs/adr/0001-squad-connection-rejected-methods.md`](docs/adr/0001-squad-connection-rejected-methods.md).
> Prohibited: console `connect IP:Port`, SendInput console automation, `steam://connect`, CLI params, Synthetic Steam Invites as a complete join solution.
> Confirmed pipeline: `Server Browser → FSQServerItem → QueueForServer → Beacon → JoinSession → ClientTravel`.

The current release is a release candidate. It is not a kernel driver, anti-cheat component, or remote server manager. It does not connect to Squad through a raw IP command and it does not claim to detect a player manually changing servers inside Squad.

## Download and run

Download the Windows or Linux binary and the matching `SHA256SUMS.txt` from the release page. Verify the checksum before starting the executable.

### Windows (По умолчанию / Primary)

1. Скачайте `citadel-sentry-v<VERSION>-windows-amd64.exe` и `SHA256SUMS.txt`.
2. В PowerShell проверьте контрольную сумму файла:
   ```powershell
   Get-FileHash .\citadel-sentry-v<VERSION>-windows-amd64.exe -Algorithm SHA256
   ```
   Сравните полученный хэш со строкой в `SHA256SUMS.txt`.
3. Запустите `.exe`. Требуется Windows 10/11 с установленным Steam и системным компонентом WebView2 (встроен в Windows 10/11 по умолчанию).

<details>
<summary>🐧 <b>Инструкция для Linux (нажмите, чтобы развернуть)</b></summary>

1. Скачайте бинарник для Linux и сделайте его исполняемым:
   ```bash
   chmod +x citadel-sentry-v<VERSION>-linux-amd64
   ./citadel-sentry-v<VERSION>-linux-amd64
   ```
2. Проверка контрольной суммы:
   ```bash
   sha256sum -c SHA256SUMS.txt --ignore-missing
   ```

> **Окружение**: Приложение скомпилировано с тегом `webkit2_41` (WebKitGTK 4.1). Поддерживаются Wayland и X11 (GNOME, KDE Plasma и др.). Конфигурация Squad автоматически обнаруживается в библиотеках Steam и префиксе Proton (AppID 393380).
</details>

On first launch, open Settings and confirm the Citadel API endpoint. The default is `https://jointhectl.com`. The application uses the public server API without a desktop login flow.

The local settings file is created under the operating system's user configuration directory as `citadel-sentry/config.json`. Settings are written with user-only permissions where the platform supports them. Existing values are validated before use or save; missing, null, ambiguous or unsupported choices are reported without replacing the file with defaults. Fresh settings request CPU-priority tuning only when the platform supports it.

## Build from source

Requirements:

- Go version declared in [`go.mod`](go.mod)
- Node.js 18 or newer and npm
- Wails CLI v2
- Linux: GTK/WebKitGTK development packages, a C compiler, and `pkg-config`
- Windows: a supported Go/MinGW toolchain and WebView2 runtime for interactive testing

Install frontend dependencies and run the checks:

```bash
npm run install:frontend
gofmt -w $(find internal -type f -name '*.go') app.go main.go
npm run typecheck
npm run lint
npm run generate
go vet ./...
go test -race ./...
```

Build locally with:

```bash
npm run build:linux
npm run build:windows
```

The release pipeline is documented in [`docs/RELEASING.md`](docs/RELEASING.md). Never put `.env` files, access tokens, auth caches, signing keys, private server configuration, or unlicensed third-party assets into a public repository.

## Product behavior and limitations

- **Seamless server switching**: When advancing to the next server in the queue (upon reaching player targets) or skipping a server manually, Sentry dispatches the new connection directly to the running Squad process without killing or restarting the game. If Squad was stopped, it performs a clean cold launch.
- A running game installation on Windows uses the user's `LOCALAPPDATA` configuration for settings and logs.

<details>
<summary>🐧 <b>Особенности обнаружения конфигураций и поведения на Linux (Proton)</b></summary>

- On Linux, Squad settings and logs are discovered in Steam's Proton prefix for app 393380, using registered libraries from `libraryfolders.vdf`, including external drives, Steam Flatpak and custom XDG data locations. The config is under `pfx/drive_c/users/<user>/AppData/Local/SquadGame/Saved/Config/Windows*/GameUserSettings.ini` (the legacy `Squad` directory is also checked). Manually relocated Wine prefixes outside these Steam libraries are not automatically discovered.
- Linux sleep inhibition uses the actual `systemd-logind` file descriptor and releases it with the background worker.
- Desktop URI dispatch on Linux uses native execution with fallback, and WebKitGTK 4.1 hardware compositing is enabled for smooth 60+ FPS UI rendering under Wayland and X11.
</details>

- The visible fleet is authoritative. Hidden or offline servers returned by the API are not queued.
- Fleet replies must contain explicit server identities and required numeric telemetry. Invalid/null/duplicate records preserve the last confirmed list and show an error, including on the dashboard. A late request cannot replace a newer list or clear its loading state. The browser preview uses the saved API endpoint; unavailable reserve-queue data and missing display targets are not replaced with invented counts.
- Seeding requires a confirmed session snapshot. Unavailable or malformed bridge state keeps the last valid snapshot, shows an unknown status and offers a retry; it does not display a new idle session or invent absent plan choices. The saved target is retained, and already reached targets are left to the backend's preflight skip check.
- Server capacity must be known and positive. The backend rejects a target above the confirmed capacity, including a capacity change detected immediately before launching Squad.
- A queue switches servers directly without closing Squad if the game is already running. If Squad was closed or stopped, it launches the game, waits for menu and login readiness, and dispatches the connection. Each server requires its own connection confirmation.
- A running process or dispatched invitation does not start the timer. New `SquadGame.log` records must show a server welcome and the deployment-menu load, plus the selected IP/game port. When the API hides that endpoint, Sentry instead requires the fresh local player's logged SteamID in the selected server's public team roster; a player who has not joined a team may remain unconfirmed. Log discovery follows the same Windows/Steam-library locations as settings. Missing confirmation stays in Connecting, with a separate 12-minute connection deadline; explicit log/read/roster errors fail visibly. These failures do not execute mission timeout actions. Log rotation/truncation resets connection evidence, and time is paused after a disconnection until the target connection is confirmed again.
- Cold launch uses Steam's normal game launch without MoviePlayer overrides. Sentry resolves the current invitation after menu/login readiness. Automated missions retry up to twice only when Squad explicitly reports that a session search prevented invitation handling; the retry delay is cancellable. Automatic intro skipping is not provided. The observed RedpointEOSRoomId invitation rejection remains unresolved, and native Windows joining is unverified.
- The unreleased investigation found an incompatible Steam lobby build before the secondary EOS error. Improved rejection reporting is covered by automated tests; successful native joining remains unconfirmed. Publication is blocked pending the scenarios in [release acceptance](docs/RELEASE_ACCEPTANCE.md).
- Target and timeout clocks use the local operating-system clock; backend elapsed time is monotonic within the running process. An explicitly selected local finish time keeps its chosen date and recalculates remaining minutes when Start is pressed. Editing the duration or choosing a duration preset returns to a relative timer. A passed or too-short finish time blocks launch instead of choosing another day.
- The initial player count comes from the confirmed live preflight response before game dispatch. An already reached target shows complete progress even when no game launch was necessary.
- A timeout policy of `retry` intentionally remains on the current server until the target is reached or the user presses Stop. The UI warns about this behavior.
- Targets and time budgets apply separately to each server. Reaching a target advances to the next queued server automatically. Timeout actions either skip the current server or stop the entire queue, as explicitly labelled; skipping the last server leaves Squad open. Any skipped server makes the overall outcome `timed_out` and prevents the successful-queue action, even when the last server later succeeds. The interface previews the selected policy before starting and reports the skipped count during/after the mission.
- General seeding settings provide defaults for a new mission. The launch dialog can override them; saving defaults does not change an already running mission.
- A selected sleep/shutdown action after success runs only when every selected server reaches its target. An explicitly selected timeout power action uses the same safety checks. Stop and failures do not execute power actions; the minimum active duration counts only confirmed connection time, excluding launch preparation, loading and waiting.
- Sentry cannot reliably detect a manual in-game server switch. It tracks the server it asked Squad to join and refreshes that server's public telemetry. Automatic cancellation on manual switching would require a trusted player/Steam telemetry contract that does not exist yet.
- Joining can fail when Steam, Squad, the browser protocol handler, the Citadel API, or the user's network is unavailable. The UI reports the error and does not silently retry a direct join.
- Potato Mode requires Squad to be closed before seeding starts. Sentry changes only supported settings already present in `GameUserSettings.ini` and, when present, the matching `Engine.ini`. It does not create missing game keys or files. Missing required settings or ambiguous values stop the attempt; a failed partial patch restores verified originals.
- Sentry records the original contents of every changed INI file with SHA-256 hashes. With automatic restoration enabled, it waits for Squad to exit after seeding before restoring settings; unrelated changes made by Squad are retained. Restoration failures remain visible as a pending restore in Settings.
- Invalid or ambiguous backup metadata blocks new patches and baseline replacement while preserving the saved originals. Directory aliases are deduplicated; file links, shared hard links and special files are rejected before mutation.
- Status and recovery decisions verify every recorded original and expected patch by SHA-256, with original sizes checked as well. Corrupt, missing or unreadable records remain visible and cannot be silently replaced by a new baseline. Nonregular manifests are rejected before opening them.
- Incomplete or invalid desktop config status is shown as unavailable. Late reads cannot hide a newer backup failure or discard a newer verified status; Settings offers a retry and keeps config actions disabled until the observations validate.
- Restoration recovers a removed managed key from its recorded original while preserving unrelated edits and valid player overrides. Repeated INI sections do not cause duplicate settings, and saved values are restored literally.
- Missing backup metadata also blocks replacement when original/session files remain. Sentry keeps these records for recovery instead of treating them as a fresh baseline. Unreadable INI/backup paths and unknown or conflicting FPS values are reported; FPS diagnostics ignore comments/unrelated sections and retain fractional values.
- New transactions also record the actual permission bits (Windows read-only state) and restore them across restarts while preserving external permission changes. Historical backups without attribute records keep existing permissions; recreated legacy files use the backup's actual permissions. These records do not reconstruct unrecorded ACLs or other filesystem attributes.
- Failed missions and interrupted preparation roll back temporary settings after a verified Squad exit even when automatic restoration after normal completion is disabled. The backup manifest preserves this obligation across application restarts; only a finished owning session can allow the user's opt-out.
- Settings prepared for a queue that never observes Squad are restored even when automatic restoration is disabled. Interrupted graphics recovery is checked before reading application preferences, so a broken `config.json` cannot discard a verified game backup.
- A failed process inspection is shown as unknown and blocks settings changes and power actions. Sleep/shutdown stops if Squad cannot be closed or its exit cannot be verified.
- Closing the desktop stops owned background work, waits for sleep-inhibitor cleanup, and attempts pending restoration only after a verified game exit. If Squad remains running, the verified backup remains pending for the next launch.
- Windows CPU-priority changes first record the observed class and process identity in a checksummed backup. Each game process in the queue is checked; failed application rolls back, and Stop, normal completion or startup restores recorded values while preserving external changes. Linux reports this optional priority feature as unavailable and blocks a requested change before patching graphics.
- An OS file lock protects application state before logging or recovery begins. A second executable cannot recover or overwrite the running instance's state. The lock is released by process exit, while its stable file remains in the user configuration directory.
- Linux sleep inhibition uses the actual logind file descriptor and releases it with the worker. Desktop URI dispatch waits for the bounded opener result and supports cancellation; Windows opens resources through the shell API without passing URLs through a command interpreter.
- The log viewer reports failed reads and clearing while keeping the last confirmed lines. Refresh retries reading; an older read cannot repopulate the view after clearing. Native log operations require their desktop methods, and malformed line payloads are rejected before rendering.
- If a previously existing INI file disappears during Potato Mode, restoration recreates it from its verified backup without replacing a concurrently created file. An absent file that Sentry originally created stays absent after restoration.
- The join resolver uses the website's verified connection metadata. If the website cannot provide a supported lobby link, Sentry reports that result instead of selecting another server from built-in names.
- Actual in-game FPS depends on Squad loading these files and not overriding the cap at runtime. Verify the FPS cap in a Windows/Squad smoke test before publishing a build with Potato Mode claims.
- Version 1.2.2 adds a Settings button that checks stable releases at `genprayer/CitadelSentry` and installs a newer version in one click. It downloads the matching portable binary into the current application directory, verifies its SHA-256 against the release checksum file, exits Sentry, swaps the binary through a helper process, and launches the new version. Stop seeding and finish any pending Squad config restoration first; the application directory must be writable.
- Builds through 1.2.1 have no updater and require one manual installation of 1.2.2. The button updates 1.2.2 and later builds when a newer stable GitHub release is published.
- Update metadata must explicitly identify the release version, draft/prerelease flags and asset records. Missing/null consumed fields, duplicate keys or asset names and unfinished platform assets stop the check before download. Unknown additional GitHub metadata and nullable digests remain compatible; downloaded binaries still require the release SHA-256 verification.
- Once updating or shutdown owns application state, user settings saves and manual Squad backup/restoration return failure without mutation. Internal shutdown recovery remains responsible for pending verified originals after Squad exits.
- New helpers record their actual filename/hash, target path and expected update hash before launch. Cleanup waits for the identified helper to exit and removes only verified recorded files after startup acknowledgement. Unknown files, linked records and historical directories without ownership metadata are retained. A cleaning checkpoint permits interrupted owned-file removal to resume without recursive directory deletion. Failed helper starts retain their records and report the directory rather than deleting uncertain contents.

## Documentation

- [`docs/INSTALLATION.md`](docs/INSTALLATION.md) — end-user installation and first-run steps
- [`docs/TESTING.md`](docs/TESTING.md) — acceptance gates, smoke tests, and leak/crash checks
- [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) — API, Steam join, queue engine, and Linux/Windows dependency injection
- [`docs/SAFETY.md`](docs/SAFETY.md) — failure handling, power actions, data handling, and incident response
- [`docs/PUBLISHING.md`](docs/PUBLISHING.md) — source/binary publication, assets, licensing, and release hygiene
- [`docs/RELEASING.md`](docs/RELEASING.md) — versioning, checksums, artifacts, and rollback

## License

The repository currently contains an all-rights-reserved [`LICENSE`](LICENSE) notice and the root package is marked `UNLICENSED`. This means the source may be published for review without granting permission to reuse, redistribute, or create derivative projects. It is source-available, not an open-source license. Obtain a lawyer's review before public distribution or commercial enforcement.
