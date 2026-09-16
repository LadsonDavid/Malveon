package caller

func callRefund() {
	httpClient.Post("/refund")
}

func callCancelOrder() {
	httpClient.Get("/cancel-order")
}

func callUpdateSettings() {
	httpClient.Put("/settings")
}
