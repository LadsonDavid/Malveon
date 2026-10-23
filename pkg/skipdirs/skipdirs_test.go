package skipdirs

import "testing"

// Folders that hold other people's code or generated output must never be read
// as the project's own: a downloaded VS Code build under .vscode-test made the
// extractor read tens of megabytes of minified JavaScript (found 2026-10-19 on
// the Malveon monorepo: minutes to check, and routes from VS Code itself
// listed as "evidence").
func TestToolFoldersAreSkipped(t *testing.T) {
	for _, name := range []string{
		// downloaded editor builds used by extension tests
		".vscode-test", ".vscode-test-web",
		// tool caches and build output
		".cache", ".parcel-cache", ".angular", ".expo", ".vercel", ".output", ".nx", ".wrangler", ".sst", ".serverless", ".aws-sam",
		// package-manager stores
		".yarn", ".pnpm-store",
		// other languages' caches and downloads
		".gradle", ".terraform", ".tox", ".mypy_cache", ".pytest_cache", ".ruff_cache",
	} {
		if !Names[name] {
			t.Errorf("%s holds generated or downloaded code but is not skipped", name)
		}
	}
}

// The list matches a folder's name anywhere in the tree, so a name a real app
// can use for its own code or a page route (target, out, public, tests) must
// never be on it: skipping it would hide real code.
func TestRealFolderNamesAreNotSkipped(t *testing.T) {
	for _, name := range []string{
		"src", "app", "lib", "api", "components", "pages", "public", "docs", "scripts",
		"test", "tests", "e2e", "target", "out", "bin", "obj", "packages", "apps", "server", "client",
		".github", ".vscode", ".claude", ".cursor", ".agents", ".husky",
	} {
		if Names[name] {
			t.Errorf("%s can be a real source folder but is skipped", name)
		}
	}
}
