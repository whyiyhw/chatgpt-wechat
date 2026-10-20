"""Optional anonymous Parallel Search MCP adapter for the HTTP plugin contract."""

import asyncio
import json

import httpx
from fastapi import APIRouter, Depends
from pydantic import BaseModel


MCP_URL = "https://search.parallel.ai/mcp"
USER_AGENT = "chatgpt-wechat/parallel-search"
PROTOCOL_VERSION = "2025-03-26"
SEARCH_TIMEOUT = 30
MAX_RESPONSE_BYTES = 1024 * 1024
router = APIRouter()


class ParallelQuery(BaseModel):
    command: str


class ParallelResponse(BaseModel):
    msg: str
    wrapper: bool = True


class ParallelError(Exception):
    pass


async def parallel_client():
    async with httpx.AsyncClient(timeout=20, follow_redirects=False) as client:
        yield client


def _result(message, request_id):
    if not isinstance(message, dict) or message.get("jsonrpc") != "2.0":
        raise ParallelError("Invalid MCP response")
    if message.get("id") != request_id:
        return None
    if "error" in message or not isinstance(message.get("result"), dict):
        raise ParallelError("MCP request failed")
    return message["result"]


async def _rpc(client, headers, request_id, method, params):
    payload = {"jsonrpc": "2.0", "id": request_id, "method": method, "params": params}
    async with client.stream("POST", MCP_URL, headers=headers, json=payload) as response:
        response.raise_for_status()
        session = response.headers.get("mcp-session-id")
        size = 0
        if response.headers.get("content-type", "").split(";")[0] == "text/event-stream":
            data = []
            async for line in response.aiter_lines():
                size += len(line.encode("utf-8"))
                if size > MAX_RESPONSE_BYTES:
                    raise ParallelError("MCP response too large")
                if line.startswith("data:"):
                    data.append(line[5:].lstrip(" "))
                elif not line and data:
                    result = _result(json.loads("\n".join(data)), request_id)
                    if result is not None:
                        return result, session
                    data = []
            if data:
                result = _result(json.loads("\n".join(data)), request_id)
                if result is not None:
                    return result, session
        else:
            body = bytearray()
            async for chunk in response.aiter_bytes():
                size += len(chunk)
                if size > MAX_RESPONSE_BYTES:
                    raise ParallelError("MCP response too large")
                body.extend(chunk)
            result = _result(json.loads(body), request_id)
            if result is not None:
                return result, session
    raise ParallelError("Missing MCP response")


async def search(client, query):
    headers = {
        "User-Agent": USER_AGENT,
        "Accept": "application/json, text/event-stream",
        "Content-Type": "application/json",
    }
    initialized, session = await _rpc(client, headers, 1, "initialize", {
        "protocolVersion": PROTOCOL_VERSION,
        "capabilities": {},
        "clientInfo": {"name": "chatgpt-wechat", "version": "1.0"},
    })
    if initialized.get("protocolVersion") != PROTOCOL_VERSION:
        raise ParallelError("Unsupported MCP protocol version")
    headers["MCP-Protocol-Version"] = PROTOCOL_VERSION
    if session:
        headers["Mcp-Session-Id"] = session
    response = await client.post(MCP_URL, headers=headers, json={
        "jsonrpc": "2.0", "method": "notifications/initialized",
    })
    response.raise_for_status()
    # The existing plugin request carries only command, not conversation context.
    # Omit optional session_id rather than sharing an identity between users.
    result, _ = await _rpc(client, headers, 2, "tools/call", {
        "name": "web_search",
        "arguments": {"objective": query, "search_queries": [query]},
    })
    if result.get("isError"):
        raise ParallelError("MCP tool failed")
    content = result.get("content")
    if not isinstance(content, list):
        raise ParallelError("Missing MCP tool content")
    text = "\n\n".join(
        block["text"] for block in content
        if isinstance(block, dict) and block.get("type") == "text"
        and isinstance(block.get("text"), str) and block["text"].strip()
    )
    if not text:
        raise ParallelError("Empty MCP tool content")
    return text


@router.post("/parallel-search", response_model=ParallelResponse)
async def parallel_search(item: ParallelQuery, client=Depends(parallel_client)):
    query = item.command.strip()
    if not query or len(query) > 2000:
        return ParallelResponse(msg=(
            "Parallel search needs a non-empty query of at most 2000 characters. "
            "No search results are available."
        ))
    try:
        text = await asyncio.wait_for(search(client, query), timeout=SEARCH_TIMEOUT)
    except (asyncio.TimeoutError, httpx.TimeoutException):
        text = "Parallel search timed out. No search results are available; please try again later."
    except (ParallelError, httpx.HTTPError, ValueError):
        text = "Parallel search failed. No search results are available; please try again later."
    # RunPlugin expects msg/wrapper even on failure; HTTP error detail alone would
    # be decoded as an empty successful plugin response by that existing caller.
    return ParallelResponse(msg=text)
