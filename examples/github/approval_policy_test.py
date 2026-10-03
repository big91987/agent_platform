"""Approval mode boundaries and explicit Issue-form opt-in parsing."""

import unittest

try:
    import approval_policy as policy
except ImportError:
    policy = None


class PolicyTest(unittest.TestCase):
    def setUp(self):
        self.assertIsNotNone(policy, "approval policy has not been implemented")

    def test_only_explicit_unique_form_checkbox_enables_autonomy(self):
        for body, expected in (
            ("", False),
            ("### 自主推进\n\n- [ ] 按推荐方案自主推进", False),
            ("### 自主推进\n\n- [x] 按推荐方案自主推进", True),
            ("### 自主推进\r\n\r\n- [X] 按推荐方案自主推进\r\n", True),
            (
                "### 目标\n文本\n### 自主推进\n- [x] 按推荐方案自主推进\n### 验收\n文本",
                True,
            ),
            ("### 说明\n- [x] 按推荐方案自主推进", False),
            (
                "### 自主推进\n- [ ] 按推荐方案自主推进\n### 说明\n- [x] 按推荐方案自主推进",
                False,
            ),
            ("### 自主推进\n> - [x] 按推荐方案自主推进", False),
            ("> ### 自主推进\n> - [x] 按推荐方案自主推进", False),
            ("### 自主推进\n    - [x] 按推荐方案自主推进", False),
            ("```markdown\n### 自主推进\n- [x] 按推荐方案自主推进\n```", False),
            ("~~~\n### 自主推进\n- [x] 按推荐方案自主推进\n~~~", False),
            ("### 自主推进\n```\n- [x] 按推荐方案自主推进\n```", False),
            ("<!--\n### 自主推进\n- [x] 按推荐方案自主推进\n-->", False),
            ("### 自主推进\n- [x] 按推荐方案自主推进（示例）", False),
            ("### 自主推进\n- [x] 按推荐方案自主推进\n- [ ] 按推荐方案自主推进", False),
            (
                "### 自主推进\n- [x] 按推荐方案自主推进\n### 自主推进\n- [x] 按推荐方案自主推进",
                False,
            ),
            ("### 自主推进\n### 示例\n- [x] 按推荐方案自主推进", False),
            ("### 自主推进\n#### 示例\n- [x] 按推荐方案自主推进", False),
        ):
            with self.subTest(body=body):
                self.assertEqual(policy.read_autonomous(body), expected)

    def test_strict_stages_require_confirmation(self):
        for stage, target in (
            ("requirements", "design"),
            ("design", "development"),
            ("development", "qa"),
            ("qa", "report"),
        ):
            with self.subTest(stage=stage):
                resolved = policy._resolve_policy(stage)
                self.assertEqual(resolved.target_stage, target)
                self.assertFalse(resolved.automatic_handoff)
                self.assertFalse(resolved.automatic_clarification)
                self.assertFalse(resolved.automatic_rework)
                self.assertFalse(resolved.integration_only)

    def test_autonomy_applies_to_every_stage(self):
        for stage in ("requirements", "design", "development", "qa"):
            with self.subTest(stage=stage):
                resolved = policy._resolve_policy(stage, autonomous=True)
                self.assertTrue(resolved.automatic_handoff)
                self.assertTrue(resolved.automatic_clarification)
                self.assertTrue(resolved.automatic_rework)
                self.assertFalse(resolved.integration_only)

    def test_ready_pr_integration_never_authorizes_upstream_product_choices(self):
        for autonomous in (False, True):
            for stage in ("requirements", "design", "development", "qa"):
                with self.subTest(stage=stage, autonomous=autonomous):
                    resolved = policy._resolve_policy(
                        stage, autonomous, integrating=True
                    )
                    downstream = stage in ("development", "qa")
                    self.assertEqual(resolved.automatic_handoff, downstream)
                    self.assertEqual(resolved.automatic_clarification, downstream)
                    self.assertEqual(resolved.automatic_rework, downstream)
                    self.assertTrue(resolved.integration_only)

    def test_unsupported_stage_does_not_acquire_permissions(self):
        for stage in ("report", "review", "merge", "unknown", ""):
            with self.subTest(stage=stage), self.assertRaises(ValueError):
                policy.approval_policy(stage, autonomous=True)


if __name__ == "__main__":
    unittest.main()
