package wiring

import (
	"regexp"
	"strings"
)

// fileRemovalTargets recognizes only direct file-deletion objects, not a file
// mentioned as the context for removing a paragraph, import or navigation item.
func fileRemovalTargets(line string, anchors []anchor) map[string]bool {
	var refs []string
	for _, a := range anchors {
		if a.kind == fileAnchor {
			refs = append(refs, regexp.QuoteMeta(a.text))
		}
	}
	if len(refs) == 0 {
		return nil
	}
	file := "[\x60\"']?(?:" + strings.Join(refs, "|") + ")[\x60\"']?"
	list := file + `(?:(?:\s*,\s*(?:and\s+)?|\s+and\s+)` + file + `)*`
	verb := `(?:remove|delete|drop|get rid of)`
	end := `\s*(?:\([^)]*\))?[.!]?\s*$`
	patterns := []string{
		`(?i)^[^A-Za-z]*` + verb + `\s+(?:(?:the\s+)?files?\s+|the\s+)?(` + list + `)` + end,
		`(?i)^\s*(` + list + `)\s*[—–:]\s*` + verb + end,
	}
	for _, pattern := range patterns {
		m := regexp.MustCompile(pattern).FindStringSubmatch(line)
		if m == nil {
			continue
		}
		out := map[string]bool{}
		for _, token := range regexp.MustCompile(file).FindAllString(m[1], -1) {
			out[strings.Trim(token, "`\"'")] = true
		}
		return out
	}
	return nil
}
