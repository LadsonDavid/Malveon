package extractor

import (
	"sort"
	"strings"
	"testing"

	"github.com/LadsonDavid/Malveon/pkg/graph"
)

// routesOf lists "METHOD /path" for every route (Method "" shows as ANY),
// plus " ?" when the path could not be read.
func routesOf(g *graph.Graph) []string {
	var out []string
	for _, n := range g.Nodes {
		if n.Kind != graph.RouteHandler {
			continue
		}
		m := n.Method
		if m == "" {
			m = "ANY"
		}
		s := m + " " + n.Path
		if n.Confidence != graph.Extracted {
			s += " ?"
		}
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

func wantRoutes(t *testing.T, g *graph.Graph, want ...string) {
	t.Helper()
	got := routesOf(g)
	have := map[string]bool{}
	for _, r := range got {
		have[r] = true
	}
	for _, w := range want {
		if !have[w] {
			t.Errorf("missing route %q; have %v", w, got)
		}
	}
}

func wantNoRoute(t *testing.T, g *graph.Graph, bad string) {
	t.Helper()
	for _, r := range routesOf(g) {
		if r == bad {
			t.Errorf("unexpected route %q in %v", bad, routesOf(g))
		}
	}
}

func TestDjangoPathAndRePath(t *testing.T) {
	g := extractProject(t, map[string]string{
		"mysite/urls.py": `
from django.urls import path, re_path
from . import views

urlpatterns = [
    path("articles/2003/", views.special_case_2003),
    path("articles/<int:year>/<slug:slug>/", views.article_detail),
    path("tags/<name>/", views.tag),
    re_path(r"^archive/(?P<year>[0-9]{4})/$", views.archive),
    re_path(r"^legacy/(?:a|b)/$", views.legacy),
]`,
		"mysite/views.py": `def special_case_2003(request): pass`,
	})
	wantRoutes(t, g,
		"ANY /articles/2003", "ANY /articles/:year/:slug", "ANY /tags/:name",
		"ANY /archive/:year", "ANY  ?") // a regex that is one URL only sometimes can't be a path
}

func TestDjangoIncludeJoinsPrefixesAcrossFiles(t *testing.T) {
	g := extractProject(t, map[string]string{
		"config/settings.py": `ROOT_URLCONF = "config.urls"`,
		"config/urls.py": `
from django.urls import include, path
urlpatterns = [
    path("api/", include("shop.urls")),
    path("blog/<slug:blog>/", include("blog.urls")),
    path("admin/", admin.site.urls),
    path("auth/", include("rest_framework.urls")),
]`,
		"shop/urls.py": `
from django.urls import path, include
app_name = "shop"
urlpatterns = [
    path("orders/<int:pk>/", views.order),
    path("cart/", include([
        path("add/", views.add),
        path("remove/<int:item>/", views.remove),
    ])),
]`,
		"blog/urls.py": `urlpatterns = [path("posts/", views.posts)]`,
	})
	wantRoutes(t, g,
		"ANY /api/orders/:pk", "ANY /api/cart/add", "ANY /api/cart/remove/:item",
		"ANY /blog/:blog/posts")
	for _, r := range routesOf(g) {
		if strings.Contains(r, "admin") || strings.Contains(r, "auth") {
			t.Errorf("admin and third-party includes are not routes malveon can read: %v", routesOf(g))
		}
	}
}

func TestDjangoOnlyTheRootUrlconfIsARoot(t *testing.T) {
	// shop/urls.py is only meaningful under /api: it must not also appear at the root.
	g := extractProject(t, map[string]string{
		"config/urls.py": `urlpatterns = [path("api/", include("shop.urls"))]`,
		"shop/urls.py":   `urlpatterns = [path("orders/", views.orders)]`,
	})
	wantRoutes(t, g, "ANY /api/orders")
	wantNoRoute(t, g, "ANY /orders")
}

func TestDjangoMethodsFromDecoratorsAndBaseClasses(t *testing.T) {
	g := extractProject(t, map[string]string{
		"app/urls.py": `
urlpatterns = [
    path("a/", views.both),
    path("b/", views.only_post),
    path("c/", views.Detail.as_view()),
    path("d/", views.Both.as_view()),
    path("e/", views.Gen.as_view()),
    path("f/", login_required(views.only_get)),
    path("g/", views.unknown),
    path("h/", views.Custom.as_view()),
]`,
		"app/views.py": `
from rest_framework.decorators import api_view
from rest_framework import generics
from django.views.decorators.http import require_POST, require_GET

@api_view(["GET", "POST"])
def both(request): pass

@require_POST
def only_post(request): pass

@require_GET
def only_get(request): pass

def unknown(request): pass

class Detail(APIView):
    def get(self, request): pass
    def delete(self, request): pass

class Both(View):
    def get(self, request): pass
    def post(self, request): pass
    def helper(self): pass

class Gen(generics.ListCreateAPIView):
    queryset = X.objects.all()

class Custom(SomethingFromElsewhere):
    def get(self, request): pass
`,
	})
	wantRoutes(t, g,
		"GET /a", "POST /a", "POST /b",
		"GET /c", "DELETE /c", "GET /d", "POST /d",
		"GET /e", "POST /e", "GET /f",
		"ANY /g",
		"ANY /h") // an unknown base class may add more methods: not guessed
	wantNoRoute(t, g, "GET /b")
	wantNoRoute(t, g, "POST /f")
	wantNoRoute(t, g, "POST /c")
}

func TestDjangoRestFrameworkRouters(t *testing.T) {
	g := extractProject(t, map[string]string{
		"api/urls.py": `
from rest_framework.routers import DefaultRouter
from . import views

router = DefaultRouter()
router.register(r"users", views.UserViewSet, basename="user")
router.register("reports", views.ReportViewSet)
router.register("things", views.WeirdViewSet)

urlpatterns = [
    path("health/", views.health),
    path("", include(router.urls)),
]`,
		"api/views.py": `
class UserViewSet(viewsets.ModelViewSet):
    @action(detail=True, methods=["post"], url_path="set-password")
    def set_password(self, request, pk=None): pass

    @action(detail=False)
    def recent(self, request): pass

class ReportViewSet(viewsets.ReadOnlyModelViewSet):
    pass

class WeirdViewSet(SomeCustomBase):
    pass
`,
	})
	wantRoutes(t, g,
		"GET /users", "POST /users",
		"GET /users/:pk", "PUT /users/:pk", "PATCH /users/:pk", "DELETE /users/:pk",
		"POST /users/:pk/set-password", "GET /users/recent",
		"GET /reports", "GET /reports/:pk",
		"ANY /things", "ANY /things/:pk",
		"ANY /health")
	wantNoRoute(t, g, "POST /reports")
	wantNoRoute(t, g, "DELETE /reports/:pk")
}

func TestDjangoRouterUrlsAppendedToUrlpatterns(t *testing.T) {
	g := extractProject(t, map[string]string{
		"urls.py": `
router = SimpleRouter()
router.register(r"items", ItemViewSet)
urlpatterns = [path("x/", views.x)]
urlpatterns += router.urls
`,
		"views.py": `class ItemViewSet(mixins.ListModelMixin, mixins.CreateModelMixin, viewsets.GenericViewSet): pass`,
	})
	wantRoutes(t, g, "ANY /x", "GET /items", "POST /items")
	wantNoRoute(t, g, "DELETE /items/:pk")
}

func TestDjangoTemplatesAreTheCallers(t *testing.T) {
	g := extractProject(t, map[string]string{
		"config/urls.py": `
urlpatterns = [
    path("orders/<int:pk>/cancel/", views.cancel, name="cancel_order"),
    path("orders/", views.orders, name="orders"),
]`,
		"templates/orders.html": `
<form method="post" action="{% url 'cancel_order' order.pk %}"><button>Cancel</button></form>
<a href="{% url 'orders' %}">All</a>
<button hx-post="/orders/1/cancel/">go</button>
<button hx-get="{% url 'orders' %}">go</button>
<form action="/orders/" method="get"></form>
`,
	})
	calls := map[string]bool{}
	for _, n := range g.Nodes {
		if n.Kind == graph.NetworkCall && n.Confidence == graph.Extracted {
			calls[n.Method+" "+n.Path] = true
		}
	}
	for _, w := range []string{"POST /orders/:pk/cancel", "GET /orders"} {
		if !calls[w] {
			t.Errorf("missing call %q; have %v", w, calls)
		}
	}
	if !calls["POST /orders/1/cancel"] {
		t.Errorf("hx-post with a literal URL is a call: %v", calls)
	}
}

func TestDjangoNothingFromAComment(t *testing.T) {
	g := extractProject(t, map[string]string{
		"urls.py": `
urlpatterns = [
    # path("old/", views.old),
    path("new/", views.new),
]`,
	})
	wantRoutes(t, g, "ANY /new")
	wantNoRoute(t, g, "ANY /old")
}
