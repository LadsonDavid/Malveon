package backend

func setupRoutes(router *Router) {
	router.POST("/refund", refundHandler)
	router.GET("/cancel-order-info", cancelInfoHandler) // unrelated endpoint - not the same path the caller actually calls
	router.POST("/settings", settingsHandler)            // registered as POST, but the caller uses PUT
	router.GET("/profile", profileHandler)                // nothing calls this - stays NOT TESTED, never guessed unreachable
}
