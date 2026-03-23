package fzf

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"golang.org/x/term"
)

// Available returns true if fzf is installed and stdin is a terminal.
func Available() bool {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return false
	}
	_, err := exec.LookPath("fzf")
	return err == nil
}

// Pick presents the given items in fzf and returns the selected item.
// header is shown at the top of the fzf prompt (e.g. "Select Deployment").
// If the user cancels (Ctrl-C / Esc), an error is returned.
func Pick(items []string, header string) (string, error) {
	if len(items) == 0 {
		return "", fmt.Errorf("no resources found")
	}

	cmd := exec.Command("fzf", "--ansi", "--no-preview", "--header", header)
	cmd.Stdin = strings.NewReader(strings.Join(items, "\n"))
	cmd.Stderr = os.Stderr

	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("fzf selection cancelled")
	}

	selected := strings.TrimSpace(out.String())
	if selected == "" {
		return "", fmt.Errorf("fzf selection cancelled")
	}
	return selected, nil
}
