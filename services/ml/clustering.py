import numpy as np
import umap
import hdbscan


def cluster_embeddings(vectors: list[list[float]]) -> list[int]:
    """
    Reduce dimensionality with UMAP then cluster with HDBSCAN.
    Returns a list of integer cluster labels (same length as input).
    Label -1 means noise / unclustered.
    """
    arr = np.array(vectors, dtype=np.float32)

    n = len(arr)
    n_neighbors = min(5, n - 1)
    n_components = min(2, n - 2) if n > 3 else 2

    reducer = umap.UMAP(
        n_neighbors=n_neighbors,
        n_components=n_components,
        metric="cosine",
        random_state=42,
    )
    reduced = reducer.fit_transform(arr)

    clusterer = hdbscan.HDBSCAN(min_cluster_size=2, metric="euclidean")
    labels = clusterer.fit_predict(reduced)
    return labels.tolist()
