package extractor

import (
	"regexp"
	"strings"
)

const responseIdent = `[A-Za-z_$][\w$]*`

var (
	fetchBinding       = regexp.MustCompile(`^[ \t]*const[ \t]+(` + responseIdent + `)[ \t]*=[ \t]*await[ \t]+$`)
	responseBinding    = regexp.MustCompile(`^[ \t]*(?:;|\r?\n)\s*const\s+(` + responseIdent + `|\{[^{}]*\})\s*=\s*await\s+(` + responseIdent + `)\.json\(\)[ \t]*`)
	responseRead       = regexp.MustCompile(`^\s*return[ \t]+(` + responseIdent + `)\.(` + responseIdent + `)[ \t]*(?:;|\s*\})`)
	jsonStatementEnd   = regexp.MustCompile(`^(?:;|\r?\n\s*(?:return\b|\})|\})`)
	plainResponseField = regexp.MustCompile(`^` + responseIdent + `$`)
	shadowedFetch      = regexp.MustCompile(`\b(?:const|let|var|function|class)\s+fetch\b`)
	shadowedResponse   = regexp.MustCompile(`\b(?:const|let|var|function|class)\s+(?:Response|NextResponse)\b`)
	responseParam      = regexp.MustCompile(`\b(?:Response|NextResponse)\b`)
	jsonReturn         = regexp.MustCompile(`^\{\s*return[ \t]+(Response|NextResponse)\.json\(\s*\{`)
	jsonResponseEnd    = regexp.MustCompile(`^\s*\)\s*;?\s*\}$`)
	jsonResponsePair   = regexp.MustCompile(`^\s*(` + responseIdent + `|'` + responseIdent + `'|"` + responseIdent + `")\s*:\s*(?:'[^'\\\r\n]*'|"[^"\\\r\n]*"|-?\d+(?:\.\d+)?|` + responseIdent + `(?:\.` + responseIdent + `)*)\s*`)
)

// Track only adjacent statements so later assignments and nested scopes cannot
// turn an unrelated property read into a dependency on this fetch.
func fetchResponseFields(src string, fetch, open int) []string {
	// The supported assignment begins on this line. Bound the lookback so
	// repeated fetches in a minified one-line bundle do not rescan its prefix.
	start := max(0, fetch-256)
	if newline := strings.LastIndexByte(src[start:fetch], '\n'); newline >= 0 {
		start += newline + 1
	} else if start > 0 {
		return nil
	}
	binding := fetchBinding.FindStringSubmatch(src[start:fetch])
	if binding == nil {
		return nil
	}
	args := readBalanced(src, open)
	if len(args) < 2 {
		return nil
	}
	url := strings.TrimSpace(args[1 : len(args)-1])
	if len(url) < 4 || (url[0] != '\'' && url[0] != '"') || url[len(url)-1] != url[0] || !strings.HasPrefix(url[1:], "/") || strings.HasPrefix(url[1:], "//") || strings.ContainsAny(url[1:len(url)-1], "'\"\\\r\n") {
		return nil
	}
	after := src[open+len(args):]
	decoded := responseBinding.FindStringSubmatch(after)
	if decoded == nil || decoded[2] != binding[1] {
		return nil
	}
	after = after[len(decoded[0]):]
	if !jsonStatementEnd.MatchString(after) {
		return nil
	}
	if names, ok := strings.CutPrefix(decoded[1], "{"); ok {
		var fields []string
		for _, field := range strings.Split(strings.TrimSuffix(names, "}"), ",") {
			field = strings.TrimSpace(field)
			if !plainResponseField.MatchString(field) {
				return nil
			}
			fields = append(fields, field)
		}
		return fields
	}
	read := responseRead.FindStringSubmatch(strings.TrimPrefix(after, ";"))
	if read == nil || read[1] != decoded[1] {
		return nil
	}
	return []string{read[2]}
}

// Require a single default-status JSON return with a closed object shape.
// Spreads, branches, helpers, computed keys and custom serialization abstain.
func nextResponseFields(src string, paren int) []string {
	params := readBalanced(src, paren)
	if params == "" || responseParam.MatchString(params) || shadowedResponse.MatchString(src) {
		return nil
	}
	remaining := strings.TrimLeft(src[paren+len(params):], " \t\r\n")
	body := maskComments(readBalanced(remaining, 0), jsSyntax)
	match := jsonReturn.FindStringSubmatch(body)
	if match == nil {
		return nil
	}
	module := importTable(src)[match[1]]
	if (match[1] == "Response" && module != "") || (match[1] == "NextResponse" && module != "next/server") {
		return nil
	}
	start := len(match[0]) - 1
	object := readBalanced(body, start)
	if object == "" || !jsonResponseEnd.MatchString(body[start+len(object):]) {
		return nil
	}
	fields := []string{}
	for rest := strings.TrimSpace(object[1 : len(object)-1]); rest != ""; {
		entry := jsonResponsePair.FindStringSubmatch(rest)
		if entry == nil {
			return nil
		}
		key := strings.Trim(entry[1], `'"`)
		if key == "toJSON" || key == "__proto__" {
			return nil
		}
		fields = append(fields, key)
		rest = rest[len(entry[0]):]
		if rest != "" {
			var ok bool
			rest, ok = strings.CutPrefix(rest, ",")
			if !ok {
				return nil
			}
			rest = strings.TrimSpace(rest)
		}
	}
	return fields
}
