package extractor

import "testing"

// kiroshi-monorepo: Rust mock servers in tests.rs and tests/ were read as production routes and
// "collided" with each other.
func TestRustTestFilesAreTestFiles(t *testing.T) {
	for _, f := range []string{"apps/app/src-tauri/src/devices/tests.rs", "apps/app/src-tauri/tests/account.rs"} {
		if !IsTestFile(f) {
			t.Errorf("%s should be a test file", f)
		}
	}
	for _, f := range []string{"src/contests.rs", "src/attests/x.ts"} {
		if IsTestFile(f) {
			t.Errorf("%s is not a test file", f)
		}
	}
}
