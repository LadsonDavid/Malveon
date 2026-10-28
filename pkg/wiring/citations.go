package wiring

import "regexp"

// Keep the original requirement in Result; only exclude an explicitly
// attributed parenthetical citation from local anchor lookup.
var citationParenthesis = regexp.MustCompile("(?i)\\((?:idea|inspiration|inspired|reference)(?: from| by|:)[^)]*\\)")
