import asyncio
import json
import unittest
from unittest.mock import patch

import httpx

from app.main import app
from app.parallel import (
    MAX_RESPONSE_BYTES, MCP_URL, PROTOCOL_VERSION, USER_AGENT, parallel_client,
)


class ParallelTests(unittest.IsolatedAsyncioTestCase):
    async def asyncSetUp(self):
        self.requests = []
        self.clients = []
        self.tool_result = {
            "content": [{"type": "text", "text": "Python release\nhttps://python.org/\nRelease notes"}],
            "isError": False,
        }
        self.tool_status = 200
        self.tool_body = None
        self.tool_exception = None
        self.sse = False

        async def handler(request):
            self.requests.append(request)
            self.assertEqual(str(request.url), MCP_URL)
            self.assertEqual(request.headers["user-agent"], USER_AGENT)
            self.assertNotIn("authorization", request.headers)
            self.assertIn("application/json", request.headers["accept"])
            self.assertIn("text/event-stream", request.headers["accept"])
            payload = json.loads(request.content)
            method = payload["method"]
            if method == "initialize":
                self.assertNotIn("mcp-session-id", request.headers)
                self.assertEqual(payload["params"]["protocolVersion"], PROTOCOL_VERSION)
                return httpx.Response(200, headers={"Mcp-Session-Id": "transport-session"}, json={
                    "jsonrpc": "2.0", "id": 1,
                    "result": {"protocolVersion": PROTOCOL_VERSION},
                })
            self.assertEqual(request.headers["mcp-session-id"], "transport-session")
            self.assertEqual(request.headers["mcp-protocol-version"], PROTOCOL_VERSION)
            if method == "notifications/initialized":
                self.assertNotIn("id", payload)
                return httpx.Response(202)
            self.assertEqual(method, "tools/call")
            self.assertEqual(payload["params"]["name"], "web_search")
            # Neither a process-wide nor a connection-wide conversation ID exists.
            self.assertNotIn("session_id", payload["params"]["arguments"])
            if self.tool_exception:
                raise self.tool_exception
            if self.tool_body is not None:
                return httpx.Response(self.tool_status, content=self.tool_body)
            packet = {"jsonrpc": "2.0", "id": 2, "result": self.tool_result}
            if self.sse:
                body = ': keepalive\n\ndata: {"jsonrpc":"2.0","method":"notifications/progress"}\n\n'
                body += "event: message\ndata: " + json.dumps(packet) + "\n\n"
                return httpx.Response(200, headers={"Content-Type": "text/event-stream; charset=utf-8"}, text=body)
            return httpx.Response(200, json=packet)

        self.handler = handler

        async def client_override():
            async with httpx.AsyncClient(transport=httpx.MockTransport(self.handler)) as client:
                self.clients.append(client)
                yield client

        app.dependency_overrides[parallel_client] = client_override
        self.http = httpx.AsyncClient(transport=httpx.ASGITransport(app=app), base_url="http://plugin")

    async def asyncTearDown(self):
        await self.http.aclose()
        app.dependency_overrides.clear()

    async def call(self, query="Python release notes"):
        return await self.http.post("/parallel-search", json={"command": query})

    async def test_anonymous_route_returns_excerpts_and_urls(self):
        with patch.dict("os.environ", {"PARALLEL_API_KEY": "must-not-be-used", "OPENAI_API_KEY": "unused"}):
            response = await self.call("  Python release notes  ")
        self.assertEqual(response.status_code, 200)
        self.assertEqual(response.json(), {"msg": self.tool_result["content"][0]["text"], "wrapper": True})
        args = json.loads(self.requests[-1].content)["params"]["arguments"]
        self.assertEqual(args, {"objective": "Python release notes", "search_queries": ["Python release notes"]})
        self.assertTrue(all(client.is_closed for client in self.clients))

    async def test_sse_response_preserves_source_urls(self):
        self.sse = True
        response = await self.call()
        self.assertIn("https://python.org/", response.json()["msg"])

    async def test_interleaved_calls_keep_transport_sessions_separate(self):
        sessions = {}

        async def interleaved(request):
            self.assertEqual(str(request.url), MCP_URL)
            self.assertEqual(request.headers["user-agent"], USER_AGENT)
            self.assertNotIn("authorization", request.headers)
            payload = json.loads(request.content)
            task = asyncio.current_task()
            if payload["method"] == "initialize":
                self.assertNotIn("mcp-session-id", request.headers)
                session = str(len(sessions))
                sessions[task] = session
                await asyncio.sleep(0.001)
                return httpx.Response(200, headers={"Mcp-Session-Id": session}, json={
                    "jsonrpc": "2.0", "id": 1, "result": {"protocolVersion": PROTOCOL_VERSION},
                })
            self.assertEqual(request.headers["mcp-session-id"], sessions[task])
            if payload["method"] == "notifications/initialized":
                return httpx.Response(202)
            args = payload["params"]["arguments"]
            self.assertNotIn("session_id", args)
            return httpx.Response(200, json={"jsonrpc": "2.0", "id": 2, "result": {
                "content": [{"type": "text", "text": args["objective"]}],
            }})

        self.handler = interleaved
        responses = await asyncio.gather(*(self.call("query " + str(i)) for i in range(4)))
        self.assertEqual([r.json()["msg"] for r in responses], ["query " + str(i) for i in range(4)])
        self.assertEqual(len(set(sessions.values())), 4)

    async def test_old_search_provider_is_preserved(self):
        with patch("app.main.DuckDuckGoSearchRun") as duck:
            duck.return_value.run.return_value = "DuckDuckGo result"
            response = await self.http.post("/search", json={"command": "existing choice"})
        self.assertEqual(response.json(), {"msg": "DuckDuckGo result", "wrapper": True})
        duck.return_value.run.assert_called_once_with("existing choice")
        self.assertEqual(self.requests, [])

    async def test_invalid_input_does_not_call_mcp(self):
        for query in (" ", "x" * 2001):
            self.assertEqual((await self.call(query)).status_code, 422)
        self.assertEqual(self.requests, [])

    async def test_http_error_and_quota_are_explicit_failures(self):
        for status in (401, 403, 429, 500):
            self.tool_status = status
            self.tool_body = "upstream private details"
            response = await self.call()
            self.assertIn("No search results are available", response.json()["msg"])
            self.assertNotIn("private details", response.json()["msg"])
            self.assertTrue(response.json()["wrapper"])

    async def test_bad_rpc_and_empty_results_are_explicit_failures(self):
        for body in ("bad json", '[]', '{"jsonrpc":"2.0","id":2,"error":{"code":-1}}',
                     '{"jsonrpc":"2.0","id":99,"result":{}}', "x" * (MAX_RESPONSE_BYTES + 1)):
            self.tool_body = body
            self.assertIn("search failed", (await self.call()).json()["msg"])
        self.tool_body = None
        for result in ({"isError": True}, {"content": []}, {"content": "invalid"},
                       {"content": [{"type": "text", "text": 1}]}):
            self.tool_result = result
            self.assertIn("search failed", (await self.call()).json()["msg"])

    async def test_network_timeout_and_transport_error(self):
        for error in (httpx.ReadTimeout("timeout"), httpx.ConnectError("unavailable")):
            self.tool_exception = error
            response = await self.call()
            self.assertIn("No search results are available", response.json()["msg"])
            self.assertTrue(all(client.is_closed for client in self.clients))

    async def test_overall_deadline(self):
        original = self.handler

        async def slow(request):
            if json.loads(request.content)["method"] == "tools/call":
                await asyncio.sleep(5)
            return await original(request)

        self.handler = slow
        with patch("app.parallel.SEARCH_TIMEOUT", 0.01):
            self.assertIn("timed out", (await self.call()).json()["msg"])
        self.assertTrue(all(client.is_closed for client in self.clients))

    async def test_cancellation_propagates_and_closes_client(self):
        entered = asyncio.Event()
        stopped = asyncio.Event()
        original = self.handler

        async def slow(request):
            if json.loads(request.content)["method"] == "tools/call":
                entered.set()
                try:
                    await asyncio.sleep(5)
                finally:
                    stopped.set()
            return await original(request)

        self.handler = slow
        task = asyncio.create_task(self.call())
        await asyncio.wait_for(entered.wait(), timeout=1)
        task.cancel()
        with self.assertRaises(asyncio.CancelledError):
            await task
        self.assertTrue(stopped.is_set())
        self.assertTrue(all(client.is_closed for client in self.clients))


if __name__ == "__main__":
    unittest.main()
