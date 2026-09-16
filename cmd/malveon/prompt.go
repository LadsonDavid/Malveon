package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// isInteractive reports whether stdin is a real terminal — used to decide
// whether asking the user a question is even possible, versus a script or
// CI pipe where a prompt would just hang forever.
func isInteractive() bool {
	stat, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (stat.Mode() & os.ModeCharDevice) != 0
}

// resolveFeaturesPath decides which plan file to use when --features
// wasn't given. No assumptions, ever, confirmed 2026-09-16 after a real
// external test run silently trusted a wrong single candidate: every
// candidate this function returns — whether found by a deliberate name
// match or only by a weaker content guess, whether there's one or many —
// gets shown to the user and requires an explicit answer before it's
// used. The only way to skip being asked is to pass --features directly.
// w is where the prompt/announcement is written (stderr in production,
// so stdout output stays clean for piping); r is where the answer is
// read from (stdin in production).
func resolveFeaturesPath(candidates []string, fromContentScan bool, r io.Reader, w io.Writer, interactive bool) (string, error) {
	// One scanner for the whole call: the "no" branch below reads a
	// confirmation answer and then, on rejection, a follow-up path — two
	// reads from the same underlying stream. A fresh bufio.Scanner per
	// read loses whatever the first one had already buffered, so every
	// read in this function must go through this one shared scanner.
	scanner := bufio.NewScanner(r)

	switch len(candidates) {
	case 1:
		label := "by name"
		if fromContentScan {
			label = "by content, not by name"
		}
		if !interactive {
			return "", fmt.Errorf("found a possible plan file (%s): %s — no terminal to confirm it, pass --features explicitly", label, candidates[0])
		}
		fmt.Fprintf(w, "found a possible plan file (%s): %s\n", label, candidates[0])
		fmt.Fprint(w, "use it? [Y/n]: ")
		answer, err := readLine(scanner)
		if err != nil {
			return "", err
		}
		switch strings.ToLower(answer) {
		case "", "y", "yes":
			return candidates[0], nil
		}
		fmt.Fprint(w, "Enter the path to your plan/checklist file: ")
		return readLine(scanner)
	case 0:
		if !interactive {
			return "", fmt.Errorf("no plan file found automatically, and no terminal to ask — pass --features explicitly")
		}
		fmt.Fprint(w, "no plan file found automatically. Enter the path to your plan/checklist file: ")
		return readLine(scanner)
	default:
		if !interactive {
			return "", fmt.Errorf("found more than one possible plan file (%s) — pass one explicitly with --features", strings.Join(candidates, ", "))
		}
		fmt.Fprintln(w, "found more than one possible plan file:")
		for i, c := range candidates {
			fmt.Fprintf(w, "  %d) %s\n", i+1, c)
		}
		fmt.Fprint(w, "which one is your plan? [1]: ")
		answer, err := readLine(scanner)
		if err != nil {
			return "", err
		}
		if answer == "" {
			return candidates[0], nil
		}
		n, convErr := strconv.Atoi(answer)
		if convErr != nil || n < 1 || n > len(candidates) {
			return "", fmt.Errorf("%q isn't one of the listed options", answer)
		}
		return candidates[n-1], nil
	}
}

func readLine(scanner *bufio.Scanner) (string, error) {
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return "", err
		}
		return "", fmt.Errorf("no input given")
	}
	return strings.TrimSpace(scanner.Text()), nil
}
