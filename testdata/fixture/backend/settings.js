const app = require("express")();

app.post("/settings", function (req, res) {
  // save settings - registered as POST, but the frontend calls it with PUT
});
