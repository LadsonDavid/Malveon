package extractor

import (
	"testing"

	"github.com/LadsonDavid/Malveon/pkg/graph"
)

func TestScanRust_ActixAttribute(t *testing.T) {
	src := `
#[get("/refund")]
async fn refund_handler() -> impl Responder {
    HttpResponse::Ok()
}
`
	sites := scanRust(src)
	if len(sites) != 1 {
		t.Fatalf("expected 1 call site, got %d: %+v", len(sites), sites)
	}
	if sites[0].kind != graph.RouteHandler || sites[0].method != "GET" {
		t.Fatalf("unexpected site: %+v", sites[0])
	}
	path, ok := literalPath(sites[0].argRaw)
	if !ok || path != "/refund" {
		t.Errorf("expected literal path /refund, got %q (ok=%v)", path, ok)
	}
}

func TestScanRust_AxumBuilderCall(t *testing.T) {
	src := `
let app = Router::new()
    .route("/refund", get(refund_handler))
    .route("/orders", post(create_order));
`
	sites := scanRust(src)
	if len(sites) != 2 {
		t.Fatalf("expected 2 call sites, got %d: %+v", len(sites), sites)
	}
	if sites[0].method != "GET" || sites[1].method != "POST" {
		t.Fatalf("expected verbs GET, POST from the nested calls, got %q, %q", sites[0].method, sites[1].method)
	}
	if path, ok := literalPath(sites[0].argRaw); !ok || path != "/refund" {
		t.Errorf("expected literal path /refund, got %q (ok=%v)", path, ok)
	}
}

func TestScanJava_SpringMethodMapping(t *testing.T) {
	src := `
@RestController
public class RefundController {
    @GetMapping("/refund")
    public Refund getRefund() { return null; }

    @RequestMapping("/legacy-refund")
    public void legacy() {}
}
`
	sites := scanJava(src)
	if len(sites) != 2 {
		t.Fatalf("expected 2 call sites, got %d: %+v", len(sites), sites)
	}
	if sites[0].method != "GET" {
		t.Errorf("expected GET from @GetMapping, got %q", sites[0].method)
	}
	if sites[1].method != "" {
		t.Errorf("expected an empty method for the ambiguous @RequestMapping form, got %q", sites[1].method)
	}
	if path, ok := literalPath(sites[0].argRaw); !ok || path != "/refund" {
		t.Errorf("expected literal path /refund, got %q (ok=%v)", path, ok)
	}
}

func TestScanRuby_RailsRoutes(t *testing.T) {
	src := `
Rails.application.routes.draw do
  get '/refund', to: 'orders#refund'
  post('/orders', to: 'orders#create')
end
`
	sites := scanRuby(src)
	if len(sites) != 2 {
		t.Fatalf("expected 2 call sites, got %d: %+v", len(sites), sites)
	}
	if sites[0].method != "GET" || sites[1].method != "POST" {
		t.Fatalf("expected verbs GET, POST, got %q, %q", sites[0].method, sites[1].method)
	}
	if path, ok := literalPath(sites[0].argRaw); !ok || path != "/refund" {
		t.Errorf("expected literal path /refund, got %q (ok=%v)", path, ok)
	}
	if path, ok := literalPath(sites[1].argRaw); !ok || path != "/orders" {
		t.Errorf("expected literal path /orders, got %q (ok=%v)", path, ok)
	}
}

// TestScanRuby_InterpolatedPathIsNotLiteral proves Ruby's #{...}
// interpolation (a real gap in the shared literalPath classifier, which
// only knows JS's ${...} and Python's f-string {...}) is correctly
// rejected rather than silently misread as a resolved literal.
func TestScanRuby_InterpolatedPathIsNotLiteral(t *testing.T) {
	src := `get "/users/#{id}", to: 'users#show'`
	sites := scanRuby(src)
	if len(sites) != 1 {
		t.Fatalf("expected 1 call site, got %d: %+v", len(sites), sites)
	}
	if _, ok := literalPath(sites[0].argRaw); ok {
		t.Errorf("expected an interpolated Ruby string to be non-literal, got a resolved path from %q", sites[0].argRaw)
	}
}

