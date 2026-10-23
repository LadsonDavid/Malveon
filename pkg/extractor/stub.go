package extractor

import (
	"regexp"
	"strings"
)

// A route whose handler only answers with an error is registered but not
// built: "POST /api/pay" that returns 501 "not implemented" is no payment
// feature (found by audit, 2026-10-10: it read CONNECTED). The first version
// called any short handler with a 500 in it a stub and flagged ordinary
// try/catch handlers; a stub is now defined structurally: every answer the
// handler can send is an error, and it does nothing else.
var (
	// where a response statement starts: res.status(..)/res.json(..), Response.json(..), new Response(..)
	responseStart = regexp.MustCompile(`\b(?:res|response|reply)\s*\.\s*(?:status|sendStatus|json|send|end|render|redirect|sendFile|type|set|header|write|code)\b|\b(?:Next)?Response\s*\.\s*(?:json|redirect)\b|\bnew\s+(?:Next)?Response\b`)
	// an error answer: status 500/501, or text saying it is not implemented
	errorStatus = regexp.MustCompile(`(?i)(?:\.status\(\s*|sendStatus\(\s*|\.code\(\s*|status\s*:\s*|statusCode\s*=\s*)50[01]\b`)
	stubText    = regexp.MustCompile("(?i)not[ _-]?implemented|NotImplementedError|unimplemented")
	throwErr    = regexp.MustCompile(`\bthrow\s+new\s+\w*Error\s*\(`)
	callOrAwait = regexp.MustCompile(`\bawait\b|[\w$\]\)]\s*\(`)
)

const stubMaxLines = 12

// isStubHandler: the handler text (a function or its body) sends only error
// answers and does nothing else.
func isStubHandler(handler string) bool {
	if nonEmptyLines(handler) > stubMaxLines {
		return false
	}
	body := handlerBodyOf(handler)
	rest := body
	responses, errors := 0, 0
	// cut each response statement out, remembering whether it was an error
	for {
		loc := responseStart.FindStringIndex(rest)
		if loc == nil {
			break
		}
		end := responseEnd(rest, loc[0])
		stmt := rest[loc[0]:end]
		responses++
		if errorStatus.MatchString(stmt) || stubText.MatchString(stmt) {
			errors++
		}
		rest = rest[:loc[0]] + " " + rest[end:]
	}
	if responses != errors {
		return false // it can answer with something that isn't an error
	}
	thrown := false
	if loc := throwErr.FindStringIndex(rest); loc != nil && stubText.MatchString(rest[loc[0]:]) {
		end := loc[1] - 1 + len(readBalanced(rest, loc[1]-1))
		rest = rest[:loc[0]] + " " + rest[min(end, len(rest)):]
		thrown = true
	}
	if responses == 0 && !thrown {
		return false
	}
	// anything else the handler calls or awaits is work
	return !callOrAwait.MatchString(rest)
}

// responseEnd is the index just past a response statement that starts at i:
// res.status(500).json({...}) — a chain of .name(...) calls.
func responseEnd(s string, i int) int {
	k := i
	if strings.HasPrefix(s[k:], "new ") {
		k += 4
	}
	// the leading identifier (res, Response, NextResponse)
	for k < len(s) && (isIdent(s[k])) {
		k++
	}
	for k < len(s) {
		j := k
		for j < len(s) && (s[j] == ' ' || s[j] == '\t' || s[j] == '\n' || s[j] == '\r') {
			j++
		}
		switch {
		case j < len(s) && s[j] == '.':
			j++
			for j < len(s) && (s[j] == ' ' || s[j] == '\t') {
				j++
			}
			for j < len(s) && isIdent(s[j]) {
				j++
			}
			k = j
		case j < len(s) && s[j] == '(':
			group := readBalanced(s, j)
			if group == "" {
				return len(s)
			}
			k = j + len(group)
		default:
			return k
		}
	}
	return k
}

func isIdent(c byte) bool {
	return c == '_' || c == '$' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

// handlerBodyOf drops the parameter list: what follows "=>" or the first "{".
func handlerBodyOf(h string) string {
	if i := strings.Index(h, "=>"); i >= 0 {
		return h[i+2:]
	}
	if i := strings.IndexByte(h, '{'); i >= 0 {
		return h[i:]
	}
	return h
}

func nonEmptyLines(s string) int {
	n := 0
	for _, l := range strings.Split(s, "\n") {
		if strings.TrimSpace(l) != "" {
			n++
		}
	}
	return n
}

// handlerBodyAt is the text of the function whose parameter list opens at
// parenIdx: the {...} block after it (the whole rest of the line for an
// expression-bodied arrow function).
func handlerBodyAt(src string, parenIdx int) string {
	params := readBalanced(src, parenIdx)
	if params == "" {
		return ""
	}
	rest := src[parenIdx+len(params):]
	brace := strings.IndexByte(rest, '{')
	if brace < 0 || brace > 80 {
		return ""
	}
	return readBalanced(src, parenIdx+len(params)+brace)
}
