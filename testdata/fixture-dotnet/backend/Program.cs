var app = builder.Build();

app.MapPost("/refund", () => Results.Ok());
app.MapGet("/cancel-order-info", () => Results.Ok()); // unrelated endpoint - not the same path the caller actually calls
app.MapPost("/settings", () => Results.Ok()); // registered as POST, but the caller uses PUT
app.MapGet("/profile", () => Results.Ok()); // nothing calls this - stays NOT TESTED, never guessed unreachable

app.Run();
