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
// wasn't given: exactly one candidate found in root is used automatically
// (and announced, never silent); more than one or none means asking,
// never guessing. w is where the prompt/announcement is written (stderr
// in production, so stdout output stays clean for piping); r is where the
// answer is read from (stdin in production).
func resolveFeaturesPath(candidates []string, r io.Reader, w io.Writer, interactive bool) (string, error) {
	switch len(candidates) {
	case 1:
		fmt.Fprintf(w, "using plan file: %s\n", candidates[0])
		return candidates[0], nil
	case 0:
		if !interactive {
			return "", fmt.Errorf("no plan file found automatically, and no terminal to ask — pass --features explicitly")
		}
		fmt.Fprint(w, "no plan file found automatically. Enter the path to your plan/checklist file: ")
		return readLine(r)
	default:
		if !interactive {
			return "", fmt.Errorf("found more than one possible plan file (%s) — pass one explicitly with --features", strings.Join(candidates, ", "))
		}
		fmt.Fprintln(w, "found more than one possible plan file:")
		for i, c := range candidates {
			fmt.Fprintf(w, "  %d) %s\n", i+1, c)
		}
		fmt.Fprint(w, "which one is your plan? [1]: ")
		answer, err := readLine(r)
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

func readLine(r io.Reader) (string, error) {
	scanner := bufio.NewScanner(r)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return "", err
		}
		return "", fmt.Errorf("no input given")
	}
	return strings.TrimSpace(scanner.Text()), nil
}
