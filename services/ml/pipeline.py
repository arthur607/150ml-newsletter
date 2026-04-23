import logging
from collections import defaultdict

from clustering import cluster_embeddings
from db import fetch_unprocessed_items, save_cluster, assign_cluster
from embeddings import embed_texts
from labeler import label_cluster

log = logging.getLogger(__name__)


def run(conn, date: str):
    items = fetch_unprocessed_items(conn, date)
    if not items:
        log.info("no unprocessed items for %s", date)
        return

    log.info("clustering %d items for %s", len(items), date)

    texts = [item["embedding_text"] for item in items]
    vectors = embed_texts(texts)
    labels = cluster_embeddings(vectors)

    clusters: dict[int, list[int]] = defaultdict(list)
    for idx, label in enumerate(labels):
        if label != -1:
            clusters[label].append(idx)

    log.info("found %d clusters (%d noise items)", len(clusters), labels.count(-1))

    for cluster_label, indices in clusters.items():
        cluster_texts = [texts[i] for i in indices]
        topic_label = label_cluster(cluster_texts)
        cluster_id = save_cluster(conn, date, topic_label, len(indices))

        for idx in indices:
            assign_cluster(conn, items[idx]["id"], cluster_id)

        log.info("saved cluster '%s' with %d items", topic_label, len(indices))

    conn.commit()
