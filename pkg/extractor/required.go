package extractor

import (
	"regexp"
	"strings"
)

// guardPattern is an `if (!a || !b.c)` condition: every term negated and joined by ||, the usual
// "turn the request away when a field is missing" check.
var guardPattern = regexp.MustCompile(`\bif\s*\(\s*((?:!\s*[\w$.?]+\s*\|\|\s*)*!\s*[\w$.?]+)\s*\)`)

// requiredFieldsIn names which of the fields a handler reads it insists on: those that appear
// negated in a guard (`if (!to || !subject) return 400`). A field the handler reads but never
// guards is optional (`html` in a mail route that sends plain text without it). It returns nil
// when no guard names any read field, meaning "not known": callers then treat every field as
// required, as before.
func requiredFieldsIn(handler string, read []string) []string {
	if len(read) == 0 {
		return nil
	}
	isRead := map[string]bool{}
	for _, f := range read {
		isRead[f] = true
	}
	var out []string
	seen := map[string]bool{}
	for _, m := range guardPattern.FindAllStringSubmatch(handler, -1) {
		for _, term := range strings.Split(m[1], "||") {
			term = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(term), "!"))
			term = strings.ReplaceAll(term, "?.", ".")
			if i := strings.LastIndexByte(term, '.'); i >= 0 {
				term = term[i+1:]
			}
			if isRead[term] && !seen[term] {
				seen[term] = true
				out = append(out, term)
			}
		}
	}
	return out
}

func lastArg(args []string) string {
	if len(args) == 0 {
		return ""
	}
	return args[len(args)-1]
}
