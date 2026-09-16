import requests


def call_profile(user_id):
    # dynamic path built from an f-string - never resolvable to a literal
    requests.get(f"/profile/{user_id}")
