"""Persist verified originals for local release rollback and restart recovery."""

import hashlib
import json
import math
import os
from pathlib import Path
import re
import shutil
import stat
import tempfile
import uuid

STATE_DIRECTORY = ".citadel-release-transaction"
STATE_FILE = "state.json"
OWNERSHIP_FILE = ".citadel-release-owner"
RELEASE_FILES = ("VERSION", "package.json", "wails.json", "citadel-sentry", "citadel-sentry.exe")
METADATA_FILES = ("VERSION", "package.json", "wails.json")
MAX_JSON_BYTES = 1 << 20
COPY_CHUNK_BYTES = 1 << 20
VERSION_PATTERN = re.compile(r"v?(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\Z")
MAX_VERSION_COMPONENT = (1 << 32) - 1  # Matches updater.parseVersion's ParseUint(..., 32).


def version_parts(value: str) -> tuple[int, int, int]:
    """Parse exactly the stable version syntax supported by the application updater."""
    if not isinstance(value, str):
        raise ValueError("Release version must be a string")
    match = VERSION_PATTERN.fullmatch(value)
    if match is None:
        raise ValueError("Invalid stable release version; expected MAJOR.MINOR.PATCH")
    parts = tuple(int(part) for part in match.groups())
    if any(part > MAX_VERSION_COMPONENT for part in parts):
        raise ValueError("Release version exceeds the updater's component range")
    return parts


def digest(path: Path) -> str:
    """Hash a file with bounded memory."""
    value = hashlib.sha256()
    with path.open("rb") as source:
        for chunk in iter(lambda: source.read(COPY_CHUNK_BYTES), b""):
            value.update(chunk)
    return value.hexdigest()


def regular_info(path: Path):
    """Inspect a replaceable regular file without following a leaf link."""
    info = path.lstat()
    if (not stat.S_ISREG(info.st_mode) or info.st_nlink != 1
            or stat.S_IMODE(info.st_mode) > 0o777):
        raise ValueError(f"Refusing a linked, shared or special file: {path}")
    return info


def _unique_object(pairs):
    result = {}
    seen = set()
    for key, value in pairs:
        if key.casefold() in seen:
            raise ValueError(f"Ambiguous JSON field: {key}")
        seen.add(key.casefold())
        result[key] = value
    return result


def _finite_float(value):
    result = float(value)
    if not math.isfinite(result):
        raise ValueError("JSON contains an unsupported non-finite number")
    return result


def _reject_constant(_value):
    raise ValueError("JSON contains a nonstandard numeric constant")


def read_json_source(path: Path) -> tuple[dict, bytes]:
    """Read a bounded object together with the exact bytes used for its choices."""
    regular_info(path)
    with path.open("rb") as source:
        data = source.read(MAX_JSON_BYTES + 1)
    if len(data) > MAX_JSON_BYTES:
        raise ValueError(f"JSON file exceeds its size limit: {path}")
    value = json.loads(data, object_pairs_hook=_unique_object, parse_float=_finite_float,
                       parse_constant=_reject_constant)
    if not isinstance(value, dict):
        raise ValueError(f"Expected a JSON object: {path}")
    return value, data


def read_json(path: Path) -> dict:
    """Read a checked JSON object without discarding parsing errors."""
    return read_json_source(path)[0]


def sync_directory(path: Path) -> None:
    """Flush directory entries before reporting a durable checkpoint."""
    descriptor = os.open(path, os.O_RDONLY | os.O_DIRECTORY)
    try:
        os.fsync(descriptor)
    finally:
        os.close(descriptor)


