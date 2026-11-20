# Changelog

## 1.3.2 — 2026-11-20

- **Crash-Free Priority Handling**: Catch all process priority and platform errors (`ERROR_ACCESS_DENIED`, snapshot errors, invalid handles) in `applySquadPriorityForRun`. Log warnings and continue seeding normally instead of aborting with `StateFailed` ("СИДИНГ НЕ УДАЛСЯ").
- **Transactional FPS Updates & Bounded History**: Implement bounded ring-buffer (`maxPreviousPatchesPerFile = 8`) in `BackupEntry.PreviousPatches` with SHA-256 validation. Eliminate naming index assumptions, supporting reliable restoration across crashes or skipped secondary configs while preserving player mid-session changes.
- **Dynamic Potato Mode Clamping**: Boost to 60 FPS during server loading and clamp down to 15 FPS once in-game connection is confirmed. Emit `squad:potato_fps_error` to UI upon update failures without breaking the seed run.
- **UE4 Engine.ini Deep Optimization**: Disable volumetric fog, volumetric clouds, distance field shadows, cloth physics, and cap CPU particles to 1 in Potato Mode.
- **GPU Compatibility**: Remove forced AMD Frame Generation to avoid GPU overhead and ensure compatibility across older GPU architectures.
- **Anonymous Technical Telemetry & One-Time Consent Modal**: Introduce an opt-in technical telemetry subsystem for hardware and memory monitoring without collecting any personal data. Includes a one-time consent modal on first run after update, and a settings toggle in General Settings.


## Unreleased — publication blocked pending native acceptance

- In app.go, JoinServerDirect now verifies in-game invitation processing before returning success to the UI. If Squad immediately rejects the invitation (e.g. lobby build mismatch, missing EOS room ID, or busy session search), the failure is surfaced directly to the user rather than claiming a successful join upon simple URI handoff.
- On Linux, dispatch Steam links through the installed Steam CLI, falling back to the desktop URI handler only when the CLI is absent. A failed dispatch is not repeated through another handler. This transport change has automated coverage but has not established successful in-game joining.
- Report the observed Steam lobby/client build mismatch as the cause of a rejected invitation, preserving it when Squad subsequently reports a missing EOS routing attribute. Ignore mismatches from earlier completed lobby searches.
- Stop waiting when Squad explicitly reports unsuccessful invitation acceptance. This improves failure reporting; it does not repair joining or establish native Linux/Windows acceptance. See `docs/RELEASE_ACCEPTANCE.md`.

## 1.2.8 — 2026-10-23

- Launch Squad without Sentry's MoviePlayer command-line overrides, removing the custom-argument confirmation caused by those overrides. Automatic intro skipping is not provided.
- Resolve the current Steam invitation after cold startup and online login. Retry up to twice if Squad explicitly ignores an invitation because a session search is busy; cancellation interrupts the retry delay.
- Support connection confirmation when the public fleet API hides the game endpoint: require fresh client load records, the local player's logged SteamID and an exact match in the selected server's public team roster. Waiting, missing team membership and disconnections do not count as seeding.
- Show preparation, connection and transition states in the header and server cards instead of labeling every running workflow as active seeding. Surface stop failures and prevent overlapping direct joins, seeding and application updates.
- Known limitation: native Linux/Squad checks still reject received invitations with an absent or non-string RedpointEOSRoomId. This release does not claim to repair that rejection. Native Windows 10/11 joining, intro behavior and in-game FPS remain unverified.

## 1.2.7 — 2026-10-23

- Distinguish per-server targets/time budgets, timeout rules for the current server, and the single action after every selected server succeeds. Show a live explanation of the selected mission policy and describe global settings as defaults for new missions.
- Render select-field hints and associate labels/descriptions with their controls; these hints were previously passed to BaseSelect but never displayed.
- Track servers skipped after a timeout. A queue with skipped servers ends as timed out and cannot execute its successful-queue action, regardless of whether its last server reaches the target.

## 1.2.6 — 2026-10-23

- Start each server's seed timer only after fresh Squad client records confirm the requested IP/game port, the server welcome and the deployment-menu load. Startup, menus, queues and interrupted connections do not consume seeding time.
- Launch a stopped Squad separately and wait for its main menu and successful online login before dispatching the Steam invitation. Queue transitions retain verified process exit before restarting and confirming the next server.
- Surface rejected invitations and connection observation failures instead of treating a running game as a successful connection. Apply a separate bounded connection deadline without executing mission timeout actions for an unconfirmed join.
- Hide the mission end marker while connecting and use confirmed remaining time after a pause. Explain the waiting state in Russian and English.
- Pass MoviePlayer options to avoid waiting for startup movies after loading and permit skipping. Automatic suppression of Squad's custom intro and native Windows behavior remain unverified; Steam may require launch-argument confirmation. The observed RedpointEOSRoomId invitation rejection is not repaired by these changes.

## 1.2.5 — 2026-10-23

- Show the current seeding launch blocker next to the disabled button, including closing Squad or disabling Potato Mode. Keep the protection against changing a running game's graphics configuration.
- Describe direct join dispatch accurately in logs: handing a URI to Steam does not confirm an in-game connection.
- The observed Steam invitation rejection for an absent or non-string RedpointEOSRoomId remains unresolved. Its source has not been isolated to lobby metadata or EOS/Proton handling.

## 1.2.4 — 2026-10-23

- Recognize Squad's observed `/Script/Squad.SQGameUserSettings` section for FPS status, patching and legacy backup detection.
- Discover Proton configuration in registered external Steam libraries, Steam Flatpak and custom XDG data locations; deduplicate library aliases and prefer libraries with an installed Squad manifest over stale prefixes.
- Refresh config discovery when requesting status after Squad creates its first settings file, while retaining active recovery ownership.
- Keep the directory-migration regression isolated from the real user's installed game configuration.

## 1.2.3 — 2026-10-23

This release includes recovery and lifecycle changes covered by automated tests.
Native Windows acceptance and in-game FPS/load measurements remain outstanding.

- Recover previously existing, missing Squad settings files from verified backups.
- Reject unknown or ambiguous Low Performance Mode settings and roll back partial writes.
- Preserve unrelated player configuration changes and leave Input.ini untouched.
- Handle concurrent seeding, stopping, recovery and application updates more conservatively.
- Keep unknown game process state and native-operation failures visible.
- Correct the remaining duration when starting with a selected calendar deadline.
- Show log read/clear failures and ignore stale responses after clearing logs.
- Validate update metadata and files, confirm replacement startup and limit helper cleanup to owned files.

The cause of the reported missing .pak/mod error has not been established.
Updates from 1.2.2 are available through the application settings; older versions
without the updater require a manual installation.
