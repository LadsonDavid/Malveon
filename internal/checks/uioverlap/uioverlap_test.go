package uioverlap

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	full := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRun(t *testing.T) {
	dir := t.TempDir()

	// Risk: absolute element, no positioning context anywhere in the file.
	writeFile(t, dir, "Escapee.tsx", `
export function Escapee() {
  return <div className="absolute top-0 right-0">Badge</div>;
}
`)

	// Safe: absolute element, but the file also has a relative container.
	writeFile(t, dir, "Safe.tsx", `
export function Safe() {
  return (
    <div className="relative">
      <span className="absolute top-0 right-0">Badge</span>
    </div>
  );
}
`)

	// Not flagged: no absolute/fixed classes at all.
	writeFile(t, dir, "Plain.tsx", `
export function Plain() {
  return <div className="flex items-center">Hello</div>;
}
`)

	// Not flagged: dynamic className, never guessed at.
	writeFile(t, dir, "Dynamic.tsx", `
export function Dynamic({cls}) {
  return <div className={cls}>Hello</div>;
}
`)

	findings, err := Run(dir)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected exactly 1 finding, got %d: %+v", len(findings), findings)
	}
	if findings[0].File != "Escapee.tsx" {
		t.Errorf("expected the finding in Escapee.tsx, got %q", findings[0].File)
	}
}

func TestRunFixedAlsoFlagged(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "Header.jsx", `
function Header() {
  return <header className="fixed w-full">Nav</header>;
}
`)
	findings, err := Run(dir)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for a fixed element with no context, got %d: %+v", len(findings), findings)
	}
}
