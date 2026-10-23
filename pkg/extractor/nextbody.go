package extractor

import (
	"regexp"
	"strings"
)

// Request fields a Next.js App Router handler reads, for the contract
// check's field comparison (2026-09-26: a demo run showed fields were
// never compared for any Next.js route — only Express registrations were
// read). Tied to the handler's own request parameter, so an inner
// `await res.json()` from a fetch inside the handler is never read as the
// request body.

var firstParamPattern = regexp.MustCompile(`^\(\s*([A-Za-z_$][\w$]*)`)

// nextReadFields reads the handler whose `export ... POST(` match ends at
// parenIdx+1. resolved is false whenever the body is read in a way this
// can't follow (passed whole to a function, spread, formData) — the
// comparison is then skipped, never guessed.
func nextReadFields(src string, parenIdx int) (fields []string, resolved bool) {
	params := readBalanced(src, parenIdx)
	if params == "" {
		return nil, false
	}
	braceIdx := strings.IndexByte(src[parenIdx+len(params):], '{')
	if braceIdx < 0 {
		return nil, false
	}
	body := readBalanced(src, parenIdx+len(params)+braceIdx)
	m := firstParamPattern.FindStringSubmatch(params)
	if m == nil {
		return []string{}, true // no request parameter: the handler can't read a body
	}
	req := regexp.QuoteMeta(m[1])

	// A field read only with a fallback (`body.x || null`, `body.x ?? []`,
	// `{ x = 1 }`) is optional: a call that leaves it out is fine. Only
	// fields read at least once without one are required.
	var order []string
	required := map[string]bool{}
	mark := func(name string, optional bool) {
		if name == "" {
			return
		}
		if _, ok := required[name]; !ok {
			order = append(order, name)
			required[name] = false
		}
		if !optional {
			required[name] = true
		}
	}
	add := func(list string) {
		for _, part := range strings.Split(list, ",") {
			name := strings.TrimSpace(part)
			optional := strings.Contains(name, "=")
			if i := strings.IndexAny(name, ":="); i >= 0 {
				name = strings.TrimSpace(name[:i])
			}
			if strings.HasPrefix(name, "...") {
				resolved = false // a rest element takes every other field: unknown
				continue
			}
			mark(name, optional)
		}
	}
	resolved = true
	uses := regexp.MustCompile(`\b` + req + `\.(?:json|formData|text|body)\b`)
	known := 0
	// const { a, b } = await req.json()   ·   const { a } = req.body
	for _, d := range regexp.MustCompile(`(?:const|let|var)\s*\{([^}]*)\}\s*=\s*(?:await\s+`+req+`\.json\(\s*\)|`+req+`\.body\b)`).FindAllStringSubmatch(body, -1) {
		add(d[1])
		known++
	}
	// req.body.a   ·   req.body['a']
	for _, d := range regexp.MustCompile(`\b`+req+`\.body(?:\.(\w+)|\[\s*['"](\w+)['"]\s*\])(\s*(?:\|\||\?\?))?`).FindAllStringSubmatch(body, -1) {
		mark(d[1]+d[2], d[3] != "")
		known++
	}
	// const body = await req.json(), then body.a / const { a } = body
	for _, v := range regexp.MustCompile(`(?:const|let|var)\s+([A-Za-z_$][\w$]*)\s*(?::[^=]+)?=\s*await\s+`+req+`\.json\(\s*\)`).FindAllStringSubmatch(body, -1) {
		known++
		name := regexp.QuoteMeta(v[1])
		all := len(regexp.MustCompile(`\b`+name+`\b`).FindAllStringIndex(body, -1)) - 1 // minus its own declaration
		read := 0
		for _, d := range regexp.MustCompile(`\b`+name+`\.(\w+)(\s*(?:\|\||\?\?))?`).FindAllStringSubmatch(body, -1) {
			mark(d[1], d[2] != "")
			read++
		}
		for _, d := range regexp.MustCompile(`(?:const|let|var)\s*\{([^}]*)\}\s*=\s*`+name+`\b`).FindAllStringSubmatch(body, -1) {
			add(d[1])
			read++
		}
		if read < all {
			resolved = false // passed on whole, spread, or returned: fields unknown
		}
	}
	if known < len(uses.FindAllStringIndex(body, -1)) {
		resolved = false // the request body is read some other way
	}
	fields = []string{}
	for _, f := range order {
		if required[f] {
			fields = append(fields, f)
		}
	}
	return fields, resolved
}
