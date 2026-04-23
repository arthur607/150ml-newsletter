import logging
import sys
from datetime import date

from db import connect
from pipeline import run

logging.basicConfig(
    level=logging.INFO,
    format='{"time":"%(asctime)s","level":"%(levelname)s","msg":"%(message)s"}',
)

if __name__ == "__main__":
    today = date.today().isoformat()
    target_date = sys.argv[1] if len(sys.argv) > 1 else today

    conn = connect()
    try:
        run(conn, target_date)
    finally:
        conn.close()
