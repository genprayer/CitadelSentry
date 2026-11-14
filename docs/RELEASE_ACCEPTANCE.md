# Native acceptance required before publication

Publication of any new Sentry release or application update is blocked by the
user's instruction of 2026-10-24 until the complete workflow is fixed and tested.
Local development builds and automated checks are permitted. Version 1.2.8 is
already public; its existence does not establish acceptance.

Record the exact executable hash, OS, Squad build, server IDs, configuration
backup identity, client/server timestamps and evidence for each native run.
Mocks, cross-compilation, an API response or successful URI dispatch cannot pass
a native scenario. An unavailable test system remains NOT RUN.

| Scenario | Linux / Proton | Windows 10 | Windows 11 |
| --- | --- | --- | --- |
| Cold launch, intro completed without intervention, actual target join | FAIL: observed incompatible lobby build | NOT RUN | NOT RUN |
| Join with Squad already open, including different menu tabs | FAIL: original API invitation rejected; full tab matrix NOT RUN | NOT RUN | NOT RUN |
| Timer starts only after exact target membership | Automated coverage only | NOT RUN | NOT RUN |
| Successful target advances through every selected server | NOT RUN | NOT RUN | NOT RUN |
| Per-server timeout and selected queue policy | Automated coverage only | NOT RUN | NOT RUN |
| Stop during startup, connection, active play and transition | Automated coverage only | NOT RUN | NOT RUN |
| Disconnect / game exit / application restart and recovery | Automated coverage only | NOT RUN | NOT RUN |
| Potato Mode applies, game obeys FPS limit, original settings return | NOT RUN | NOT RUN | NOT RUN |
| Preserve unrelated config edits and Input.ini | Automated coverage only | NOT RUN | NOT RUN |
| Terminal action executes exactly once under the selected policy | Automated coverage only | NOT RUN | NOT RUN |
| Update replacement, restart and recovery on the actual platform | NOT RUN | NOT RUN | NOT RUN |

Real sleep/shutdown tests require a designated disposable test system. Do not
power down the user's active workstation or the shared game-server host.

## Current observed blocker

The client log rejects the Steam invitation's build before the subsequent
RedpointEOSRoomId error: lobby 687916 versus client 694944. Read-only Steam SDK
queries confirmed that the supplied lobbies exist and contain the EOS routing
fields. The current public OpenAPI contract accepts only the exact `serverName`
for link creation; it exposes no operation to edit lobby builds or register
servers. A new link does not by itself establish a repaired lobby.

Isolated client-owned lobby experiments also failed to establish native
membership. Both a live SDK helper and a helper that exited before invitation
dispatch led to an unsuccessful Steam invitation search in Squad. The helper
receiving a Steam callback did not prevent the game receiving one; the native
log subsequently showed that game's search failure. No such workaround has
been added to the application.

A one-minute cold-start diagnostic ended before online readiness; the game
subsequently logged menu load and successful authentication without input from
the diagnostic. This does not pass the intro/complete cold-join scenario.

The unreleased log parser now preserves the observed build incompatibility and
recognizes explicit unsuccessful invite acceptance. Both `internal/game` and
`internal/seed` passed local race-enabled tests after this change. In `app.go`,
`JoinServerDirect` now observes in-game invitation processing rather than claiming
success upon simple URI handoff, surfacing immediate lobby build rejection or invite
failures to the UI. The native handoff experiment exercised the rejection path and
failed promptly; this is failure-detection evidence, not a successful join.

## Additional native diagnostics (2026-10-24)

Linux Steam CLI dispatch delivered the original provider invitation to the game,
but did not establish target membership. Direct CLI dispatch is now covered by
platform tests; it is not evidence that the Join button works end to end.

A process-scoped diagnostic matchmaking build override allowed the original
invitation to pass its initial build check. Squad subsequently logged an accepted
session invitation, but no server welcome or deployment-menu load followed.
The override is not part of application code or persistent launch settings.

All three public CITADEL servers were visible in Squad's own browser. Selecting
AAS+ and using its native Join button reached the queue, then timed out against
the server's beacon endpoint. That run used the experimental build override and
therefore needs a normal-client control before drawing conclusions about the
network path. Read-only checks found the server process listening on that beacon
port and an enabled inbound UDP allow rule. Those checks do not prove external
reachability. No firewall or game-server changes were made for this diagnostic.

The client also logged GPU timeouts. No successful Sentry-driven connection,
complete seed queue, or native Windows 10/11 run has been established.

A subsequent normal-build control used only `-nosound`, without the experimental
matchmaking override. It stalled during startup after a logged GPU timeout,
before menu/login readiness. The owned test process was stopped. This incomplete
control establishes neither successful joining nor a normal-build beacon failure.

A further normal-build run passed process-scoped Engine CLI overrides intended
to disable AMD frame generation and its swapchain. Menu load and online login
completed, and the game logged the normal client build `0x000a9aa0`. It still
logged six GPU timeouts; application of the requested CVars was not confirmed.
Native mouse input could not be reliably targeted in that run, so no manual join
is credited. The owned process and temporary input devices were removed, with
keyboard layout restored. No persistent game setting was changed by this probe.

Fresh provider requests and read-only Steam metadata queries subsequently
confirmed `buildid=687916` for all three exact server names (IDs 1, 2 and 6).
The published API still exposes only `serverName` for link creation and no
lobby-build update operation. Repairing this provider-generated invitation path
requires a provider-side change or access to its implementation. Such a change
must still be followed by successful native joining and the full acceptance matrix.
