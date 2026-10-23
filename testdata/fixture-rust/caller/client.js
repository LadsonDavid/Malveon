function callRefund() {
    fetch("/refund", { method: "POST" });
}

function callCancelOrder() {
    fetch("/cancel-order");
}

function callUpdateSettings() {
    fetch("/settings", { method: "PUT" });
}
