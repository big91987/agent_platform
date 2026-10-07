import concurrent.futures
import importlib.util
import json
import socket
import tempfile
import time
import unittest
import urllib.error
import urllib.request
from pathlib import Path

SOURCE = Path(__file__).with_name("port_conflict.py")
SPEC = importlib.util.spec_from_file_location("port_conflict", SOURCE)
probe = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(probe)
OLD = "a" * 40
NEW = "b" * 40


class PortConflictTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        with socket.socket() as listener:
            listener.bind(("127.0.0.1", 0))
            self.port = listener.getsockname()[1]

    def journal(self, stage, attempt="new", sha=NEW):
        value = {
            "stage": stage,
            "attempt": attempt,
            "plan": {"sha": sha},
            "previous": {"sha": OLD},
        }
        temporary = self.root / "next.json"
        temporary.write_text(json.dumps(value))
        temporary.replace(self.root / "activation.json")

    def exercise(self, **kwargs):
        return probe.exercise(
            self.root,
            self.port,
            OLD,
            NEW,
            "old",
            arm_timeout=0.6,
            hold_seconds=0.15,
            upgrade_timeout=0.3,
            first_health_timeout=0.4,
            poll_seconds=0.01,
            **kwargs,
        )

    def request(self):
        opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
        deadline = time.monotonic() + 0.5
        while time.monotonic() < deadline:
            try:
                opener.open(f"http://127.0.0.1:{self.port}/healthz", timeout=0.1)
            except urllib.error.HTTPError as error:
                with error:
                    self.assertEqual(error.code, 503)
                    self.assertEqual(
                        json.load(error)["fixture"], "isolated-port-conflict"
                    )
                return
            except (urllib.error.URLError, TimeoutError):
                time.sleep(0.01)
        self.fail("fault listener did not bind")

    def closed(self):
        with socket.socket() as listener:
            self.assertNotEqual(listener.connect_ex(("127.0.0.1", self.port)), 0)

    def test_exact_new_attempt_fault_is_bounded_and_does_not_write_journal(self):
        self.journal("backup_complete", attempt="old")
        with concurrent.futures.ThreadPoolExecutor() as executor:
            future = executor.submit(self.exercise)
            time.sleep(0.03)
            self.closed()
            self.journal("backup_complete")
            self.request()
            self.journal("upgrade_confirmed")
            time.sleep(0.03)  # Allow the read-only observer to see the stage.
            self.request()
            before = (self.root / "activation.json").read_bytes()
            result = future.result(timeout=2)
        self.assertEqual(result["attempt"], "new")
        self.assertTrue(result["upgrade_observed"])
        self.assertGreater(result["health_requests"], 0)
        self.assertEqual(result["recovery"], "not_checked")
        self.assertEqual((self.root / "activation.json").read_bytes(), before)
        self.closed()

    def test_delayed_first_health_get_receives_full_hold_window(self):
        self.journal("backup_complete")
        with concurrent.futures.ThreadPoolExecutor() as executor:
            future = executor.submit(self.exercise)
            self.request()
            self.journal("upgrade_confirmed")
            time.sleep(0.22)  # launchctl may delay the first health request.
            self.request()
            started = time.monotonic()
            result = future.result(timeout=2)
        self.assertGreaterEqual(time.monotonic() - started, 0.12)
        self.assertGreater(result["health_requests"], 0)
        self.closed()

    def test_partial_headers_do_not_extend_release_deadline(self):
        self.journal("backup_complete")
        with concurrent.futures.ThreadPoolExecutor() as executor:
            future = executor.submit(self.exercise)
            self.request()
            self.journal("upgrade_confirmed")
            time.sleep(0.03)  # Allow the read-only observer to see the stage.
            self.request()
            started = time.monotonic()
            with socket.create_connection(("127.0.0.1", self.port)) as connection:
                connection.sendall(b"GET /healthz HTTP/1.1\r\nX-Incomplete: ")
                future.result(timeout=0.5)
            self.assertLess(time.monotonic() - started, 0.5)
        self.closed()

    def test_preupgrade_health_and_port_probe_do_not_start_hold(self):
        self.journal("backup_complete")
        with concurrent.futures.ThreadPoolExecutor() as executor:
            future = executor.submit(self.exercise)
            self.request()
            self.journal("upgrade_confirmed")
            time.sleep(0.03)
            with socket.create_connection(("127.0.0.1", self.port)) as connection:
                connection.sendall(b"GET /other HTTP/1.1\r\n\r\n")
                connection.recv(4096)
            with self.assertRaisesRegex(RuntimeError, "no health request"):
                future.result(timeout=2)
        self.closed()

    def test_idle_client_cannot_keep_listener_beyond_hold(self):
        self.journal("backup_complete")
        with concurrent.futures.ThreadPoolExecutor() as executor:
            future = executor.submit(self.exercise)
            self.request()
            self.journal("upgrade_confirmed")
            time.sleep(0.03)
            self.request()
            started = time.monotonic()
            with socket.create_connection(("127.0.0.1", self.port)):
                future.result(timeout=0.5)
            self.assertLess(time.monotonic() - started, 0.5)
        self.closed()

    def test_wrong_candidate_never_occupies_port(self):
        self.journal("backup_complete", sha="c" * 40)
        with self.assertRaisesRegex(RuntimeError, "not observed"):
            self.exercise()
        self.closed()

    def test_unconfirmed_upgrade_releases_port_and_fails(self):
        self.journal("backup_complete")
        with concurrent.futures.ThreadPoolExecutor() as executor:
            future = executor.submit(self.exercise)
            self.request()
            with self.assertRaisesRegex(RuntimeError, "upgrade not confirmed"):
                future.result(timeout=2)
        self.closed()

    def test_already_started_candidate_is_not_interrupted(self):
        self.journal("upgrade_confirmed")
        with socket.socket() as existing:
            existing.bind(("127.0.0.1", self.port))
            existing.listen()
            with self.assertRaisesRegex(RuntimeError, "not observed"):
                self.exercise()
            self.assertEqual(existing.getsockname()[1], self.port)


if __name__ == "__main__":
    unittest.main()
