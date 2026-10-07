"""Dependency-free client; the platform owns authorization and Session state."""

import json
import time
import urllib.error
import urllib.parse
import urllib.request
from http.client import HTTPException

__all__ = ["APIError", "Client"]


class APIError(RuntimeError):
    """An HTTP rejection from the platform, including its status code."""

    def __init__(self, status_code, message):
        self.status_code = status_code
        super().__init__(f"Platform HTTP {status_code}: {message}")


class _NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        # Never forward the user's bearer credential to a redirected address.
        return None


class Client:
    """Use a user Token; no browser login or native Session ID is required.

    Requests are never automatically replayed. Reuse the original request_id
    when retrying an uncertain submission. Responses preserve the API format.
    """

    def __init__(self, base_url, token, *, timeout=30):
        url = urllib.parse.urlsplit(base_url)
        if (
            url.scheme not in ("http", "https")
            or not url.netloc
            or url.username
            or url.password
            or url.query
            or url.fragment
        ):
            raise ValueError(
                "base_url must be an HTTP(S) service URL without credentials or query"
            )
        if not token or "\n" in token or "\r" in token:
            raise ValueError("A user API Token is required")
        if timeout <= 0:
            raise ValueError("timeout must be positive")
        self.base_url = base_url.rstrip("/")
        self._token = token
        self.timeout = timeout
        self._opener = urllib.request.build_opener(_NoRedirect())

    def _open(self, path, body=None, headers=None, method=None):
        request = urllib.request.Request(
            self.base_url + path,
            method=method,
            data=None
            if body is None
            else json.dumps(body, ensure_ascii=False).encode(),
            headers={
                "Authorization": "Bearer " + self._token,
                "Content-Type": "application/json",
                **(headers or {}),
            },
        )
        try:
            return self._opener.open(request, timeout=self.timeout)
        except urllib.error.HTTPError as error:
            with error:
                try:
                    message = json.load(error).get("error", error.reason)
                except (ValueError, AttributeError, OSError, HTTPException):
                    message = error.reason
            raise APIError(
                error.code, str(message).replace(self._token, "[redacted]")
            ) from None
        except (OSError, urllib.error.URLError, HTTPException) as error:
            reason = str(error).replace(self._token, "[redacted]")
            raise ConnectionError(f"Platform transport failed: {reason}") from None

    def _json(self, path, body=None, *, method=None):
        with self._open(path, body, method=method) as response:
            try:
                return json.load(response)
            except (ValueError, UnicodeError):
                raise ValueError("Platform returned invalid JSON") from None
            except (OSError, HTTPException) as error:
                reason = str(error).replace(self._token, "[redacted]")
                raise ConnectionError(
                    f"Platform response interrupted: {reason}"
                ) from None

    def tool_servers(self):
        """List registered external MCP servers (administrator only)."""
        return self._json("/api/tool-servers")

    def register_tool_server(self, server):
        """Save an external MCP connection, without uploading tool code."""
        return self._json("/api/tool-servers", server)

    def discover_tool_server(self, server_id):
        """Discover names and schemas from the external MCP service."""
        return self._json(
            "/api/tool-servers/" + urllib.parse.quote(server_id, safe="") + "/discover",
            {},
        )

    @staticmethod
    def _conversation_path(conversation_id):
        if not conversation_id:
            raise ValueError("conversation_id is required")
        return "/api/conversations/" + urllib.parse.quote(conversation_id, safe="")

    def me(self):
        """Check credentials and retrieve their platform user identity."""
        return self._json("/api/me")

    def agents(self):
        return self._json("/api/agents")

    def invoke(
        self,
        message,
        *,
        request_id,
        agent_id="",
        conversation_id="",
        workspace_path="",
        user_id="",
        network_access=None,
    ):
        """Create or continue a conversation; return its durable receipt.

        request_id must identify the calling system's input event, not an HTTP
        attempt. workspace_path refers to the directory seen by the platform.
        """
        if not request_id:
            raise ValueError(
                "Use a stable request_id so a lost response can be retried safely"
            )
        if not agent_id and not conversation_id:
            raise ValueError("agent_id is required for a new conversation")
        body = {"message": message, "request_id": request_id}
        body.update(
            {
                key: value
                for key, value in {
                    "agent_id": agent_id,
                    "conversation_id": conversation_id,
                    "workspace_path": workspace_path,
                    "user_id": user_id,
                }.items()
                if value
            }
        )
        if network_access is not None:
            body["network_access"] = network_access
        return self._json("/api/invoke", body)

    def network_access(self, conversation_id, enabled):
        """Set idle conversation networking within the Agent's granted permissions."""
        return self._json(
            self._conversation_path(conversation_id) + "/network-access",
            {"enabled": enabled},
        )

    def conversations(self):
        return self._json("/api/conversations")

    def conversation(self, conversation_id):
        return self._json(self._conversation_path(conversation_id))

    def decide_tool(self, conversation_id, approval_id, decision):
        """Approve or decline one pending native tool call in this conversation."""
        if decision not in ("accept", "decline"):
            raise ValueError("decision must be accept or decline")
        path = (
            self._conversation_path(conversation_id)
            + "/approvals/"
            + urllib.parse.quote(approval_id, safe="")
        )
        return self._json(path, {"decision": decision})

    def events(self, conversation_id, *, after=0):
        query = urllib.parse.urlencode({"after": after, "format": "json"})
        return self._json(self._conversation_path(conversation_id) + "/events?" + query)

    def stream(self, conversation_id, *, after=0):
        """Yield complete stored event envelopes; ignore SSE heartbeats.

        Save event['id'] and supply it as after when reconnecting. This iterator
        does not submit inputs or restart execution. Close it when no longer used.
        """
        query = urllib.parse.urlencode({"after": after})
        path = self._conversation_path(conversation_id) + "/events?" + query
        with self._open(
            path, headers={"Accept": "text/event-stream", "Last-Event-ID": str(after)}
        ) as response:
            if response.headers.get_content_type() != "text/event-stream":
                raise ValueError("Platform returned a non-SSE response")
            data = []
            try:
                for raw in response:
                    line = raw.decode("utf-8").rstrip("\r\n")
                    if line.startswith("data:"):
                        data.append(line[5:].removeprefix(" "))
                    elif not line and data:
                        yield json.loads("\n".join(data))
                        data = []
            except (OSError, urllib.error.URLError, HTTPException) as error:
                reason = str(error).replace(self._token, "[redacted]")
                raise ConnectionError(
                    f"Platform event stream disconnected: {reason}"
                ) from None

    def wait(self, conversation_id, *, message_id=None, timeout=600, poll_interval=1):
        """Wait for this input or for the conversation to stop executing.

        idle means a native round ended, not that the business task is complete.
        A timeout ends observation only; Agent execution and Session are retained.
        Failed/stopped conversations are returned without automatically resuming.
        """
        if timeout < 0 or poll_interval <= 0:
            raise ValueError(
                "timeout must be non-negative; poll_interval must be positive"
            )
        deadline = time.monotonic() + timeout
        while True:
            result = self.conversation(conversation_id)
            if message_id is not None:
                message = next(
                    (m for m in result["messages"] if m["id"] == message_id), None
                )
                if message is None:
                    raise ValueError("message_id is not in this conversation")
                if message["status"] in ("completed", "failed", "stopped"):
                    return result
            if result["conversation"]["status"] not in (
                "queued",
                "running",
                "stopping",
            ):
                return result
            remaining = deadline - time.monotonic()
            if remaining <= 0:
                raise TimeoutError(
                    f"Still executing; query or reconnect to conversation {conversation_id}"
                )
            time.sleep(min(poll_interval, remaining))

    def workspace_access(self, conversation_id, *, read_only):
        """Change workspace access between turns; running processes are unchanged."""
        return self._json(
            self._conversation_path(conversation_id) + "/workspace-access",
            {"read_only": read_only},
            method="PATCH",
        )

    def stop(self, conversation_id, *, expected_message_id=None, discard_queued=False):
        body = {}
        if expected_message_id is not None:
            body["expected_message_id"] = expected_message_id
        if discard_queued:
            body["discard_queued"] = True
        return self._json(self._conversation_path(conversation_id) + "/stop", body)

    def steer(self, conversation_id, message_id):
        return self._json(
            self._conversation_path(conversation_id) + "/steer",
            {"message_id": message_id},
        )

    def continue_queue(self, conversation_id):
        return self._json(self._conversation_path(conversation_id) + "/continue", {})

    def close(self, conversation_id):
        return self._json(self._conversation_path(conversation_id) + "/close", {})

    def artifacts(self, conversation_id):
        return self._json(self._conversation_path(conversation_id) + "/artifacts")

    def read_file(self, conversation_id, path):
        """Return file bytes; the platform checks ownership and file boundaries."""
        query = urllib.parse.urlencode({"path": path, "download": 1})
        with self._open(
            self._conversation_path(conversation_id) + "/file?" + query
        ) as response:
            try:
                return response.read()
            except (OSError, HTTPException) as error:
                reason = str(error).replace(self._token, "[redacted]")
                raise ConnectionError(
                    f"Platform file download interrupted: {reason}"
                ) from None

    @staticmethod
    def _workflow_path(run_id):
        if not run_id:
            raise ValueError("run_id is required")
        return "/api/workflow-runs/" + urllib.parse.quote(run_id, safe="")

    def start_workflow(
        self,
        workflow_id,
        message,
        *,
        workspace_path,
        request_id,
        start_node="",
        parameters=None,
    ):
        """Start once per caller event key. No CI stage scheduling is needed."""
        if not request_id:
            raise ValueError("A stable request_id is required")
        body = {
            "workflow_id": workflow_id,
            "input": message,
            "workspace_path": workspace_path,
            "request_id": request_id,
        }
        if start_node:
            body["start_node"] = start_node
        if parameters:
            body["parameters"] = parameters
        return self._json("/api/workflow-runs", body)

    def workflow_run(self, run_id):
        return self._json(self._workflow_path(run_id))

    def workflow_command_output(self, run_id, seq, *, offset=0):
        """Read one retained log page; next_offset/eof bound subsequent reads.

        truncated reports archive overflow, not a successful command. Legacy
        receipts without log metadata have no recoverable archive (404).
        """
        if type(seq) is not int or seq < 1 or type(offset) is not int or offset < 0:
            raise ValueError("seq must be positive and offset nonnegative integers")
        return self._json(
            self._workflow_path(run_id) + f"/steps/{seq}/output?offset={offset}"
        )

    def workflow_runs(self, *, workflow_id="", before=""):
        """One page, at most 200 runs. Pass the last ID as before for the next."""
        return self._json(
            "/api/workflow-runs?"
            + urllib.parse.urlencode({"workflow_id": workflow_id, "before": before})
        )

    def workflow_by_request(self, request_id):
        """Resolve the caller's original task even after its input was edited."""
        if not request_id:
            raise ValueError("request_id is required")
        return self._json(
            "/api/workflow-runs/by-request?"
            + urllib.parse.urlencode({"request_id": request_id})
        )

    def workflow_message(self, run_id, message, *, request_id, seq=None):
        """Persist feedback to the current Agent atomically; 409 needs inspection.

        A repeated event returns its original receipt even after handoff. seq,
        when supplied, refuses to deliver a message to a different execution.
        """
        if not request_id:
            raise ValueError("A stable request_id is required")
        body = {"message": message, "request_id": request_id}
        if seq is not None:
            body["seq"] = seq
        return self._json(self._workflow_path(run_id) + "/messages", body)

    def workflow_command(self, run_id, action, *, seq, **values):
        """Explicit run control; callers must inspect state after uncertain errors."""
        if action not in ("stop", "cancel", "resume", "return", "decision"):
            raise ValueError("unsupported workflow command")
        if set(values) - {"message", "summary", "target", "route"}:
            raise ValueError("unsupported workflow command fields")
        return self._json(
            self._workflow_path(run_id) + "/" + action, {"seq": seq, **values}
        )

    def wait_workflow(self, run_id, *, timeout=600, poll_interval=2):
        """Observe until waiting/failed/stopped/cancelled/completed, never resume implicitly."""
        if timeout < 0 or poll_interval <= 0:
            raise ValueError("invalid wait bounds")
        deadline = time.monotonic() + timeout
        while True:
            result = self.workflow_run(run_id)
            if result["status"] not in ("running", "stopping"):
                return result
            remaining = deadline - time.monotonic()
            if remaining <= 0:
                raise TimeoutError("Still executing workflow " + run_id)
            time.sleep(min(poll_interval, remaining))
