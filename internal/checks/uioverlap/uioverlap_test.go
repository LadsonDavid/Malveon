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

// TestRunFixedNeverFlagged covers a real false positive found by reading
// a real project (admin/layout.tsx): a "fixed" element always positions
// against the viewport, so unlike "absolute" it never needs a
// relative/sticky ancestor to avoid escaping.
func TestRunFixedNeverFlagged(t *testing.T) {
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
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings (fixed never needs a positioning-context ancestor), got %d: %+v", len(findings), findings)
	}
}

// TestRunAbsoluteInsideFixedHasContext covers the other half of the same
// real false positive (Modal.tsx): a "fixed inset-0" wrapper around an
// "absolute inset-0" backdrop is a standard modal pattern. CSS gives a
// fixed element its own containing block, so the nested absolute child
// is correctly positioned, not escaping.
func TestRunAbsoluteInsideFixedHasContext(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "Modal.tsx", `
export function Modal() {
  return (
    <div className="fixed inset-0 z-50">
      <div className="absolute inset-0 bg-black/80" />
    </div>
  );
}
`)
	findings, err := Run(dir)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings (fixed ancestor is valid context for a nested absolute), got %d: %+v", len(findings), findings)
	}
}

// TestRunAbsoluteStillFlaggedWithoutFixedOrRelative is regression
// protection: removing "fixed" from escapingClasses must not weaken the
// real absolute-with-no-context case this check still needs to catch.
func TestRunAbsoluteStillFlaggedWithoutFixedOrRelative(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "Badge.tsx", `
export function Badge() {
  return <span className="absolute top-0 right-0">3</span>;
}
`)
	findings, err := Run(dir)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding (absolute with no relative/sticky/fixed anywhere in file), got %d: %+v", len(findings), findings)
	}
}

// TestRunPlainCSS covers the real gap: Tailwind-only coverage missed
// plain stylesheets entirely. Same risk shape, just via an actual
// position: declaration instead of a utility class name.
func TestRunPlainCSS(t *testing.T) {
	dir := t.TempDir()

	// Risk: .badge escapes, no relative/sticky anywhere in the file.
	writeFile(t, dir, "badge.css", `
.badge {
  position: absolute;
  top: 0;
  right: 0;
}
`)

	// Safe: .tooltip escapes, but .panel in the same file is relative.
	writeFile(t, dir, "panel.css", `
.panel {
  position: relative;
}
.tooltip {
  position: fixed;
}
`)

	// Not flagged: no position declarations that matter (static is the default).
	writeFile(t, dir, "layout.css", `
.row {
  display: flex;
  position: static;
}
`)

	findings, err := Run(dir)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected exactly 1 finding, got %d: %+v", len(findings), findings)
	}
	if findings[0].File != "badge.css" {
		t.Errorf("expected the finding in badge.css, got %q", findings[0].File)
	}
}

// TestRunPlainCSSNestedContextNotMissed proves the reason context
// detection scans the whole file as flat text instead of brace-pairing
// each declaration with its selector: a parent selector's own `relative`
// must still suppress a nested child's `absolute`, even though a naive
// brace-matcher would fail to pair them due to the nesting.
func TestRunPlainCSSNestedContextNotMissed(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "card.scss", `
.card {
  position: relative;
  .badge {
    position: absolute;
  }
}
`)
	findings, err := Run(dir)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings (.card's relative should suppress .badge's absolute despite nesting), got %d: %+v", len(findings), findings)
	}
}

// TestRunPlainCSSFixedNeverFlagged is the plain-CSS equivalent of
// TestRunFixedNeverFlagged: position: fixed always resolves against the
// viewport, so it's never flagged even with no relative/sticky anywhere.
func TestRunPlainCSSFixedNeverFlagged(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "navbar.css", `
.navbar {
  position: fixed;
  top: 0;
}
`)
	findings, err := Run(dir)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings (position: fixed never needs a positioning-context ancestor), got %d: %+v", len(findings), findings)
	}
}

// TestRunPlainCSSAbsoluteWithFixedContext is the plain-CSS equivalent of
// TestRunAbsoluteInsideFixedHasContext: position: fixed anywhere in the
// file counts as valid context for a position: absolute declaration.
func TestRunPlainCSSAbsoluteWithFixedContext(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "modal.css", `
.overlay {
  position: fixed;
  inset: 0;
}
.backdrop {
  position: absolute;
  inset: 0;
}
`)
	findings, err := Run(dir)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings (position: fixed elsewhere in the file is valid context), got %d: %+v", len(findings), findings)
	}
}
