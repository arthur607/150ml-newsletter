import numpy as np
from clustering import cluster_embeddings


def test_cluster_embeddings_groups_similar():
    """Items with identical vectors should land in the same cluster."""
    group_a = np.array([1.0, 0.0, 0.0, 0.0])
    group_b = np.array([0.0, 1.0, 0.0, 0.0])

    vectors = [
        group_a + np.random.normal(0, 0.01, 4),
        group_a + np.random.normal(0, 0.01, 4),
        group_a + np.random.normal(0, 0.01, 4),
        group_b + np.random.normal(0, 0.01, 4),
        group_b + np.random.normal(0, 0.01, 4),
        group_b + np.random.normal(0, 0.01, 4),
    ]

    labels = cluster_embeddings(vectors)
    assert len(labels) == 6
    assert labels[0] == labels[1] == labels[2]
    assert labels[3] == labels[4] == labels[5]
    assert labels[0] != labels[3]
