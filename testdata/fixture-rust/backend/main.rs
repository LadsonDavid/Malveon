fn build_router() -> Router {
    Router::new()
        .route("/refund", post(refund_handler))
        .route("/cancel-order-info", get(cancel_info_handler)) // unrelated endpoint - not the same path the caller actually calls
        .route("/settings", post(settings_handler)) // registered as POST, but the caller uses PUT
        .route("/profile", get(profile_handler)) // nothing calls this - stays NOT TESTED, never guessed unreachable
}
