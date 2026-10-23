package features

import "strings"

// routeNeighbor: words that, next to a bare "/word", say it is a route or page and not
// a slash command or skill ("load /software", "run /mcp").
var routeNeighbor = map[string]bool{
	"route": true, "routes": true, "endpoint": true, "endpoints": true, "url": true, "urls": true,
	"page": true, "pages": true, "path": true, "link": true, "redirect": true, "redirects": true, "api": true,
}

// LooksLikeRoute reports whether a URL found in free text (a prompt, or what an agent
// wrote) is a route and not a slash command. A URL of two or more segments, or one
// with its HTTP method, always is. A one-segment "/word" is only when the text calls
// it a route or page next to it, or (allowBacktick) it sits in backticks: a bare
// "/software" or "/gsd:plan-phase" is a skill, and "run `/mcp`" is a command, which
// is why an agent's prose passes allowBacktick false. A user typing `/pricing` in
// backticks to name a page passes true.
func LooksLikeRoute(text string, u PlanURL, allowBacktick bool) bool {
	if strings.Count(u.Path, "/") > 1 || u.Method != "" {
		return true
	}
	i := strings.Index(text, u.Path)
	if i < 0 {
		return true
	}
	if allowBacktick && i > 0 && text[i-1] == '`' {
		return true
	}
	return routeNeighbor[wordBefore(text[:i])] || routeNeighbor[wordAfter(text[i+len(u.Path):])]
}

func wordBefore(s string) string {
	s = strings.TrimRight(s, " \t\r\n`'\"([")
	i := len(s)
	for i > 0 && isLetter(s[i-1]) {
		i--
	}
	return strings.ToLower(s[i:])
}

func wordAfter(s string) string {
	s = strings.TrimLeft(s, " \t\r\n`'\")],.;:")
	i := 0
	for i < len(s) && isLetter(s[i]) {
		i++
	}
	return strings.ToLower(s[:i])
}

func isLetter(b byte) bool { return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' }
