import requests
from config import TOGETHER_AI_API_KEY

EMBEDDING_MODEL = "togethercomputer/m2-bert-80M-8k-retrieval"


def embed_texts(texts: list[str]) -> list[list[float]]:
    """Call TogetherAI Embeddings API and return vectors."""
    resp = requests.post(
        "https://api.together.xyz/v1/embeddings",
        headers={"Authorization": f"Bearer {TOGETHER_AI_API_KEY}"},
        json={"model": EMBEDDING_MODEL, "input": texts},
        timeout=60,
    )
    resp.raise_for_status()
    data = resp.json()["data"]
    return [d["embedding"] for d in sorted(data, key=lambda x: x["index"])]
