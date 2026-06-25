import os
from pathlib import Path

from anthropic import Anthropic

BASE_URL = "https://api.deepseek.com/anthropic"
client = Anthropic(api_key=os.getenv("DEMO_API_KEY"), base_url=BASE_URL)

WORKDIR = Path.cwd()
def safe_path(p: str) -> Path:
    path = (WORKDIR / p).resolve()
    if not path.is_relative_to(WORKDIR):
        raise ValueError(f"Path escapes workspace: {p}")
    return path

def run_read(path: str, limit: int = None) -> str:
    try:
        text = safe_path(path).read_text()
        lines = text.splitlines()
        if limit and limit < len(lines):
            lines = lines[:limit] + [f"... ({len(lines) - limit} more lines)"]
        return "\n".join(lines)[:50000]
    except Exception as e:
        return f"Error: {e}"

def run_write(path: str, content: str) -> str:
    try:
        fp = safe_path(path)
        fp.parent.mkdir(parents=True, exist_ok=True)
        fp.write_text(content)
        return f"Wrote {len(content)} bytes to {path}"
    except Exception as e:
        return f"Error: {e}"

def run_edit(path: str, old_text: str, new_text: str) -> str:
    try:
        fp = safe_path(path)
        content = fp.read_text()
        if old_text not in content:
            return f"Error: Text not found in {path}"
        fp.write_text(content.replace(old_text, new_text, 1))
        return f"Edited {path}"
    except Exception as e:
        return f"Error: {e}"

# -- The dispatch map: {tool_name: handler} --
TOOL_HANDLERS = {
    "read_file":  lambda **kw: run_read(kw["path"], kw.get("limit")),
    "write_file": lambda **kw: run_write(kw["path"], kw["content"]),
    "edit_file":  lambda **kw: run_edit(kw["path"], kw["old_text"], kw["new_text"]),
}

TOOLS = [
    {
        "name": "read_file", 
        "description": "Read file contents.", 
        "input_schema": {
            "type": "object", 
            "properties": {
                "path": {
                    "type": "string",
                }, 
                "limit": {
                    "type": "integer",
                },
            }, 
            "required": ["path"],
        },
    },
    {
        "name": "write_file", 
        "description": "Write content to file.", 
        "input_schema": {
            "type": "object", 
            "properties": {
                "path": {
                    "type": "string",
                }, 
                "content": {
                    "type": "string",
                },
            }, 
            "required": ["path", "content"],
        },
    },
    {
        "name": "edit_file", 
        "description": "Replace exact text in file.", 
        "input_schema": {
            "type": "object", 
            "properties": {
                "path": {
                    "type": "string",
                }, 
                "old_text": {
                    "type": "string",
                }, 
                "new_text": {
                    "type": "string",
                },
            }, 
            "required": ["path", "old_text", "new_text"],
        },
    },
]

MODEL = "deepseek-v4-flash"
SYSTEM   = f"""You are a personal assistant agent resides in ${WORKDIR}.
Use the tools provided to solve tasks. Act, do not explain."""
def agent_loop(messages: list):
    while True:
        response = client.messages.create(
            model=MODEL, system=SYSTEM, messages=messages,
            tools=TOOLS, max_tokens=8000,
        )
        messages.append({"role": "assistant", "content": response.content})
        if response.stop_reason != "tool_use":
            return
        results = []
        for block in response.content:
            if block.type == "tool_use":
                handler = TOOL_HANDLERS.get(block.name)
                output = handler(**block.input) if handler else f"Unknown tool: {block.name}"
                print(f"> {block.name}: {block.input}")
                print(output[:200])
                results.append({"type": "tool_result", "tool_use_id": block.id, "content": output})
        messages.append({"role": "user", "content": results})

if __name__ == "__main__":
    history = []
    while True:
        try:
            query = input("\033[36magent >> \033[0m")
            if query.strip() == "":
                continue
            if query.strip().lower() in ("q", "exit"):
                break
        except (EOFError, KeyboardInterrupt):
            break

        history.append({"role": "user", "content": query})
        agent_loop(history)
        response_content = history[-1]["content"]
        if isinstance(response_content, list):
            for block in response_content:
                if hasattr(block, "text"):
                    print(block.text)
        print()