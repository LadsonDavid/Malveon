// Package skipdirs holds the one shared list of directories that are
// never real source: version control, installed dependencies, and
// build/cache output. Every package that walks the project tree
// (internal/extractor, internal/features, internal/watch,
// internal/checks/overlap, internal/checks/uioverlap) uses this same
// list, so adding a new entry fixes all of them at once.
//
// This package exists specifically because that wasn't true once: each
// of those 5 packages kept its own independent copy of the same base
// list. ".next" got added to internal/features's copy but not
// internal/extractor's, and the extractor spent real time walking
// Next.js's compiled build output as if it were source — citing
// bundled, minified JS chunks as "evidence" instead of the real code
// that produced them, and diluting the common-word frequency filter
// with thousands of generated nodes (2026-09-16, found via a real
// external test against a real Next.js project).
package skipdirs

// Names is the shared skip set. Framework build-output and language
// dependency/cache directories are grouped by the ecosystem that
// produces them, even though a project only ever has one.
var Names = map[string]bool{
	".git": true, "node_modules": true, "vendor": true,
	"dist": true, "build": true, ".malveon": true,

	// JS/TS framework build output
	".next": true, ".nuxt": true, ".svelte-kit": true, ".turbo": true,

	// Python bytecode cache / virtualenvs
	"__pycache__": true, ".venv": true, "venv": true,

	// test coverage reports, any language
	"coverage": true,
}
