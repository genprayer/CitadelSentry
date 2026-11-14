# Installation and first run

## Linux

1. Use a supported 64-bit Linux desktop with a working graphical session.
2. Install the runtime libraries required by the Wails/WebKitGTK build used for the release. Distribution package names differ; on Ubuntu/Debian, verify the installed WebKitGTK 4.1 and GTK 3 runtime packages before launching.
3. Download the Linux artifact and its checksum file from the same release.
4. Verify the SHA-256 hash, mark the binary executable, and start it from a terminal once so startup errors are visible.
5. Open Settings and confirm the endpoint and sleep-inhibition preference.
6. Start with one low-risk test server. Confirm Steam is running and that clicking Join opens Steam/Squad through the lobby link.

## Windows

1. Use a supported 64-bit Windows installation with Steam and the Microsoft WebView2 runtime.
2. Download the Windows artifact and its checksum file from the same release.
3. Verify the hash in PowerShell:

```powershell
Get-FileHash .\citadel-sentry-v<VERSION>-windows-amd64.exe -Algorithm SHA256
```

4. Start the executable. If SmartScreen warns about an unsigned binary, stop and verify the download and publisher identity before choosing whether to continue.
5. Confirm that Steam and Squad are installed for the same user session. Sentry launches a Steam lobby URL; it does not require a manually entered Squad executable path.
6. Test Join on one server before enabling a multi-server queue or a power action.

## Authentication

The public fleet and direct Join path use the configured Citadel API. Desktop SSO is optional. When SSO is used, the app starts a temporary listener on loopback, accepts only the configured Citadel web origin, requires POST plus a CSRF state value, and closes the listener after completion or timeout.

Never paste a token, callback payload, or `config.json` into an issue or chat. To sign out, use the app's logout action; deleting the local auth cache is a fallback only after the app is closed.

## First-run acceptance

Before inviting another user, verify:

- server list loads without a static fallback;
- Join opens Steam/Squad on the intended server;
- Stop cancels an active seed and releases sleep inhibition;
- a two-server queue closes Squad before launching the next lobby;
- timeout, target completion, and post-queue actions are visible;
- no secrets are present in logs, screenshots, release archives, or the repository.
