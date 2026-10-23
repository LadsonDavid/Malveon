@RestController
public class RefundController {
    @PostMapping("/refund")
    public void refund() {}

    @GetMapping("/cancel-order-info") // unrelated endpoint - not the same path the caller actually calls
    public void cancelInfo() {}

    @PostMapping("/settings") // registered as POST, but the caller uses PUT
    public void settings() {}

    @GetMapping("/profile") // nothing calls this - stays NOT TESTED, never guessed unreachable
    public void profile() {}
}
