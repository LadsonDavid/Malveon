function saveSettings() {
  fetch("/settings", { method: "PUT" });
}
