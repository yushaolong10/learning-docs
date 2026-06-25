import os

from anthropic import Anthropic

BASE_URL = "https://api.deepseek.com/anthropic"
client = Anthropic(api_key=os.getenv("DEMO_API_KEY"), base_url=BASE_URL)

MODEL = "deepseek-v4-flash"
SYSTEM = f"You are a personal assistant agent."
def llm(messages: list):
    response = client.messages.create(
        model=MODEL, system=SYSTEM, messages=messages,
        max_tokens=8000,
    )
    # Append assistant turn
    messages.append({"role": "assistant", "content": response.content})
    return

if __name__ == "__main__":
    history = []
    while True:
        try:
            query = input("\033[36mchatbot >> \033[0m")
            if query.strip() == "":
                continue
            if query.strip().lower() in ("q", "exit"):
                break
        except (EOFError, KeyboardInterrupt):
            break

        history.append({"role": "user", "content": query})
        llm(history)
        response_content = history[-1]["content"]
        if isinstance(response_content, list):
            for block in response_content:
                if hasattr(block, "text"):
                    print(block.text)
        print()
