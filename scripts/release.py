"""Build a local release with explicit versions and checked rollback; never publish it."""

import fcntl
import hashlib
import json
import os
from pathlib import Path
import re
import signal
import stat
import subprocess
import sys

from release_state import (OWNERSHIP_FILE, ReleaseState, asset_names, atomic_write,
                           digest, is_group_running, read_json_source, regular_info,
                           sync_directory, version_parts, MAX_JSON_BYTES)

MAX_RELEASES = 3
CHILD_EXIT_TIMEOUT_SECONDS = 5
LOCK_FILE = ".citadel-release.lock"
VALIDATION_COMMANDS = (("go", "vet", "-tags", "webkit2_41", "./..."),
                       ("go", "test", "-tags", "webkit2_41", "./..."))


def release_inputs(root: Path, action: str):
    """Require consistent saved metadata and an explicitly supported target version."""
    regular_info(root / "VERSION")
    with (root / "VERSION").open("rb") as source:
        version_data = source.read(MAX_JSON_BYTES + 1)
    if len(version_data) > MAX_JSON_BYTES:
        raise ValueError("VERSION exceeds the metadata size limit")
    current = version_data.decode().strip()
    parts = version_parts(current)
    current = ".".join(map(str, parts))
    package, package_data = read_json_source(root / "package.json")
    wails, wails_data = read_json_source(root / "wails.json")
    if package.get("version") != current or not isinstance(wails.get("info"), dict):
        raise ValueError("Saved VERSION/package/Wails metadata is missing or inconsistent")
    if wails["info"].get("productVersion") != current:
        raise ValueError("Saved Wails productVersion disagrees with VERSION")
    output = wails.get("outputfilename")
    if not isinstance(output, str) or re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9._-]*", output) is None:
        raise ValueError("Wails outputfilename must name an explicit local build artifact")
    frontend = wails.get("frontend:dir")
    if (not isinstance(frontend, str) or not frontend or Path(frontend).is_absolute()
            or not (root / frontend).resolve().is_relative_to(root)
            or not (root / frontend).is_dir()):
        raise ValueError("Wails frontend:dir must identify an existing project directory")
    if action in {"patch", "minor", "major"}:
        index = {"major": 0, "minor": 1, "patch": 2}[action]
        target_parts = list(parts)
        target_parts[index] += 1
        for following in range(index + 1, 3):
            target_parts[following] = 0
        target = ".".join(map(str, target_parts))
    else:
        target = ".".join(map(str, version_parts(action)))
    if version_parts(target) <= parts:
        raise ValueError("A release version must be newer than the saved version")
    target_folder = root / "releases" / ("v" + target)
    if target_folder.exists() or target_folder.is_symlink():
        raise ValueError(f"Release already exists; refusing replacement: {target_folder}")
    package["version"] = target
    wails["info"]["productVersion"] = target
    data = {"VERSION": (target + "\n").encode(),
            "package.json": (json.dumps(package, indent=2, ensure_ascii=False) + "\n").encode(),
            "wails.json": (json.dumps(wails, indent=2, ensure_ascii=False) + "\n").encode()}
    original_hashes = {name: hashlib.sha256(raw).hexdigest() for name, raw in
                       (("VERSION", version_data), ("package.json", package_data), ("wails.json", wails_data))}
    return current, target, output, frontend, data, original_hashes


def run_command(state: ReleaseState, lock_fd: int, arguments) -> None:
    """Checkpoint an owned build group before releasing its startup gate."""
    print("Running: " + " ".join(arguments), flush=True)
    child = subprocess.Popen(("/bin/sh", "-c", 'read -r ready && [ "$ready" = ready ] && exec "$@"', "release-worker", *arguments),
                             cwd=state.root, stdin=subprocess.PIPE, pass_fds=(lock_fd,),
                             start_new_session=True)
    try:
        state.data["groupId"] = child.pid
        state.save()
        child.stdin.write(b"ready\n")
        child.stdin.close()
        result = child.wait()
    finally:
        if child.poll() is None:
            os.killpg(child.pid, signal.SIGTERM)
            try:
                child.wait(timeout=CHILD_EXIT_TIMEOUT_SECONDS)
            except subprocess.TimeoutExpired:
                os.killpg(child.pid, signal.SIGKILL)
                child.wait(timeout=CHILD_EXIT_TIMEOUT_SECONDS)
        if child.stdin is not None and not child.stdin.closed:
            child.stdin.close()
    if is_group_running(child.pid):
        raise RuntimeError("Build command left a live process group; recovery remains pending")
    state.data["groupId"] = None
    state.save()
    if result != 0:
        raise subprocess.CalledProcessError(result, arguments)


def publish_local(state: ReleaseState, output_name: str, lock_fd: int) -> None:
    """Publish only complete checked builds and record root-copy recovery obligations."""
    version = state.data["version"]
    state.verify_expected(("VERSION", "package.json", "wails.json"))
    sources = (state.root / "build/bin" / output_name, state.root / "build/bin" / (output_name + ".exe"))
    for source in sources:
        info = regular_info(source)
        if info.st_size == 0:
            raise ValueError(f"Build artifact is empty: {source}")
    state.data["phase"] = "publishing"
    state.save()
    parent = state.root / "releases"
    parent.mkdir(exist_ok=True)
    if parent.is_symlink():
        raise ValueError("Refusing a linked releases directory")
    if parent.stat().st_dev != state.folder.stat().st_dev:
        raise ValueError("Releases and staging must share a filesystem for an atomic move")
    target = state.folder / "staged-release"
    target.mkdir()
    atomic_write(target / OWNERSHIP_FILE, state.data["token"].encode(), 0o600, is_missing=True)
    checksums = []
    prepared_copies = []
    for source, name, convenience in zip(sources, asset_names(version), ("citadel-sentry", "citadel-sentry.exe")):
        data = source.read_bytes()
        mode = stat.S_IMODE(source.stat().st_mode)
        atomic_write(target / name, data, mode, is_missing=True)
        checksums.append(f"{digest(target / name)}  {name}\n")
        prepared_copies.append((convenience, data, mode))
    atomic_write(target / "SHA256SUMS.txt", "".join(checksums).encode(), 0o644, is_missing=True)
    destination = parent / ("v" + version)
    # A no-replace move preserves a directory that appeared after validation.
    # Confirm the move happened: GNU mv can also exit 0 when it skipped one.
    run_command(state, lock_fd, ("mv", "--no-clobber", "-T", str(target), str(destination)))
    if target.exists():
        raise ValueError("A release directory appeared during publication; preserving it")
    sync_directory(parent)
    for convenience, data, mode in prepared_copies:
        state.write(convenience, data, mode)
    state.verify_expected(state.data["files"])
    state.data["phase"] = "committed"
    state.save()
    state.cleanup()


