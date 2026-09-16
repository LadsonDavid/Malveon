// refund button
function handleRefundClick() {
  fetch("/refund", { method: "POST" });
}

// cancel order button
function handleCancelClick() {
  fetch("/cancel-order");
}
