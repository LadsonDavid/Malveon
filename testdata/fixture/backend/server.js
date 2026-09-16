const app = require("express")();

app.post("/refund", function (req, res) {
  // process refund
});

app.get("/cancel-order-info", function (req, res) {
  // unrelated endpoint - not the same path the frontend actually calls
});
