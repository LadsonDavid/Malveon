package extractor

import (
	"reflect"
	"testing"
)

func TestNextReadFields(t *testing.T) {
	cases := []struct {
		name, src string
		want      []string
		resolved  bool
	}{
		{"destructured json()", `export async function POST(request: Request) {
  const { title, company } = await request.json()
}`, []string{"title", "company"}, true},
		{"json() into a variable", `export async function POST(req) {
  const body = await req.json()
  const { entity_type } = body
  if (!body.entity_id) return
}`, []string{"entity_id", "entity_type"}, true},
		{"req.body destructured", `export async function POST(req) {
  const { title, company } = req.body
}`, []string{"title", "company"}, true},
		{"an inner fetch's json() is not the request", `export async function POST(req) {
  const { a } = await req.json()
  const r = await fetch("https://x.test"); const data = await r.json(); use(data.z)
}`, []string{"a"}, true},
		{"no request parameter: reads nothing", `export async function GET() { return Response.json({}) }`, []string{}, true},
		{"body passed on whole: unknown", `export async function POST(req) {
  const body = await req.json()
  await save(body)
}`, nil, false},
		{"rest element: unknown", `export async function POST(req) {
  const { a, ...rest } = await req.json()
}`, nil, false},
		{"fields read only with a fallback are optional (a real app's /api/submissions)", `export async function POST(request: Request) {
  const body = await request.json();
  if (!body.challenge_id || !body.wrong_decision) return
  await supabase.from("submissions").insert({
    challenge_id: body.challenge_id,
    github_repo_url: body.github_repo_url || null,
    tech_stack: body.tech_stack ?? [],
    wrong_decision: body.wrong_decision,
  })
}`, []string{"challenge_id", "wrong_decision"}, true},
		{"a destructured default is optional", `export async function POST(req) {
  const { title, page = 1 } = await req.json()
}`, []string{"title"}, true},
		{"formData: unknown", `export async function POST(req) {
  const f = await req.formData()
}`, nil, false},
	}
	for _, c := range cases {
		sites := scanNextRouteHandlers("app/api/x/route.ts", c.src)
		if len(sites) != 1 {
			t.Fatalf("%s: %d sites", c.name, len(sites))
		}
		got := sites[0].bodyFields
		if !c.resolved {
			if got != nil {
				t.Errorf("%s: fields %v, want unresolved (nil)", c.name, got)
			}
			continue
		}
		if !reflect.DeepEqual(sortCopy(got), sortCopy(c.want)) {
			t.Errorf("%s: fields %v, want %v", c.name, got, c.want)
		}
	}
}

func sortCopy(s []string) []string {
	out := append([]string{}, s...)
	for i := range out {
		for j := i + 1; j < len(out); j++ {
			if out[j] < out[i] {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}
