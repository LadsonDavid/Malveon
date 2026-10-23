// refund button
function handleRefundClick() {
  fetch("/refund", { method: "POST" });
}

// cancel order button
function handleCancelClick() {
  fetch("/cancel-order");
}

document.getElementById("refund").addEventListener("click", handleRefundClick);
document.getElementById("cancel").addEventListener("click", handleCancelClick);
