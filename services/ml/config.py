import os
from dotenv import load_dotenv

load_dotenv()

DATABASE_URL = os.environ["DATABASE_URL"]
TOGETHER_AI_API_KEY = os.environ["TOGETHER_AI_API_KEY"]
