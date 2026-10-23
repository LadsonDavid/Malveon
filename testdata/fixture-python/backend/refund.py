from flask import Flask

app = Flask(__name__)


@app.post("/refund")
def refund():
    pass


@app.get("/cancel-order-info")
def cancel_order_info():
    # unrelated endpoint - not the same path the caller actually calls
    pass


@app.post("/settings")
def update_settings():
    # save settings - registered as POST, but the caller uses PUT
    pass


@app.get("/profile")
def profile():
    pass
