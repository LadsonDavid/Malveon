import requests


def call_refund():
    requests.post("/refund")


def call_cancel_order():
    requests.get("/cancel-order")


def call_update_settings():
    requests.put("/settings")