def prune_releases(root: Path) -> None:
    """Remove only verified standard release files, preserving unknown directories/data."""
    candidates = []
    for folder in (root / "releases").iterdir():
        if folder.is_symlink() or not folder.is_dir() or not folder.name.startswith("v"):
            continue
        try:
            version = folder.name[1:]
            parts = version_parts(version)
            if folder.name != "v" + ".".join(map(str, parts)):
                continue
            names = asset_names(version)
            if {p.name for p in folder.iterdir()} != set(names) | {"SHA256SUMS.txt"}:
                continue
            lines = (folder / "SHA256SUMS.txt").read_text().splitlines()
            expected = {name: digest(folder / name) for name in names}
            if len(lines) != len(names) or {tuple(line.split()) for line in lines} != {(value, name) for name, value in expected.items()}:
                continue
            for path in folder.iterdir():
                regular_info(path)
            candidates.append((parts, folder, expected))
        except (OSError, ValueError):
            continue  # Unknown or unverified entries are outside the retention scope.
    for _, folder, expected in sorted(candidates)[:-MAX_RELEASES]:
        for name, value in expected.items():
            regular_info(folder / name)
            if digest(folder / name) != value:
                raise ValueError("Release changed while preparing retention; preserving it")
        for name in expected:
            (folder / name).unlink()
        (folder / "SHA256SUMS.txt").unlink()
        folder.rmdir()  # Refuses to remove a directory if unknown files appeared.
        sync_directory(folder.parent)


def interrupted(signum, _frame):
    """Unwind the owned command before attempting restoration."""
    raise KeyboardInterrupt(f"Release interrupted by signal {signum}")


def main() -> int:
    """Run local validation/build/publication under one persistent OS lock."""
    root = Path(__file__).resolve().parent.parent
    if sys.platform != "linux":
        print("The local Linux/Windows cross-release pipeline requires a Linux build host", file=sys.stderr)
        return 1
    state = ReleaseState(root)
    if len(sys.argv) > 2:
        print("Usage: release.sh patch|minor|major|<version>", file=sys.stderr)
        return 1
    signal.signal(signal.SIGINT, interrupted)
    signal.signal(signal.SIGTERM, interrupted)
    lock_fd = os.open(root / LOCK_FILE, os.O_RDWR | os.O_CREAT | os.O_NOFOLLOW, 0o600)
    try:
        fcntl.flock(lock_fd, fcntl.LOCK_EX | fcntl.LOCK_NB)
        if state.load():
            state.recover()
        current, target, output, frontend, changes, original_hashes = release_inputs(root, sys.argv[1] if len(sys.argv) == 2 else "patch")
        print(f"Local release: {current} -> {target}", flush=True)
        state.begin(target)
        try:
            if any(state.data["files"][name]["originalHash"] != value for name, value in original_hashes.items()):
                raise ValueError("Release inputs changed during preparation; preserving the actual saved values")
            for command in VALIDATION_COMMANDS:
                run_command(state, lock_fd, command)
            for check in ("typecheck", "lint"):
                run_command(state, lock_fd, ("npm", "--prefix", frontend, "run", check))
            for name, data in changes.items():
                state.write(name, data, state.data["files"][name]["mode"])
            run_command(state, lock_fd, ("npm", "--prefix", frontend, "run", "generate"))
            for platform, suffix in (("linux/amd64", ""), ("windows/amd64", ".exe")):
                artifact = root / "build/bin" / (output + suffix)
                previous_hash = None
                if artifact.exists() or artifact.is_symlink():
                    regular_info(artifact)
                    previous_hash = digest(artifact)
                tags = ("-tags", "webkit2_41") if not suffix else ()
                run_command(state, lock_fd, ("wails", "build", *tags, "-platform", platform, "-s"))
                regular_info(artifact)
                if digest(artifact) == previous_hash:
                    raise ValueError(f"Wails returned without replacing the previous artifact: {artifact}")
            publish_local(state, output, lock_fd)
        except (OSError, ValueError, RuntimeError, subprocess.SubprocessError, KeyboardInterrupt):
            if state.folder.exists():
                state.recover()
            raise
        try:
            prune_releases(root)
        except (OSError, ValueError) as error:
            print(f"Local release saved in releases/v{target}; retention is incomplete: {error}", file=sys.stderr)
            return 1
        print(f"Local release completed: releases/v{target}; nothing uploaded", flush=True)
        return 0
    except (OSError, ValueError, RuntimeError, subprocess.SubprocessError, KeyboardInterrupt) as error:
        print(f"Release stopped: {error}", file=sys.stderr)
        return 1
    finally:
        os.close(lock_fd)


if __name__ == "__main__":
    sys.exit(main())
