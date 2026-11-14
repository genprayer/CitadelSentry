# Release procedure

The local pipeline is `./scripts/release.sh patch|minor|major|<version>`, using Python 3.10+ and GNU coreutils on the Linux build workstation. It validates the saved version and matching package/Wails metadata, runs source checks, regenerates the configured frontend, builds Linux and Windows artifacts, and prepares a complete local release with checksums. The version must be newer and its release directory must be absent; missing or inconsistent metadata stops the attempt without inventing a version.

A persistent OS lock prevents overlapping releases. Before mutation, a checksummed journal in `.citadel-release-transaction` records exact original metadata and convenience-binary backups. Each intended replacement records its checksum and actual file identity before publication. Failed validation/builds and SIGTERM restore verified originals; interrupted work is recovered before reading inputs on the next invocation. Recovery preserves externally replaced/changed files and keeps the journal when original data or build-group state cannot be verified. The stable `.citadel-release.lock` file remains after process exit; do not delete it to bypass an active build.

Both fresh binaries must exist and differ from their previous build outputs. Complete release contents are staged and moved without replacing a competing creator; staging and releases must be on the same filesystem. Retention runs after committing the new release and considers only canonical version folders containing exactly the two standard assets and a matching checksum file. Unknown directories, extra files and unverified artifacts are preserved.

Before running it:

1. Confirm the working tree contains only intended changes. This project is not allowed to push automatically.
2. Run the complete source and functional gates from [`TESTING.md`](TESTING.md).
3. Review the target version and retention behavior. The script keeps the three newest verified standard local releases; historical or unrecognized folders are outside that deletion scope.
4. Ensure no credentials or private files are inside the release directory.

After a successful build:

```bash
ls -lh releases/v<VERSION>
sha256sum -c releases/v<VERSION>/SHA256SUMS.txt
```

Run the Linux binary on the supported desktop and install the Windows artifact on the dedicated Windows test host. The cross-build and checksum are not substitutes for interactive Steam/Squad acceptance.

Release regression tests run on copied project fixtures with substitute Go/npm/Wails commands: `PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s scripts/tests -p test_release.py -v`. They exercise actual local locks, no-clobber directory moves, SIGTERM/SIGKILL boundaries, verified child cleanup, exact rollback and restart recovery. They do not build, replace or publish the real application. Run real release builds through the verified local resource profile and keep native packaging/desktop acceptance as separate gates.

The in-app updater reads the latest stable release from `genprayer/CitadelSentry`. GitHub release assets must be named `citadel-sentry-v<VERSION>-linux-amd64`, `citadel-sentry-v<VERSION>-windows-amd64.exe`, and `SHA256SUMS.txt`; the checksum file must contain both exact asset names. Keep the embedded `VERSION` equal to the release tag and retain previous known-good releases for rollback. Before publishing version 1.2.2, verify the update flow on installed Windows and Linux copies, including interrupted download, unwritable application directory, and rollback when restart fails.

For rollback, remove the new distribution entry from the public release page and restore the previous known-good artifacts. Do not delete the only remaining copy of a known-good binary. Never use `git push` from this workflow; publish through an explicitly reviewed human-operated release step.
