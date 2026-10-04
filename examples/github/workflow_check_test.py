import tempfile
import unittest
from pathlib import Path

import workflow_check


class WorkflowSyntaxTest(unittest.TestCase):
    def test_expression_is_not_shell_syntax_but_unclosed_if_is_rejected(self):
        with tempfile.TemporaryDirectory() as temp:
            path = Path(temp) / "workflow.yml"
            path.write_text(
                'on: push\njobs:\n  test:\n    runs-on: ubuntu-latest\n    steps:\n      - run: echo "${{ github.sha }}"\n'
            )
            workflow_check.check(path)
            path.write_text(
                "on: push\njobs:\n  test:\n    runs-on: ubuntu-latest\n    steps:\n      - run: if true; then\n"
            )
            with self.assertRaises(ValueError):
                workflow_check.check(path)
