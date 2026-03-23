package cmd

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

// renderFunc writes tree output to the provided writer. In watch mode the
// writer is a buffer so the screen can be overwritten atomically without
// flicker.
type renderFunc func(w io.Writer) error

// runWatch calls fn in a loop at WatchInterval, overwriting the previous
// output in-place (no screen clear). If Watch is false it calls fn once
// writing directly to os.Stdout.
func runWatch(fn renderFunc) error {
	if !Watch {
		return fn(os.Stdout)
	}

	// Clear the screen once before the first frame.
	fmt.Fprint(os.Stdout, "\033[H\033[2J")

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sig)

	ticker := time.NewTicker(WatchInterval)
	defer ticker.Stop()

	var prevLines int

	// Render immediately on first call.
	n, err := renderFrame(fn, 0)
	if err != nil {
		return err
	}
	prevLines = n

	for {
		select {
		case <-sig:
			fmt.Fprintln(os.Stdout)
			return nil
		case <-ticker.C:
			n, err := renderFrame(fn, prevLines)
			if err != nil {
				return err
			}
			prevLines = n
		}
	}
}

// renderFrame renders a single watch frame without flicker.
//
// Strategy:
//  1. Render the header + tree into a buffer.
//  2. Move cursor to home position (top-left).
//  3. Write the buffer (overwrites previous content in-place).
//  4. If the new output has fewer lines than the previous, erase the
//     leftover lines so stale content doesn't linger.
//
// Returns the number of lines rendered.
func renderFrame(fn renderFunc, prevLines int) (int, error) {
	var buf bytes.Buffer

	// Write refresh timestamp header.
	ts := time.Now().Format("15:04:05")
	if !NoColor {
		fmt.Fprintf(&buf, "\033[2mEvery %s: kurt  %s\033[0m\n\n", WatchInterval, ts)
	} else {
		fmt.Fprintf(&buf, "Every %s: kurt  %s\n\n", WatchInterval, ts)
	}

	// Render the tree into the buffer.
	if err := fn(&buf); err != nil {
		return 0, err
	}

	content := buf.String()

	// Count lines in the rendered output.
	lines := strings.Count(content, "\n")
	// Account for content that doesn't end with a newline.
	if len(content) > 0 && content[len(content)-1] != '\n' {
		lines++
	}

	// Move cursor to home position (1,1) — no screen clear.
	fmt.Fprint(os.Stdout, "\033[H")

	// Append "erase to end of line" after every newline so that leftover
	// characters from a previously wider frame are cleared.
	content = strings.ReplaceAll(content, "\n", "\033[K\n")

	// Write the buffered content — overwrites previous frame in-place.
	fmt.Fprint(os.Stdout, content)

	// If the previous frame had more lines, erase the leftover ones.
	if prevLines > lines {
		for i := 0; i < prevLines-lines; i++ {
			// Clear entire line, then move down.
			fmt.Fprint(os.Stdout, "\033[2K\n")
		}
	}

	return lines, nil
}
