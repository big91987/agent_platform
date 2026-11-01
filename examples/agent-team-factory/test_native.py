"""Real process lifecycle regression; no model calls or product acceptance."""

import json
import os
import signal
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

from factory_store import Store


class NativeLifecycleTests(unittest.TestCase):
    def test_large_stdin_is_subject_to_timeout_and_status_is_preserved(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            fixture = root / "unread-input"
            fixture.write_text("#!/usr/bin/env python3\nimport time\ntime.sleep(20)\n")
            fixture.chmod(0o700)
            store = Store(root / "runtime")
            store.initialize(
                {"concurrency": 1, "codex": str(fixture), "timeout_seconds": 0.7}
            )
            code = "import sys; from native import Native; from factory_store import Store; Native(Store(sys.argv[1])).run('fixture',sys.argv[2],'x'*1000000)"
            controller = subprocess.Popen(
                [sys.executable, "-c", code, str(store.root), str(root)],
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
                env={**os.environ, "PYTHONPATH": str(Path(__file__).parent)},
            )
            timed_out = False
            try:
                output, error = controller.communicate(timeout=2)
            except subprocess.TimeoutExpired:
                timed_out = True
                controller.terminate()
                output, error = controller.communicate(timeout=2)
            finally:
                for event in store.snapshot()["events"]:
                    if event["kind"] == "native_started":
                        try:
                            os.killpg(event["detail"]["pid"], signal.SIGKILL)
                        except ProcessLookupError:
                            pass
            self.assertFalse(timed_out, "large stdin bypassed the native timeout")
            self.assertEqual(controller.returncode, 0, error.decode(errors="replace"))
            status = json.loads((store.root / "native/fixture/status.json").read_text())
            self.assertEqual(status["reason"], "timeout")

    def test_timeout_stops_descendant_even_after_native_parent_has_exited(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            fixture = root / "exited-parent"
            fixture.write_text(
                "#!/usr/bin/env python3\nimport subprocess,sys\nsys.stdin.read()\nsubprocess.Popen([sys.executable,'-c','import time; time.sleep(20)'])\n"
            )
            fixture.chmod(0o700)
            store = Store(root / "runtime")
            store.initialize(
                {"concurrency": 1, "codex": str(fixture), "timeout_seconds": 0.2}
            )
            code = "import sys; from native import Native; from factory_store import Store; Native(Store(sys.argv[1])).run('fixture',sys.argv[2],'isolated lifecycle fixture')"
            env = {**os.environ, "PYTHONPATH": str(Path(__file__).parent)}
            controller = subprocess.Popen(
                [sys.executable, "-c", code, str(store.root), str(root)],
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
                env=env,
            )
            timed_out = False
            try:
                output, error = controller.communicate(timeout=2)
            except subprocess.TimeoutExpired:
                timed_out = True
                controller.terminate()
                output, error = controller.communicate(timeout=2)
            finally:
                for event in store.snapshot()["events"]:
                    if event["kind"] == "native_started":
                        try:
                            os.killpg(event["detail"]["pid"], signal.SIGKILL)
                        except ProcessLookupError:
                            pass
            self.assertFalse(
                timed_out,
                "native parent exited, but child stdout kept controller stuck past timeout",
            )
            self.assertEqual(controller.returncode, 0, error.decode(errors="replace"))
            status = json.loads((store.root / "native/fixture/status.json").read_text())
            self.assertEqual(status["reason"], "timeout")


if __name__ == "__main__":
    unittest.main()
