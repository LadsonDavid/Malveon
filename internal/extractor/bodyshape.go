// Body-shape extraction for the contract check's real future-work slice:
// which fields does a frontend call's request body actually send, and
// which fields does the matching backend handler actually read.
// Deliberately JS/TS-only for v1 — Python/Python and Go both build
// request bodies in ways (kwargs, struct literals) this text-scanning
// approach can't resolve as cheaply, so their fields stay unresolved
// (nil) rather than guessed. Never partially resolve a field set: a
// spread (...x) or any non-literal shape means "can't be sure," so the
// whole set comes back unresolved instead of an undercounted one that
// could produce a false CONTRACT MISMATCH.
package extractor

import (
	"regexp"
	"strings"
)

var (
	jsBodyStringifyPattern = regexp.MustCompile(`\bbody\s*:\s*JSON\.stringify\s*\(`)
	jsBodyDirectPattern    = regexp.MustCompile(`\bbody\s*:\s*(\{)`)

	jsReqBodyDotPattern        = regexp.MustCompile(`\breq\.body\.(\w+)`)
	jsReqBodyBracketPattern    = regexp.MustCompile(`\breq\.body\[\s*['"](\w+)['"]\s*\]`)
	jsReqBodyDestructurePattern = regexp.MustCompile(`(?:const|let|var)\s*\{([^}]*)\}\s*=\s*req\.body\b`)
)

// objectLiteralKeys extracts the top-level key names from a raw object
// literal string like "{ amount: x, reason: y }" or "{ amount, reason }".
// resolved is false if raw isn't a plain object literal, or contains a
// spread that could be hiding fields this scan can't see.
func objectLiteralKeys(raw string) (keys []string, resolved bool) {
	raw = strings.TrimSpace(raw)
	if len(raw) < 2 || raw[0] != '{' || raw[len(raw)-1] != '}' {
		return nil, false
	}
	return topLevelKeys(raw[1 : len(raw)-1])
}

func topLevelKeys(body string) (keys []string, resolved bool) {
	keys = []string{}
	depth := 0
	var quote byte
	segStart := 0
	flush := func(end int) bool {
		seg := strings.TrimSpace(body[segStart:end])
		if seg == "" {
			return true
		}
		if strings.HasPrefix(seg, "...") {
			return false // spread - can't know what fields it hides
		}
		name := seg
		if idx := strings.IndexByte(seg, ':'); idx >= 0 {
			name = seg[:idx]
		}
		name = strings.Trim(strings.TrimSpace(name), `'"`+"`")
		if name != "" {
			keys = append(keys, name)
		}
		return true
	}
	i := 0
	for i < len(body) {
		c := body[i]
		if quote != 0 {
			if c == '\\' {
				i += 2
				continue
			}
			if c == quote {
				quote = 0
			}
			i++
			continue
		}
		switch c {
		case '\'', '"', '`':
			quote = c
		case '{', '[', '(':
			depth++
		case '}', ']', ')':
			depth--
		case ',':
			if depth == 0 {
				if !flush(i) {
					return nil, false
				}
				segStart = i + 1
			}
		}
		i++
	}
	if !flush(len(body)) {
		return nil, false
	}
	return keys, true
}

// readBalanced returns s[startIdx:] through the matching close bracket
// for whichever of '{'/'['/'(' sits at s[startIdx], inclusive, respecting
// quotes so a bracket inside a string doesn't throw the count off.
func readBalanced(s string, startIdx int) string {
	if startIdx < 0 || startIdx >= len(s) {
		return ""
	}
	open := s[startIdx]
	var closeB byte
	switch open {
	case '{':
		closeB = '}'
	case '[':
		closeB = ']'
	case '(':
		closeB = ')'
	default:
		return ""
	}
	depth := 0
	var quote byte
	i := startIdx
	for i < len(s) {
		c := s[i]
		if quote != 0 {
			if c == '\\' {
				i += 2
				continue
			}
			if c == quote {
				quote = 0
			}
			i++
			continue
		}
		switch c {
		case '\'', '"', '`':
			quote = c
		case open:
			depth++
		case closeB:
			depth--
			if depth == 0 {
				return s[startIdx : i+1]
			}
		}
		i++
	}
	return s[startIdx:]
}

// splitTopLevelArgs splits a call's raw argument-list text (as returned
// by readArgsList) into its top-level comma-separated arguments.
func splitTopLevelArgs(s string) []string {
	var args []string
	depth := 0
	var quote byte
	start := 0
	i := 0
	for i < len(s) {
		c := s[i]
		if quote != 0 {
			if c == '\\' {
				i += 2
				continue
			}
			if c == quote {
				quote = 0
			}
			i++
			continue
		}
		switch c {
		case '\'', '"', '`':
			quote = c
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
		case ',':
			if depth == 0 {
				args = append(args, strings.TrimSpace(s[start:i]))
				start = i + 1
			}
		}
		i++
	}
	args = append(args, strings.TrimSpace(s[start:]))
	return args
}

// jsSentFieldsFromFetchArgs looks for a body:-style request field set in
// a fetch()/axios() call's raw argument-list text. Handles body:
// JSON.stringify({...}), body: {...} directly (fetch-style), and a
// literal object passed as axios's second positional argument. Returns
// resolved=false if no recognizable body was found.
func jsSentFieldsFromCallArgs(argsList string) (keys []string, resolved bool) {
	if m := jsBodyStringifyPattern.FindStringIndex(argsList); m != nil {
		open := m[1] - 1
		raw := readFirstArg(argsList, open)
		return objectLiteralKeys(raw)
	}
	if m := jsBodyDirectPattern.FindStringSubmatchIndex(argsList); m != nil {
		braceIdx := m[2]
		raw := readBalanced(argsList, braceIdx)
		return objectLiteralKeys(raw)
	}
	// axios-style: second positional argument is the body object.
	parts := splitTopLevelArgs(argsList)
	if len(parts) >= 2 {
		return objectLiteralKeys(parts[1])
	}
	return nil, false
}

// jsReadFieldsFromHandler extracts field names read via req.body from a
// route handler's raw argument-list text (the registration call's full
// args, including the trailing handler function). Always resolved (even
// to an empty set) unless no handler body could be located at all —
// under-detecting an indirect read (req.body assigned to a variable
// first, a spread read) is the safe direction of error here: it can only
// cause a missed mismatch, never a false one.
func jsReadFieldsFromHandler(argsList string) (keys []string, resolved bool) {
	parts := splitTopLevelArgs(argsList)
	if len(parts) == 0 {
		return nil, false
	}
	handler := parts[len(parts)-1]
	braceIdx := strings.IndexByte(handler, '{')
	if braceIdx < 0 {
		return nil, false
	}
	body := readBalanced(handler, braceIdx)

	seen := map[string]bool{}
	fields := []string{}
	add := func(f string) {
		if f != "" && !seen[f] {
			seen[f] = true
			fields = append(fields, f)
		}
	}
	for _, m := range jsReqBodyDotPattern.FindAllStringSubmatch(body, -1) {
		add(m[1])
	}
	for _, m := range jsReqBodyBracketPattern.FindAllStringSubmatch(body, -1) {
		add(m[1])
	}
	for _, m := range jsReqBodyDestructurePattern.FindAllStringSubmatch(body, -1) {
		for _, part := range strings.Split(m[1], ",") {
			name := strings.TrimSpace(part)
			if idx := strings.Index(name, ":"); idx >= 0 {
				name = strings.TrimSpace(name[:idx])
			}
			add(name)
		}
	}
	return fields, true
}
