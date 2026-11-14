"""Exercise release failure/recovery without building or publishing real binaries."""

import hashlib
import ctypes
import importlib.util
import json
import os
from pathlib import Path
import shutil
import signal
import subprocess
import tempfile
import time
import unittest


SCRIPTS = Path(__file__).resolve().parents[1]
STUB_SOURCE = '''#!/usr/bin/python3
import json, os, sys, time
from pathlib import Path
name = Path(sys.argv[0]).name
args = sys.argv[1:]
with open("commands.jsonl", "a") as trace:
    trace.write(json.dumps([name, args]) + "\\n")
stage = name
if name == "npm" and args[-1] == "generate": stage = "generate"
if name == "wails": stage = args[args.index("-platform") + 1]
if os.environ.get("RELEASE_FIXTURE_BLOCK") == stage:
    Path("blocked.json").write_text(json.dumps({"pid": os.getpid(), "stage": stage}))
    deadline = time.monotonic() + 5
    while not Path("unblock").exists():
        if time.monotonic() > deadline: sys.exit(19)
        time.sleep(0.01)
if os.environ.get("RELEASE_FIXTURE_FAIL") == stage: sys.exit(17)
if os.environ.get("RELEASE_FIXTURE_SKIP_OUTPUT") == stage: sys.exit(0)
if name == "wails":
    output = Path("build/bin")
    output.mkdir(parents=True, exist_ok=True)
    filename = json.loads(Path("wails.json").read_text())["outputfilename"]
    if stage == "windows/amd64": filename += ".exe"
    target = output / filename
    target.write_bytes((stage + ":" + Path("VERSION").read_text()).encode())
    target.chmod(0o755)
    if stage == "windows/amd64" and os.environ.get("RELEASE_FIXTURE_COMPETING_RELEASE"):
        directory = Path("releases/v1.2.3")
        directory.mkdir(parents=True)
        (directory / "keep.txt").write_bytes(b"competing user data")
'''


class ReleaseTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(prefix="citadel-release-fixture-")
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        scripts = self.root / "scripts"
        scripts.mkdir()
        for source in SCRIPTS.glob("release*"):
            if source.is_file():
                shutil.copy2(source, scripts / source.name)
        self.script = scripts / "release.sh"
        self.originals = {
            "VERSION": b"1.2.2\n",
            "package.json": b'{"name":"fixture", "version":"1.2.2"}\n',
            "wails.json": b'{"outputfilename":"citadel-sentry", "frontend:dir":"frontend", "info":{"productVersion":"1.2.2"}}\n',
            "citadel-sentry": b"original-linux",
            "citadel-sentry.exe": b"original-windows",
        }
        for name, data in self.originals.items():
            (self.root / name).write_bytes(data)
        (self.root / "frontend").mkdir()
        tools = self.root / "fixture-tools"
        tools.mkdir()
        stub = tools / "fake-tool"
        stub.write_text(STUB_SOURCE)
        stub.chmod(0o755)
        for name in ("go", "npm", "wails"):
            (tools / name).symlink_to(stub)
        self.env = {**os.environ, "PATH": str(tools) + os.pathsep + os.environ["PATH"]}

    def run_release(self, action="patch", fail_stage=None):
        env = self.env.copy()
        if fail_stage:
            env["RELEASE_FIXTURE_FAIL"] = fail_stage
        return subprocess.run(["bash", str(self.script), action], cwd=self.root,
                              env=env, stdout=subprocess.PIPE, stderr=subprocess.STDOUT,
                              text=True, timeout=10)

    def assert_originals(self):
        for name, original in self.originals.items():
            self.assertEqual((self.root / name).read_bytes(), original, name)

    def assert_no_commands(self):
        self.assertFalse((self.root / "commands.jsonl").exists(), "invalid state reached build tools")

    def pending_state(self):
        spec = importlib.util.spec_from_file_location("release_fixture_state", SCRIPTS / "release_state.py")
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)
        state = module.ReleaseState(self.root)
        state.begin("1.2.3")
        state.write("VERSION", b"1.2.3\n", state.data["files"]["VERSION"]["mode"])
        return state

    def start_blocked_release(self):
        output = tempfile.TemporaryFile()
        self.addCleanup(output.close)
        env = {**self.env, "RELEASE_FIXTURE_BLOCK": "generate"}
        process = subprocess.Popen(["bash", str(self.script), "patch"], cwd=self.root,
                                   env=env, stdout=output, stderr=subprocess.STDOUT)

        def stop_owned():
            if process.poll() is None:
                process.terminate()
                process.wait(timeout=8)

        self.addCleanup(stop_owned)
        deadline = time.monotonic() + 5
        marker = self.root / "blocked.json"
        while not marker.exists() and process.poll() is None and time.monotonic() < deadline:
            time.sleep(0.01)
        self.assertTrue(marker.exists(), "owned fixture command did not reach the gate")
        child_pid = json.loads(marker.read_text())["pid"]
        return process, child_pid

    def add_release(self, version):
        target = self.root / "releases" / ("v" + version)
        target.mkdir(parents=True)
        checksums = []
        for suffix in ("linux-amd64", "windows-amd64.exe"):
            name = f"citadel-sentry-v{version}-{suffix}"
            data = name.encode()
            (target / name).write_bytes(data)
            checksums.append(f"{hashlib.sha256(data).hexdigest()}  {name}\n")
        (target / "SHA256SUMS.txt").write_text("".join(checksums))
        return target

    def test_missing_version_is_not_invented(self):
        (self.root / "VERSION").unlink()
        result = self.run_release()
        self.assertNotEqual(result.returncode, 0, result.stdout)
        self.assertFalse((self.root / "VERSION").exists())
        self.assert_no_commands()

    def test_incomplete_version_is_not_completed_with_defaults(self):
        (self.root / "VERSION").write_text("1.2\n")
        result = self.run_release()
        self.assertNotEqual(result.returncode, 0, result.stdout)
        self.assertEqual((self.root / "VERSION").read_text(), "1.2\n")
        self.assert_no_commands()

    def test_generate_failure_restores_exact_metadata_and_binaries(self):
        result = self.run_release(fail_stage="generate")
        self.assertNotEqual(result.returncode, 0, result.stdout)
        self.assert_originals()
        self.assertFalse((self.root / "releases/v1.2.3").exists())

    def test_windows_build_failure_restores_exact_metadata_and_binaries(self):
        result = self.run_release(fail_stage="windows/amd64")
        self.assertNotEqual(result.returncode, 0, result.stdout)
        self.assert_originals()
        self.assertFalse((self.root / "releases/v1.2.3").exists())

    def test_disagreeing_version_metadata_is_preserved(self):
        path = self.root / "package.json"
        original = b'{"version":"9.8.7"}\n'
        path.write_bytes(original)
        result = self.run_release()
        self.assertNotEqual(result.returncode, 0, result.stdout)
        self.assertEqual(path.read_bytes(), original)
        self.assert_no_commands()

    def test_missing_wails_version_is_not_added_or_ignored(self):
        path = self.root / "wails.json"
        original = b'{"outputfilename":"citadel-sentry"}\n'
        path.write_bytes(original)
        result = self.run_release()
        self.assertNotEqual(result.returncode, 0, result.stdout)
        self.assertEqual(path.read_bytes(), original)
        self.assert_no_commands()

    def test_existing_release_is_never_overwritten(self):
        target = self.add_release("1.2.3")
        before = {p.name: p.read_bytes() for p in target.iterdir()}
        result = self.run_release()
        self.assertNotEqual(result.returncode, 0, result.stdout)
        self.assertEqual({p.name: p.read_bytes() for p in target.iterdir()}, before)
        self.assert_originals()
        self.assert_no_commands()

    def test_retention_preserves_unrecognized_directories(self):
        for version in ("1.0.0", "1.0.1", "1.0.2"):
            self.add_release(version)
        notes = self.root / "releases/v0-user-notes"
        notes.mkdir()
        (notes / "keep.txt").write_bytes(b"user notes")
        result = self.run_release()
        self.assertEqual(result.returncode, 0, result.stdout)
        self.assertEqual((notes / "keep.txt").read_bytes(), b"user notes")

    def test_success_publishes_matching_version_and_checksums(self):
        result = self.run_release()
        self.assertEqual(result.returncode, 0, result.stdout)
        self.assertEqual((self.root / "VERSION").read_text(), "1.2.3\n")
        for name, field in (("package.json", "version"), ("wails.json", "info")):
            data = json.loads((self.root / name).read_text())
            value = data[field] if field == "version" else data[field]["productVersion"]
            self.assertEqual(value, "1.2.3")
        target = self.root / "releases/v1.2.3"
        for line in (target / "SHA256SUMS.txt").read_text().splitlines():
            expected, name = line.split()
            self.assertEqual(hashlib.sha256((target / name).read_bytes()).hexdigest(), expected)
        self.assertIn(b"1.2.3", (self.root / "citadel-sentry").read_bytes())
        for suffix, convenience in (("linux-amd64", "citadel-sentry"), ("windows-amd64.exe", "citadel-sentry.exe")):
            self.assertEqual((target / f"citadel-sentry-v1.2.3-{suffix}").read_bytes(),
                             (self.root / convenience).read_bytes())

    def test_interrupted_journal_recovers_before_reading_version_inputs(self):
        self.pending_state()
        result = self.run_release(fail_stage="go")
        self.assertNotEqual(result.returncode, 0, result.stdout)
        self.assert_originals()
        self.assertFalse((self.root / ".citadel-release-transaction").exists())

    def test_corrupt_original_backup_blocks_recovery_without_guessing(self):
        state = self.pending_state()
        (state.folder / "original-VERSION").write_bytes(b"corrupt")
        before = {name: (self.root / name).read_bytes() for name in self.originals}
        result = self.run_release()
        self.assertNotEqual(result.returncode, 0, result.stdout)
        self.assertEqual({name: (self.root / name).read_bytes() for name in self.originals}, before)
        self.assertTrue(state.folder.exists())
        self.assert_no_commands()

    def test_valid_json_state_corruption_cannot_discard_originals(self):
        state = self.pending_state()
        path = state.folder / "state.json"
        envelope = json.loads(path.read_text())
        envelope["payload"]["phase"] = "committed"
        path.write_text(json.dumps(envelope))
        result = self.run_release()
        self.assertNotEqual(result.returncode, 0, result.stdout)
        self.assertTrue((state.folder / "original-VERSION").exists())
        self.assertEqual((self.root / "VERSION").read_bytes(), b"1.2.3\n")
        self.assert_no_commands()

    def test_external_file_changes_block_recovery_without_overwrite(self):
        state = self.pending_state()
        path = self.root / "wails.json"
        external = b'{"user":"changed settings"}\n'
        path.write_bytes(external)
        result = self.run_release()
        self.assertNotEqual(result.returncode, 0, result.stdout)
        self.assertEqual(path.read_bytes(), external)
        self.assertTrue(state.folder.exists())
        self.assertEqual((self.root / "VERSION").read_bytes(), b"1.2.3\n")
        self.assert_no_commands()

    def test_native_signal_stops_owned_child_before_restoring_originals(self):
        process, child_pid = self.start_blocked_release()
        process.send_signal(signal.SIGTERM)
        self.assertNotEqual(process.wait(timeout=8), 0)
        self.assert_originals()
        with self.assertRaises(ProcessLookupError):
            os.kill(child_pid, 0)
        self.assertFalse((self.root / ".citadel-release-transaction").exists())

    def test_second_release_does_not_modify_an_active_build(self):
        process, _ = self.start_blocked_release()
        before = {name: (self.root / name).read_bytes() for name in self.originals}
        result = self.run_release()
        self.assertNotEqual(result.returncode, 0, result.stdout)
        self.assertEqual({name: (self.root / name).read_bytes() for name in self.originals}, before)
        (self.root / "unblock").touch()
        self.assertEqual(process.wait(timeout=8), 0)

    def test_successful_command_cannot_reuse_a_stale_binary(self):
        binary = self.root / "build/bin/citadel-sentry.exe"
        binary.parent.mkdir(parents=True)
        binary.write_bytes(b"stale-build")
        self.env["RELEASE_FIXTURE_SKIP_OUTPUT"] = "windows/amd64"
        result = self.run_release()
        self.assertNotEqual(result.returncode, 0, result.stdout)
        self.assert_originals()
        self.assertFalse((self.root / "releases/v1.2.3").exists())

    def test_competing_release_creator_is_preserved_and_metadata_rolls_back(self):
        self.env["RELEASE_FIXTURE_COMPETING_RELEASE"] = "1"
        result = self.run_release()
        self.assertNotEqual(result.returncode, 0, result.stdout)
        self.assert_originals()
        self.assertEqual((self.root / "releases/v1.2.3/keep.txt").read_bytes(), b"competing user data")
        self.assertFalse((self.root / ".citadel-release-transaction").exists())

    def test_validation_uses_the_explicit_frontend_directory(self):
        path = self.root / "wails.json"
        data = json.loads(path.read_text())
        data["frontend:dir"] = "custom-ui"
        (self.root / "frontend").rename(self.root / "custom-ui")
        path.write_text(json.dumps(data))
        result = self.run_release()
        self.assertEqual(result.returncode, 0, result.stdout)
        calls = [json.loads(line) for line in (self.root / "commands.jsonl").read_text().splitlines()]
        npm_calls = [args for name, args in calls if name == "npm"]
        self.assertTrue(npm_calls)
        self.assertTrue(all(args[:2] == ["--prefix", "custom-ui"] for args in npm_calls))

    def test_same_bytes_from_an_external_replacement_are_not_claimed_as_owned(self):
        state = self.pending_state()
        path = self.root / "VERSION"
        replacement = self.root / "external-version"
        replacement.write_bytes(path.read_bytes())
        replacement.replace(path)
        result = self.run_release()
        self.assertNotEqual(result.returncode, 0, result.stdout)
        self.assertEqual(path.read_bytes(), b"1.2.3\n")
        self.assertTrue(state.folder.exists())
        self.assert_no_commands()

    def test_invalid_versions_cannot_reach_build_tools(self):
        for value in ("", "01.2.2", "1.02.2", "1.2.4294967296", "1.2.2 4", "vv1.2.2"):
            with self.subTest(version=value):
                (self.root / "VERSION").write_text(value)
                result = self.run_release()
                self.assertNotEqual(result.returncode, 0, result.stdout)
                self.assertEqual((self.root / "VERSION").read_text(), value)
                self.assert_no_commands()

    def test_downgrades_and_repeated_versions_are_preserved(self):
        for value in ("1.2.2", "1.2.1", "01.2.3"):
            with self.subTest(target=value):
                result = self.run_release(value)
                self.assertNotEqual(result.returncode, 0, result.stdout)
                self.assert_originals()
                self.assert_no_commands()

    def test_retention_preserves_an_unverified_release(self):
        for version in ("1.0.0", "1.0.1", "1.0.2"):
            self.add_release(version)
        invalid = self.add_release("0.9.0")
        artifact = invalid / "citadel-sentry-v0.9.0-linux-amd64"
        artifact.write_bytes(b"changed after release")
        result = self.run_release()
        self.assertEqual(result.returncode, 0, result.stdout)
        self.assertEqual(artifact.read_bytes(), b"changed after release")

    def test_sigkill_keeps_the_child_lock_then_recovers_after_verified_exit(self):
        # Become a subreaper only for this fixture, so its orphaned owned child
        # can be waited on explicitly instead of relying on system init timing.
        libc = ctypes.CDLL(None, use_errno=True)
        libc.prctl.argtypes = [ctypes.c_int, ctypes.c_ulong, ctypes.c_ulong, ctypes.c_ulong, ctypes.c_ulong]
        previous = ctypes.c_int()
        if libc.prctl(37, ctypes.addressof(previous), 0, 0, 0) != 0:
            raise OSError(ctypes.get_errno(), "read fixture subreaper state")
        if libc.prctl(36, 1, 0, 0, 0) != 0:
            raise OSError(ctypes.get_errno(), "enable fixture subreaper")

        def restore_subreaper():
            if libc.prctl(36, previous.value, 0, 0, 0) != 0:
                raise OSError(ctypes.get_errno(), "restore fixture subreaper state")

        self.addCleanup(restore_subreaper)
        process, child_pid = self.start_blocked_release()

        def reap_owned_child():
            try:
                waited, _ = os.waitpid(child_pid, os.WNOHANG)
            except ChildProcessError:
                return  # No direct owned child remains to signal.
            if waited == 0:
                os.kill(child_pid, signal.SIGTERM)
                os.waitpid(child_pid, 0)

        self.addCleanup(reap_owned_child)
        process.kill()
        self.assertEqual(process.wait(timeout=5), -signal.SIGKILL)
        before = {name: (self.root / name).read_bytes() for name in self.originals}
        result = self.run_release()
        self.assertNotEqual(result.returncode, 0, result.stdout)
        self.assertEqual({name: (self.root / name).read_bytes() for name in self.originals}, before)
        (self.root / "unblock").touch()
        deadline = time.monotonic() + 5
        while time.monotonic() < deadline:
            waited, _ = os.waitpid(child_pid, os.WNOHANG)
            if waited == child_pid:
                break
            time.sleep(0.01)
        else:
            self.fail("owned crash fixture child did not exit")
        result = self.run_release(fail_stage="go")
        self.assertNotEqual(result.returncode, 0, result.stdout)
        self.assert_originals()
        self.assertFalse((self.root / ".citadel-release-transaction").exists())

    def test_cleanup_restart_does_not_require_already_deleted_backups(self):
        state = self.pending_state()
        (self.root / "VERSION").write_bytes(self.originals["VERSION"])
        state.data["phase"] = "restored"
        state.save()
        (state.folder / "original-VERSION").unlink()
        result = self.run_release(fail_stage="go")
        self.assertNotEqual(result.returncode, 0, result.stdout)
        self.assert_originals()
        self.assertFalse(state.folder.exists())

    def test_empty_cleanup_directory_is_recoverable_without_defaults(self):
        (self.root / ".citadel-release-transaction").mkdir()
        result = self.run_release(fail_stage="go")
        self.assertNotEqual(result.returncode, 0, result.stdout)
        self.assert_originals()
        self.assertFalse((self.root / ".citadel-release-transaction").exists())

    def test_nonstandard_json_numbers_cannot_reach_build_tools(self):
        path = self.root / "package.json"
        for value in ("NaN", "Infinity", "1e400"):
            with self.subTest(number=value):
                original = ('{"version":"1.2.2", "extra":' + value + '}\n').encode()
                path.write_bytes(original)
                result = self.run_release()
                self.assertNotEqual(result.returncode, 0, result.stdout)
                self.assertEqual(path.read_bytes(), original)
                self.assert_no_commands()

    def test_inputs_changed_before_snapshot_are_not_overwritten_by_old_choices(self):
        helper = self.root / "scripts/release_state.py"
        source = helper.read_text()
        hook = '''        (self.root / "VERSION").write_bytes(b"1.2.9\\n")
        for filename, key in (("package.json", "version"), ("wails.json", "info")):
            value = json.loads((self.root / filename).read_text())
            if key == "version": value[key] = "1.2.9"
            else: value[key]["productVersion"] = "1.2.9"
            (self.root / filename).write_text(json.dumps(value))
'''
        anchor = '        self.folder.mkdir(mode=0o700)\n'
        self.assertIn(anchor, source)
        helper.write_text(source.replace(anchor, hook + anchor, 1))
        result = self.run_release()
        self.assertNotEqual(result.returncode, 0, result.stdout)
        self.assertEqual((self.root / "VERSION").read_bytes(), b"1.2.9\n")
        self.assertEqual(json.loads((self.root / "package.json").read_text())["version"], "1.2.9")
        self.assert_no_commands()


if __name__ == "__main__":
    unittest.main()