def atomic_write(path: Path, data: bytes, mode: int, is_missing=False, token=None, on_prepared=None) -> None:
    """Publish synced bytes, optionally preserving a competing file creator."""
    prefix = ".citadel-release-" + ((token + "-") if token is not None else "")
    descriptor, name = tempfile.mkstemp(prefix=prefix, dir=path.parent)
    temporary = Path(name)
    try:
        with os.fdopen(descriptor, "wb") as output:
            output.write(data)
            output.flush()
            os.fchmod(output.fileno(), mode)
            os.fsync(output.fileno())
            info = os.fstat(output.fileno())
        if on_prepared is not None:
            on_prepared({"device": info.st_dev, "inode": info.st_ino})
        if is_missing:
            os.link(temporary, path)
        else:
            os.replace(temporary, path)
        sync_directory(path.parent)
    finally:
        temporary.unlink(missing_ok=True)


def is_group_running(group_id: int | None) -> bool:
    """Keep recovery read-only while a recorded build group may still be active."""
    if group_id is None:
        return False
    try:
        os.killpg(group_id, 0)
        return True
    except ProcessLookupError:
        return False


class ReleaseState:
    """Own one release journal; never replace unknown current data during recovery."""

    def __init__(self, root: Path):
        self.root = root
        self.folder = root / STATE_DIRECTORY
        self.data = {}

    def save(self) -> None:
        """Persist the current recovery checkpoint atomically."""
        payload = json.dumps(self.data, sort_keys=True, separators=(",", ":")).encode()
        envelope = {"payload": self.data, "sha256": hashlib.sha256(payload).hexdigest()}
        atomic_write(self.folder / STATE_FILE,
                     (json.dumps(envelope, indent=2) + "\n").encode(), 0o600)

    def begin(self, version: str) -> None:
        """Snapshot all originals before any version or convenience-file mutation."""
        self.folder.mkdir(mode=0o700)
        sync_directory(self.root)
        self.data = {"schema": 1, "root": str(self.root), "phase": "snapshotting",
                     "token": uuid.uuid4().hex, "version": version, "groupId": None, "files": {}}
        self.save()
        for name in RELEASE_FILES:
            path = self.root / name
            try:
                info = regular_info(path)
            except FileNotFoundError:
                if name in METADATA_FILES:
                    raise ValueError(f"Required release metadata disappeared: {name}")
                self.data["files"][name] = {"existed": False, "originalHash": None,
                                            "mode": None, "expectedHash": None, "expectedIdentity": None}
                continue
            backup = self.folder / ("original-" + name)
            shutil.copy2(path, backup)
            with backup.open("rb") as saved:
                os.fsync(saved.fileno())
            original_hash = digest(backup)
            if digest(path) != original_hash:
                raise ValueError(f"Original changed while being backed up: {path}")
            self.data["files"][name] = {"existed": True, "originalHash": original_hash,
                                        "mode": stat.S_IMODE(info.st_mode), "expectedHash": None, "expectedIdentity": None}
        self.data["phase"] = "prepared"
        self.save()

    def load(self) -> bool:
        """Validate a previous journal before interpreting its state."""
        if not self.folder.exists():
            if self.folder.is_symlink():
                raise ValueError("Release journal directory is a broken link")
            return False
        if self.folder.is_symlink() or not self.folder.is_dir():
            raise ValueError("Release journal path is not an owned directory")
        if not any(self.folder.iterdir()):
            # Creation and final removal both have an empty-directory checkpoint.
            self.folder.rmdir()
            sync_directory(self.root)
            return False
        envelope = read_json(self.folder / STATE_FILE)
        if set(envelope) != {"payload", "sha256"} or not isinstance(envelope["payload"], dict):
            raise ValueError("Invalid release recovery envelope")
        payload = json.dumps(envelope["payload"], sort_keys=True, separators=(",", ":")).encode()
        if hashlib.sha256(payload).hexdigest() != envelope["sha256"]:
            raise ValueError("Release recovery journal checksum is invalid")
        self.data = envelope["payload"]
        if (set(self.data) != {"schema", "root", "phase", "token", "version", "groupId", "files"}
                or type(self.data["schema"]) is not int or self.data["schema"] != 1
                or self.data["root"] != str(self.root)
                or not isinstance(self.data["phase"], str)
                or self.data["phase"] not in {"snapshotting", "prepared", "publishing", "restored", "committed"}
                or not isinstance(self.data["token"], str)
                or re.fullmatch(r"[0-9a-f]{32}", self.data["token"]) is None):
            raise ValueError("Invalid release recovery journal")
        parts = version_parts(self.data["version"])
        if self.data["version"] != ".".join(map(str, parts)):
            raise ValueError("Release recovery version is not canonical")
        group_id = self.data["groupId"]
        if group_id is not None and (type(group_id) is not int or group_id <= 0):
            raise ValueError("Invalid release build-group identity")
        records = self.data["files"]
        if not isinstance(records, dict) or not set(records).issubset(RELEASE_FILES):
            raise ValueError("Invalid release backup paths")
        if self.data["phase"] != "snapshotting" and set(records) != set(RELEASE_FILES):
            raise ValueError("Incomplete release recovery records")
        for name, record in records.items():
            if (not isinstance(record, dict)
                    or set(record) != {"existed", "originalHash", "mode", "expectedHash", "expectedIdentity"}
                    or type(record["existed"]) is not bool):
                raise ValueError(f"Invalid release backup record: {name}")
            for key in ("originalHash", "expectedHash"):
                value = record[key]
                if value is not None and (not isinstance(value, str) or re.fullmatch(r"[0-9a-f]{64}", value) is None):
                    raise ValueError(f"Invalid release backup checksum: {name}")
            if record["existed"]:
                if record["originalHash"] is None or type(record["mode"]) is not int or not 0 <= record["mode"] <= 0o777:
                    raise ValueError(f"Incomplete release original metadata: {name}")
            elif record["originalHash"] is not None or record["mode"] is not None:
                raise ValueError(f"Conflicting release original metadata: {name}")
            if self.data["phase"] != "snapshotting" and name in METADATA_FILES and not record["existed"]:
                raise ValueError(f"Release metadata has no recorded original: {name}")
            identity = record["expectedIdentity"]
            if identity is not None:
                if (not isinstance(identity, dict) or set(identity) != {"device", "inode"}
                        or type(identity["device"]) is not int or identity["device"] < 0
                        or type(identity["inode"]) is not int or identity["inode"] <= 0
                        or record["expectedHash"] is None):
                    raise ValueError(f"Invalid prepared release file identity: {name}")
        return True

    def write(self, name: str, data: bytes, mode: int) -> None:
        """Checkpoint intended bytes before an atomic guarded mutation."""
        record = self.data["files"][name]
        if record["expectedHash"] is not None:
            raise ValueError(f"Release file was already prepared: {name}")
        observed_hash = self._check_current(name, record)
        if record["existed"] and observed_hash is None:
            raise ValueError(f"Recorded release original disappeared: {name}")
        record["expectedHash"] = hashlib.sha256(data).hexdigest()
        self.save()

        def checkpoint_identity(identity):
            if self._check_current(name, record) != observed_hash:
                raise ValueError(f"Release file appeared or changed during preparation: {name}")
            record["expectedIdentity"] = identity
            self.save()

        atomic_write(self.root / name, data, mode, is_missing=observed_hash is None,
                     token=self.data["token"], on_prepared=checkpoint_identity)

    def _check_current(self, name, record):
        path = self.root / name
        try:
            info = regular_info(path)
            current_hash = digest(path)
        except FileNotFoundError:
            return None
        if current_hash not in {record["originalHash"], record["expectedHash"]}:
            raise ValueError(f"Release file changed outside the transaction; preserving it: {path}")
        if current_hash != record["originalHash"] and record["expectedIdentity"] != {"device": info.st_dev, "inode": info.st_ino}:
            raise ValueError(f"Release file was replaced outside the transaction; preserving it: {path}")
        return current_hash

    def verify_expected(self, names) -> None:
        """Confirm prepared inputs before labelling artifacts or committing recovery."""
        for name in names:
            path = self.root / name
            regular_info(path)
            expected = self.data["files"][name]["expectedHash"]
            if expected is None or digest(path) != expected:
                raise ValueError(f"Prepared release file changed; refusing commit: {path}")
            self._check_current(name, self.data["files"][name])

    def recover(self) -> None:
        """Restore verified originals after interruption, or clean a committed release."""
        if is_group_running(self.data["groupId"]):
            raise RuntimeError("A recorded build group is still active; recovery remains pending")
        if self.data["phase"] in {"snapshotting", "restored", "committed"}:
            self.cleanup()
            return
        for name, record in self.data["files"].items():
            self._check_current(name, record)
            if record["existed"]:
                backup = self.folder / ("original-" + name)
                regular_info(backup)
                if digest(backup) != record["originalHash"]:
                    raise ValueError(f"Release original backup checksum is invalid: {name}")
        self._remove_partial_release()
        for name, record in self.data["files"].items():
            self._check_current(name, record)
            path = self.root / name
            if record["existed"]:
                if path.exists() and digest(path) == record["originalHash"]:
                    continue
                backup = self.folder / ("original-" + name)
                atomic_write(path, backup.read_bytes(), record["mode"], is_missing=not path.exists(), token=self.data["token"])
            elif path.exists():
                path.unlink()
                sync_directory(path.parent)
        self.data["phase"] = "restored"
        self.save()
        self.cleanup()

    def _remove_partial_release(self):
        targets = (self.folder / "staged-release", self.root / "releases" / ("v" + self.data["version"]))
        for target in targets:
            if not target.exists():
                continue
            owner = target / OWNERSHIP_FILE
            is_private_staging = target == targets[0]
            if target.is_symlink() or not target.is_dir():
                raise ValueError("Partial release path is linked or special")
            if owner.exists() and owner.read_text() != self.data["token"]:
                continue  # A competing creator owns this release, not this transaction.
            if not owner.exists() and not is_private_staging:
                continue
            allowed = set(asset_names(self.data["version"])) | {"SHA256SUMS.txt", OWNERSHIP_FILE}
            if any((item.name not in allowed and not item.name.startswith(".citadel-release-"))
                   or not item.is_file() or item.is_symlink() for item in target.iterdir()):
                raise ValueError("Partial release contains unrecognized files; preserving it")
            for item in target.iterdir():
                item.unlink()
            target.rmdir()
            sync_directory(target.parent)

    def cleanup(self) -> None:
        """Remove only this validated transaction's known backup files."""
        if self.data["phase"] == "committed":
            owner = self.root / "releases" / ("v" + self.data["version"]) / OWNERSHIP_FILE
            if owner.exists():
                if owner.read_text() != self.data["token"]:
                    raise ValueError("Committed release ownership cannot be verified")
                owner.unlink()
                sync_directory(owner.parent)
        allowed = {STATE_FILE} | {"original-" + name for name in RELEASE_FILES}
        if any((item.name not in allowed and not item.name.startswith(".citadel-release-"))
               or not item.is_file() or item.is_symlink() for item in self.folder.iterdir()):
            raise ValueError("Release journal contains unrecognized files; preserving it")
        for item in self.root.glob(".citadel-release-" + self.data["token"] + "-*"):
            regular_info(item)
            item.unlink()
        for item in self.folder.iterdir():
            if item.name != STATE_FILE:
                item.unlink()
        # Keep the completion checkpoint until all backups have been removed.
        sync_directory(self.folder)
        (self.folder / STATE_FILE).unlink()
        sync_directory(self.folder)
        self.folder.rmdir()
        sync_directory(self.root)


def asset_names(version: str) -> tuple[str, str]:
    """Return the exact portable asset names consumed by the application updater."""
    return (f"citadel-sentry-v{version}-linux-amd64",
            f"citadel-sentry-v{version}-windows-amd64.exe")
