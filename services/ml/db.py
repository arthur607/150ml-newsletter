import psycopg
from config import DATABASE_URL


def connect():
    return psycopg.connect(DATABASE_URL)


def fetch_unprocessed_items(conn, date: str) -> list[dict]:
    """Return processed_items without cluster_id ingested on the given date."""
    with conn.cursor() as cur:
        cur.execute("""
            SELECT pi.id, pi.embedding_text
            FROM processed_items pi
            JOIN ingested_items ii ON ii.id = pi.ingested_item_id
            WHERE pi.cluster_id IS NULL
              AND ii.ingested_at::date = %s
        """, (date,))
        return [{"id": str(row[0]), "embedding_text": row[1]} for row in cur.fetchall()]


def save_cluster(conn, briefing_date: str, label: str, item_count: int) -> str:
    with conn.cursor() as cur:
        cur.execute("""
            INSERT INTO clusters (briefing_date, label, item_count)
            VALUES (%s, %s, %s) RETURNING id
        """, (briefing_date, label, item_count))
        return str(cur.fetchone()[0])


def assign_cluster(conn, item_id: str, cluster_id: str):
    with conn.cursor() as cur:
        cur.execute(
            "UPDATE processed_items SET cluster_id = %s WHERE id = %s",
            (cluster_id, item_id),
        )
