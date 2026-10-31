package extractor

import (
	"regexp"
	"strings"
)

const litPat = "(?:'[^'\\n]*'|\"[^\"\\n]*\"|`[^`]*`)"

// ternaryConst is `const url = cond ? <literal> : <literal>`, the way a page picks which address to call.
var ternaryConst = regexp.MustCompile("(?:const|let|var)\\s+(\\w+)\\s*=\\s*[^;?=]{1,200}?\\?\\s*(" + litPat + ")\\s*:\\s*(" + litPat + ")")

// stringAlts maps a variable to the literals it can hold when it is chosen by a ternary of two
// string literals (the raw text, quotes included).
func stringAlts(src string, cm codeMap) map[string][]string {
	var out map[string][]string
	for _, m := range ternaryConst.FindAllStringSubmatchIndex(src, -1) {
		if !cm.codeAt(src, m[0]) {
			continue
		}
		if out == nil {
			out = map[string][]string{}
		}
		out[src[m[2]:m[3]]] = []string{src[m[4]:m[5]], src[m[6]:m[7]]}
	}
	return out
}

var (
	plainIdentArg = regexp.MustCompile(`^[A-Za-z_$][\w$]*$`)
	identInTpl    = regexp.MustCompile("^`\\$\\{\\s*([A-Za-z_$][\\w$]*)\\s*\\}(.*)`$")
)

// urlAlternatives turns the first argument of a request call into the literal(s) it can be:
// the argument itself, or each literal a ternary variable holds (`fetch(url)` or
// fetch(`${url}?${params}`)). A query string is cut off: the address is what is compared.
func urlAlternatives(arg string, alts map[string][]string) []string {
	if len(alts) == 0 {
		return []string{arg}
	}
	var lits []string
	switch {
	case plainIdentArg.MatchString(arg) && alts[arg] != nil:
		lits = alts[arg]
	default:
		m := identInTpl.FindStringSubmatch(arg)
		if m == nil || alts[m[1]] == nil || !(m[2] == "" || strings.HasPrefix(m[2], "?")) {
			return []string{arg}
		}
		lits = alts[m[1]]
	}
	out := make([]string, 0, len(lits))
	for _, l := range lits {
		out = append(out, trimQuery(l))
	}
	return out
}

// trimQuery cuts a quoted literal at its "?", keeping the quote style: "/a?b=${c}" becomes "/a".
func trimQuery(lit string) string {
	if len(lit) < 2 {
		return lit
	}
	if i := strings.IndexByte(lit, '?'); i > 0 {
		return lit[:i] + lit[:1]
	}
	return lit
}

var absoluteURLArg = regexp.MustCompile("^[`'\"]https?://[^/$`'\"]")

// isAbsoluteURLArg: the argument starts with a literal http(s) host ("https://api.cloudinary.com/...${x}"),
// so the call goes to another service, not to this project's server.
func isAbsoluteURLArg(arg string) bool {
	return absoluteURLArg.MatchString(arg)
}

var (
	// BASE + '/path', URL+"/path", config.url + `/path/${id}`
	concatTail = regexp.MustCompile(`^[A-Za-z_$][\w$.?]*(?:\([^()]*\))?\s*\+\s*(['"` + "`" + `]/[^'"` + "`" + `]*['"` + "`" + `])\s*$`)
	// `${API_URL}/path/${id}`: a base held in a variable, then the address
	templateTail = regexp.MustCompile("^`" + `\$\{[^}]+\}(/[^` + "`" + `]*)` + "`$")
)

// tailOfAddress reads a request address written as "base variable + literal path". Only the tail is
// known; the base (a host, or a prefix such as /api) was written somewhere else, so the call is
// matched to a route that ENDS with this path, and is never judged on its own (see graph.Node.Tail).
func tailOfAddress(arg string) (string, bool) {
	arg = strings.TrimSpace(arg)
	if m := concatTail.FindStringSubmatch(arg); m != nil {
		return m[1], true
	}
	if m := templateTail.FindStringSubmatch(arg); m != nil && len(m[1]) > 1 {
		return "`" + m[1] + "`", true
	}
	return "", false
}

func tailArg(a string) string {
	if t, ok := tailOfAddress(a); ok {
		return t
	}
	return a
}

func isTail(a string) bool {
	_, ok := tailOfAddress(a)
	return ok
}

var verbLiteral = regexp.MustCompile(`(?i)^['"` + "`" + `](GET|POST|PUT|PATCH|DELETE)['"` + "`" + `]$`)

// verbArg is the HTTP method written as a string literal among a call's leading arguments, or "".
func verbArg(args []string) string {
	for _, a := range args {
		if m := verbLiteral.FindStringSubmatch(strings.TrimSpace(a)); m != nil {
			return strings.ToUpper(m[1])
		}
	}
	return ""
}