func TestScanPHP_LaravelRoutes(t *testing.T) {
	src := `
Route::get('/refund', [OrderController::class, 'refund']);
Route::post("/orders", [OrderController::class, 'create']);
`
	sites := scanPHP(src)
	if len(sites) != 2 {
		t.Fatalf("expected 2 call sites, got %d: %+v", len(sites), sites)
	}
	if sites[0].method != "GET" || sites[1].method != "POST" {
		t.Fatalf("expected verbs GET, POST, got %q, %q", sites[0].method, sites[1].method)
	}
	if path, ok := literalPath(sites[0].argRaw); !ok || path != "/refund" {
		t.Errorf("expected literal path /refund, got %q (ok=%v)", path, ok)
	}
}

// TestScanPHP_InterpolatedPathIsNotLiteral proves PHP's double-quoted
// $variable/{$variable} interpolation (a real gap in the shared
// literalPath classifier — it has no PHP-specific rule at all) is
// correctly rejected rather than silently misread as a resolved literal.
func TestScanPHP_InterpolatedPathIsNotLiteral(t *testing.T) {
	src := `Route::get("/users/{$id}", [UserController::class, 'show']);`
	sites := scanPHP(src)
	if len(sites) != 1 {
		t.Fatalf("expected 1 call site, got %d: %+v", len(sites), sites)
	}
	if _, ok := literalPath(sites[0].argRaw); ok {
		t.Errorf("expected an interpolated PHP string to be non-literal, got a resolved path from %q", sites[0].argRaw)
	}
}

// TestScanPHP_SingleQuotedDollarSignIsStillLiteral proves the PHP
// interpolation guard doesn't over-trigger: a single-quoted PHP string
// never interpolates, so a literal '$' inside one (unusual for a route
// path, but a real single-quote case) must still resolve normally.
func TestScanPHP_SingleQuotedDollarSignIsStillLiteral(t *testing.T) {
	got := phpArgRaw(`'/price/$1'`)
	if got != `'/price/$1'` {
		t.Errorf("expected a single-quoted string to pass through unchanged, got %q", got)
	}
}

func TestScanDotnet_MinimalAPI(t *testing.T) {
	src := `
var app = builder.Build();
app.MapGet("/refund", () => Results.Ok());
app.MapPost("/orders", (Order o) => Results.Created());
`
	sites := scanDotnet(src)
	if len(sites) != 2 {
		t.Fatalf("expected 2 call sites, got %d: %+v", len(sites), sites)
	}
	if sites[0].method != "GET" || sites[1].method != "POST" {
		t.Fatalf("expected verbs GET, POST, got %q, %q", sites[0].method, sites[1].method)
	}
	if path, ok := literalPath(sites[0].argRaw); !ok || path != "/refund" {
		t.Errorf("expected literal path /refund, got %q (ok=%v)", path, ok)
	}
}

// TestScanDotnet_InterpolatedStringIsNotLiteral proves C#'s $"..."
// string interpolation is rejected — not via a special case, but because
// literalPath's existing quote-first-character check already happens to
// reject it (the leading '$' isn't a recognized quote character).
func TestScanDotnet_InterpolatedStringIsNotLiteral(t *testing.T) {
	src := `app.MapGet($"/users/{id}", () => Results.Ok());`
	sites := scanDotnet(src)
	if len(sites) != 1 {
		t.Fatalf("expected 1 call site, got %d: %+v", len(sites), sites)
	}
	if _, ok := literalPath(sites[0].argRaw); ok {
		t.Errorf("expected an interpolated C# string to be non-literal, got a resolved path from %q", sites[0].argRaw)
	}
}
