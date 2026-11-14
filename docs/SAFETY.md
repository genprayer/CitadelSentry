# Safety, failure handling, and incident response

## Scope and warranty

Sentry can launch a game, close a game process, prevent sleep, sleep the computer, or shut the computer down when the user selects those actions. It is not a safety-certified or mission-critical system. Do not run a shutdown/sleep policy on a workstation with unsaved work, unattended maintenance, or another critical workload.

The project must publish a separate end-user agreement and privacy notice before a public distribution. This document is operational guidance, not legal advice and not a warranty disclaimer drafted for a particular jurisdiction.

## User protections

- The seed run is cancellable with Stop.
- A single run owns its cancellation context; a second run cannot silently replace it.
- Process matching is scoped to Squad executable names rather than broad `pkill -f` patterns.
- Configuration and auth writes are atomic to reduce corruption after a crash.
- The UI warns about indefinite retry and about power actions after a multi-server queue.
- No token, auth callback, or local config should be logged or attached to an issue.

## Critical bug procedure

If the application crashes, freezes, repeatedly relaunches Squad, corrupts configuration, or performs an unexpected power action:

1. Press Stop if the UI responds.
2. Terminate Sentry and Squad from the operating system if needed.
3. Disable Sentry auto-start and do not select sleep/shutdown policies.
4. Preserve the binary version, OS version, logs, and a minimal reproduction without including secrets.
5. Report the incident privately to the project maintainer before publishing details that could expose a user or server.
6. Reproduce on a test server/account and add a regression test before releasing a fix.

## Security reporting

Use the private security contact described by the release repository. Do not open a public issue for an access token, authentication bypass, arbitrary process execution, or a vulnerability that affects users before a fix and communication plan exist.
