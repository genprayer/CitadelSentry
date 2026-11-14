# Architecture and trust boundaries

## Architectural Decision Record (ADR 0001)

This project strictly adheres to [ADR 0001: Rejected Squad Connection Methods](adr/0001-squad-connection-rejected-methods.md).
Prohibited: console `connect IP:Port`, SendInput console automation, `steam://connect`, `steam://run/393380//...` CLI params, and Synthetic Steam Invites as a complete join solution.
Confirmed joining pipeline for Squad 10.6+:
`Server Browser → FSQServerItem → QueueForServer → Beacon → JoinSession → ClientTravel`.

## Runtime flow

```text
Nuxt UI
  -> Wails bridge (App)
  -> citadel.Client -> Citadel API
  -> seed.Engine -> GameController -> platform.Adapter
  -> Server Browser / Favorites / Queue Engine -> Squad 10.6+
```

The API is authoritative for the visible fleet and server metadata. The connection engine drives the confirmed Squad Server Browser / Favorites pipeline to initiate `QueueForServer` without manual console typing or manual searching.

## Queue engine

`seed.Engine` builds a deterministic queue, owns one cancellable run at a time, refreshes telemetry on a bounded interval, emits a per-second progress clock, and applies the post-seed action once every selected server reaches its target. It does not run that action after timeout, Stop, or failure. `SwitchServer` closes the current Squad process and waits for its exit before launching the next lobby URL. A failed join is terminal for that run; it is not hidden behind an unbounded reconnect loop.

The engine does not infer manual in-game navigation. For hidden game endpoints, confirmation combines fresh client welcome/deployment records, the local player's logged SteamID and exact membership in the selected server's existing public team roster. Roster requests are bounded and cached for 15 seconds; observed client disconnections clear confirmation immediately. This is not a Steam presence stream and does not identify arbitrary manual switches. A player without team membership may remain unconfirmed.

The invitation resolver runs after any required cold startup. Automated missions retry at most twice for Squad's observed busy-session-search warning, refreshing the invitation each time. Other explicit rejection errors remain terminal. Direct joins reserve a cancellable application operation without holding the application operation mutex throughout startup; overlapping joins, mission starts and updates are rejected.

## Platform dependency injection

The platform interface is implemented separately for Linux and Windows. Linux uses its desktop URI opener, process matching, and OS sleep inhibitor. Windows uses the native URI opener, process handling, and Windows power APIs. The shared engine depends only on the interface, so OS-specific commands do not leak into queue logic.

## Configuration

`seed.defaultPostSeedAction` is the single post-seed policy. The old `power.powerActionAfterSeed`, `seed.autoJoinSquadOnSeed`, and `seed.squadExecutablePath` fields are migration-only input from older JSON files and are not written by current releases. A legacy sleep/shutdown value is migrated into the canonical seed action when no explicit seed action exists.

Application settings and recovery records are written atomically. Existing settings require explicit supported values and bounded, unambiguous JSON; a validation failure preserves the current file and cached settings. The desktop uses the public fleet/connection API without an authentication callback or local auth cache.

Linux sleep inhibition obtains a logind descriptor through the existing D-Bus client. The worker owns and closes the descriptor; acquisition receives the run's cancellation context. The Linux URI opener is waited and reaped with a timeout, and Windows URI dispatch calls the shell API directly. These primitives do not prove an in-game connection: an observed Squad process is tracked separately in the session.
