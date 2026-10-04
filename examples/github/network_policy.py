"""Repository networking defaults and explicit, owner-selected Issue overrides."""

import re

CHOICES = {"沿用仓库默认": None, "允许": True, "禁止": False}


def issue_network(body, default=True):
    """Read a single form field, never grant from quoted examples or prose."""
    sections = 0
    selected = []
    active = False
    fence = None
    comment = False
    for line in body.splitlines():
        if comment:
            if "-->" in line:
                comment = False
            continue
        if "<!--" in line:
            comment = "-->" not in line.split("<!--", 1)[1]
            continue
        if fence:
            if re.fullmatch(
                r" {0,3}" + re.escape(fence[0]) + r"{" + str(len(fence)) + r",}\s*",
                line,
            ):
                fence = None
            continue
        opening = re.match(r" {0,3}(`{3,}|~{3,})", line)
        if opening:
            fence = opening.group(1)
            continue
        if re.match(r" {0,3}#{1,6}(?:\s|$)", line):
            active = line.rstrip() == "### 联网权限"
            sections += int(active)
            continue
        if active and line.strip():
            selected.append(line.rstrip())
    if not sections:
        return default
    if sections != 1 or len(selected) != 1 or selected[0] not in CHOICES:
        raise ValueError("联网权限格式不明确，请选择沿用仓库默认、允许或禁止")
    value = CHOICES[selected[0]]
    return default if value is None else value


def network_command(event, repository):
    """Only the repository owner can change an existing task's grant selection."""
    owner = repository.split("/")[0]
    if (
        event.get("repository", {}).get("full_name") != repository
        or event.get("sender", {}).get("login") != owner
        or event.get("comment", {}).get("user", {}).get("login") != owner
        or event.get("action") != "created"
        or "pull_request" in event.get("issue", {})
    ):
        raise PermissionError(
            "Only an owner-created Issue comment may select networking"
        )
    body = event["comment"]["body"].strip()
    if body not in ("/network allow", "/network deny", "/network default"):
        raise ValueError("Use /network allow, /network deny or /network default")
    return body.split()[1]
