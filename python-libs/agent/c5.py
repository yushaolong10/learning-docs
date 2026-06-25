import os
import subprocess
from pathlib import Path

from anthropic import Anthropic

BASE_URL = "https://api.deepseek.com/anthropic"
client = Anthropic(api_key=os.getenv("DEMO_API_KEY"), base_url=BASE_URL)

WORKDIR = Path.cwd()
# -- Tool implementations shared by parent and child --
def safe_path(p: str) -> Path:
    path = (WORKDIR / p).resolve()
    if not path.is_relative_to(WORKDIR):
        raise ValueError(f"Path escapes workspace: {p}")
    return path

def run_bash(command: str) -> str:
    dangerous = ["rm -rf /", "sudo", "shutdown", "reboot", "> /dev/"]
    if any(d in command for d in dangerous):
        return "Error: Dangerous command blocked"
    try:
        r = subprocess.run(command, shell=True, cwd=WORKDIR,
                           capture_output=True, text=True, timeout=120)
        out = (r.stdout + r.stderr).strip()
        return out[:50000] if out else "(no output)"
    except subprocess.TimeoutExpired:
        return "Error: Timeout (120s)"
    except (FileNotFoundError, OSError) as e:
        return f"Error: {e}"

def run_read(path: str, limit: int = None) -> str:
    try:
        lines = safe_path(path).read_text().splitlines()
        if limit and limit < len(lines):
            lines = lines[:limit] + [f"... ({len(lines) - limit} more)"]
        return "\n".join(lines)[:50000]
    except Exception as e:
        return f"Error: {e}"

def run_write(path: str, content: str) -> str:
    try:
        fp = safe_path(path)
        fp.parent.mkdir(parents=True, exist_ok=True)
        fp.write_text(content)
        return f"Wrote {len(content)} bytes"
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

TOOL_HANDLERS = {
    "bash":       lambda **kw: run_bash(kw["command"]),
    "read_file":  lambda **kw: run_read(kw["path"], kw.get("limit")),
    "write_file": lambda **kw: run_write(kw["path"], kw["content"]),
    "edit_file":  lambda **kw: run_edit(kw["path"], kw["old_text"], kw["new_text"]),
    "subagent": lambda **kw: run_subagent(kw["prompt"], kw.get("description", "subtask")),
}

# Child gets all base tools except task (no recursive spawning)
CHILD_TOOLS = [
    {"name": "bash", "description": "Run a shell command.",
     "input_schema": {"type": "object", "properties": {"command": {"type": "string"}}, "required": ["command"]}},
    {"name": "read_file", "description": "Read file contents.",
     "input_schema": {"type": "object", "properties": {"path": {"type": "string"}, "limit": {"type": "integer"}}, "required": ["path"]}},
    {"name": "write_file", "description": "Write content to file.",
     "input_schema": {"type": "object", "properties": {"path": {"type": "string"}, "content": {"type": "string"}}, "required": ["path", "content"]}},
    {"name": "edit_file", "description": "Replace exact text in file.",
     "input_schema": {"type": "object", "properties": {"path": {"type": "string"}, "old_text": {"type": "string"}, "new_text": {"type": "string"}}, "required": ["path", "old_text", "new_text"]}},
]

MODEL = "deepseek-v4-flash"
SUBAGENT_SYSTEM = f"""You are an assistant subagent at {WORKDIR}.
Complete the given task, then summarize your findings."""
# -- Subagent: fresh context, filtered tools, summary-only return --
def run_subagent(prompt: str, description: str) -> str:
    print(f"-------- [subagent] running task {description} --------")
    sub_messages = [{"role": "user", "content": prompt}]  # fresh context
    agent_loop(SUBAGENT_SYSTEM, sub_messages, CHILD_TOOLS)
    response_content = sub_messages[-1]["content"]
    print(f"-------- [subagent] finished task {description} --------")
    return "".join(b.text for b in response_content if hasattr(b, "text")) or "(no summary)"

# -- Parent tools: base tools + task dispatcher --
PARENT_TOOLS = CHILD_TOOLS + [
    {
        "name": "subagent", 
        "description": "Spawn a subagent with fresh context when you feel the requirement is complicated to fullfill. It shares the filesystem but not conversation history.", 
        "input_schema": {
            "type": "object", 
            "properties": {
                "prompt": {
                    "type": "string",
                }, 
                "description": {
                    "type": "string", 
                    "description": "Short description of the task",
                },
            }, 
            "required": ["prompt"],
        },
    },
]

SYSTEM = f"""You are an assistant agent at {WORKDIR}.
Use the task tool to delegate exploration or subtasks."""
def agent_loop(system: str, messages: list, tools: list):
    while True:
        response = client.messages.create(
            model=MODEL, system=system, messages=messages,
            tools=tools, max_tokens=8000,
        )
        messages.append({"role": "assistant", "content": response.content})
        if response.stop_reason != "tool_use":
            return
        results = []
        for block in response.content:
            if block.type == "tool_use":
                handler = TOOL_HANDLERS.get(block.name)
                input_repr = str(block.input)
                print(f"> \033[31m{block.name}\033[0m: \033[34m{input_repr}\033[0m")
                try:
                    output = handler(**block.input) if handler else f"Unknown tool: {block.name}"
                except Exception as e:
                    output = f"Error: {e}"
                output_repr = str(output)
                if len(output_repr) > 200:
                    output_repr = output_repr[:200] + "..."
                print(f"\033[32m{output_repr}\033[0m\n")
                results.append({"type": "tool_result", "tool_use_id": block.id, "content": str(output)})
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
        agent_loop(SYSTEM, history, PARENT_TOOLS)
        response_content = history[-1]["content"]
        if isinstance(response_content, list):
            for block in response_content:
                if hasattr(block, "text"):
                    print(block.text)
        print()