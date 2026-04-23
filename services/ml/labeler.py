import requests
from config import TOGETHER_AI_API_KEY

LLM_MODEL = "meta-llama/Llama-3.2-11B-Vision-Instruct-Turbo"


def label_cluster(embedding_texts: list[str]) -> str:
    """Ask the LLM to generate a short topic label for a cluster."""
    sample = "\n".join(f"- {t}" for t in embedding_texts[:10])
    prompt = (
        "The following are brief descriptions of articles grouped together by topic similarity.\n"
        f"{sample}\n\n"
        "Give this cluster a concise topic label (5 words or fewer). "
        "Return only the label, no punctuation."
    )
    resp = requests.post(
        "https://api.together.xyz/v1/chat/completions",
        headers={"Authorization": f"Bearer {TOGETHER_AI_API_KEY}"},
        json={
            "model": LLM_MODEL,
            "messages": [{"role": "user", "content": prompt}],
            "max_tokens": 20,
        },
        timeout=30,
    )
    resp.raise_for_status()
    return resp.json()["choices"][0]["message"]["content"].strip()
