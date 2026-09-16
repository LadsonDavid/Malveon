package overlap

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var skipDirs = map[string]bool{
	".git": true, "node_modules": true, "vendor": true,
	"dist": true, "build": true, ".malveon": true,
}

var sourceExts = map[string]bool{
	".js": true, ".jsx": true, ".ts": true, ".tsx": true, ".mjs": true, ".cjs": true,
	".py": true, ".go": true,
}

// referencedElsewhere reports whether funcName is mentioned anywhere in
// root's source tree beyond the single mention that is its own
// declaration. A total occurrence count of exactly one means nothing
// else in the codebase ever names this function — a real, structural
// signal that whatever route it registers likely never runs. Any scan
// failure, or an unknown (empty) function name, returns true (assume
// referenced) — this signal must never turn an uncertainty into a false
// "unreachable" claim.
func referencedElsewhere(root, funcName string) bool {
	if funcName == "" {
		return true
	}
	pattern := regexp.MustCompile(`\b` + regexp.QuoteMeta(funcName) + `\b`)
	total := 0
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !sourceExts[strings.ToLower(filepath.Ext(path))] {
			return nil
		}
		raw, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		total += len(pattern.FindAllStringIndex(string(raw), -1))
		return nil
	})
	if err != nil {
		return true
	}
	return total > 1
}
