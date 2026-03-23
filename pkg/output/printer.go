package output

import (
	"fmt"
	"io"
	"math"
	"strings"
	"time"

	"github.com/kurt/kurt/pkg/model"
)

// ANSI color codes.
const (
	colorReset  = "\033[0m"
	colorGreen  = "\033[92m" // light green
	colorRed    = "\033[91m" // light red
	colorYellow = "\033[93m" // light yellow
	colorDim    = "\033[2m"  // dim/faint
	colorBold   = "\033[1m"
)

// NoColor disables ANSI color output when set to true.
var NoColor bool

// ExcludeKinds contains resource kinds to filter from the tree output.
var ExcludeKinds map[model.ResourceKind]bool

// ShowHosts enables the HOSTS column when set to true.
var ShowHosts bool

// ShowInactive shows inactive (zero-pod) ReplicaSets when set to true.
// By default they are hidden from the tree output.
var ShowInactive bool

// row represents a single flattened row for tabular output.
type row struct {
	namespace string
	name      string // includes tree-drawing prefix + Kind/Name
	ready     string
	reason    string
	status    string
	age       string
	hosts     string
	inactive  bool // true for unused ReplicaSets and their children
}

// col defines a column for tabular output.
type col struct {
	label string
	get   func(r row) string
}

// PrintTree renders the resource tree as a kubectl-style columnar table.
func PrintTree(w io.Writer, root *model.Node) {
	root = model.FilterTree(root, ExcludeKinds)
	rows := flattenTree(root, "", true, false)
	if len(rows) == 0 {
		return
	}

	// Build column list.
	cols := []col{
		{"NAMESPACE", func(r row) string { return r.namespace }},
		{"NAME", func(r row) string { return r.name }},
		{"READY", func(r row) string { return r.ready }},
		{"REASON", func(r row) string { return r.reason }},
		{"STATUS", func(r row) string { return r.status }},
	}
	if ShowHosts {
		cols = append(cols, col{"HOSTS", func(r row) string { return r.hosts }})
	}
	cols = append(cols, col{"AGE", func(r row) string { return r.age }})

	// Compute column widths.
	widths := make([]int, len(cols))
	for i, c := range cols {
		widths[i] = runeWidth(c.label)
	}
	for _, r := range rows {
		for i, c := range cols {
			if n := runeWidth(c.get(r)); n > widths[i] {
				widths[i] = n
			}
		}
	}

	// Print bold header.
	var hdrParts []string
	for i, c := range cols {
		if i == len(cols)-1 {
			hdrParts = append(hdrParts, c.label)
		} else {
			hdrParts = append(hdrParts, padRight(c.label, widths[i]))
		}
	}
	hdrLine := strings.Join(hdrParts, "  ")
	if NoColor {
		fmt.Fprintln(w, hdrLine)
	} else {
		fmt.Fprintf(w, "%s%s%s\n", colorBold, hdrLine, colorReset)
	}

	// Print data rows.
	for _, r := range rows {
		printRow(w, cols, widths, r)
	}
}

// printRow writes a single formatted row with color applied.
func printRow(w io.Writer, cols []col, widths []int, r row) {
	var parts []string
	for i, c := range cols {
		val := c.get(r)
		isLast := i == len(cols)-1
		var cell string
		if r.inactive && !NoColor {
			if isLast {
				cell = val
			} else {
				cell = padRight(val, widths[i])
			}
		} else {
			switch c.label {
			case "NAME":
				cell = colorizeName(val, widths[i])
			case "READY":
				cell = colorizeReady(val, widths[i])
			case "REASON":
				cell = colorizeReason(val, widths[i])
			case "STATUS":
				cell = colorizeStatus(val, widths[i])
			default:
				if isLast {
					cell = val
				} else {
					cell = padRight(val, widths[i])
				}
			}
		}
		parts = append(parts, cell)
	}
	line := strings.Join(parts, "  ")
	if r.inactive && !NoColor {
		fmt.Fprintf(w, "%s%s%s\n", colorDim, line, colorReset)
	} else {
		fmt.Fprintln(w, line)
	}
}

// colorizeReady applies color to the READY column value.
func colorizeReady(val string, width int) string {
	plain := padRight(val, width)
	if NoColor || val == "-" {
		return plain
	}
	parts := strings.SplitN(val, "/", 2)
	if len(parts) == 2 {
		if parts[0] == parts[1] {
			return colorGreen + plain + colorReset
		}
		return colorRed + plain + colorReset
	}
	switch strings.ToLower(val) {
	case "true":
		return colorGreen + plain + colorReset
	case "false":
		return colorRed + plain + colorReset
	default:
		return plain
	}
}

// colorizeReason applies color to the REASON column value.
func colorizeReason(val string, width int) string {
	plain := padRight(val, width)
	if NoColor || val == "" || val == "-" {
		return plain
	}
	switch strings.ToLower(val) {
	case "crashloopbackoff", "error", "oomkilled", "imagepullbackoff", "errimagepull",
		"createcontainererror", "createcontainerconfigerror", "runcontainererror":
		return colorRed + plain + colorReset
	default:
		return colorYellow + plain + colorReset
	}
}

// colorizeStatus applies color to the STATUS column value.
func colorizeStatus(val string, width int) string {
	plain := padRight(val, width)
	if NoColor {
		return plain
	}
	switch strings.ToLower(val) {
	case "running", "succeeded", "current", "active", "bound", "allow", "mutual", "istio_mutual":
		return colorGreen + plain + colorReset
	case "failed", "error", "unknown", "deny", "disable":
		return colorRed + plain + colorReset
	case "pending", "terminating", "containercreating", "permissive", "simple":
		return colorYellow + plain + colorReset
	default:
		return plain
	}
}

// colorizeName bolds the resource name (part after the last /) in the NAME column.
// Tree-drawing characters and the Kind prefix remain unstyled.
func colorizeName(val string, width int) string {
	plain := padRight(val, width)
	if NoColor {
		return plain
	}
	slashIdx := strings.LastIndex(val, "/")
	if slashIdx < 0 {
		return plain
	}
	prefix := val[:slashIdx+1]
	name := val[slashIdx+1:]
	padding := width - runeWidth(val)
	if padding < 0 {
		padding = 0
	}
	return prefix + colorBold + name + colorReset + strings.Repeat(" ", padding)
}

// isUnusedReplicaSet returns true if a node is a ReplicaSet with zero Pod children.
func isUnusedReplicaSet(node *model.Node) bool {
	if node.Kind != model.KindReplicaSet {
		return false
	}
	for _, child := range node.Children {
		if child.Kind == model.KindPod {
			return false
		}
	}
	return true
}

// flattenTree recursively converts the tree into a flat slice of rows.
// prefix is the tree-drawing prefix accumulated from parent levels.
// isRoot indicates whether this is the top-level node (no tree prefix).
// inactive propagates the dimming flag to all descendants.
func flattenTree(node *model.Node, prefix string, isRoot bool, inactive bool) []row {
	var rows []row

	// Detect if this node is an unused ReplicaSet.
	nodeInactive := inactive || isUnusedReplicaSet(node)

	// When ShowInactive is off, skip unused ReplicaSets entirely.
	if nodeInactive && !ShowInactive && !isRoot {
		return rows
	}

	displayName := string(node.Kind) + "/" + node.Name
	if !isRoot {
		displayName = prefix + displayName
	}

	rows = append(rows, row{
		namespace: node.Namespace,
		name:      displayName,
		ready:     node.Ready,
		reason:    node.Reason,
		status:    node.Status,
		hosts:     node.Hosts,
		age:       formatAge(node.CreatedAt),
		inactive:  nodeInactive,
	})

	for i, child := range node.Children {
		isLast := i == len(node.Children)-1

		var childConnector string
		var childPrefix string
		if isRoot {
			if isLast {
				childConnector = "└─"
				childPrefix = "  "
			} else {
				childConnector = "├─"
				childPrefix = "│ "
			}
		} else {
			if isLast {
				childConnector = prefix + "  └─"
				childPrefix = prefix + "    "
			} else {
				childConnector = prefix + "  ├─"
				childPrefix = prefix + "  │ "
			}
		}

		childRows := flattenChild(child, childConnector, childPrefix, nodeInactive)
		rows = append(rows, childRows...)
	}

	return rows
}

// flattenChild renders a child node with its connector prefix and recursively
// processes grandchildren with the continuation prefix.
func flattenChild(node *model.Node, connector string, continuation string, inactive bool) []row {
	var rows []row

	// Detect if this node is an unused ReplicaSet.
	nodeInactive := inactive || isUnusedReplicaSet(node)

	// When ShowInactive is off, skip unused ReplicaSets entirely.
	if nodeInactive && !ShowInactive {
		return rows
	}

	displayName := connector + string(node.Kind) + "/" + node.Name

	rows = append(rows, row{
		namespace: node.Namespace,
		name:      displayName,
		ready:     node.Ready,
		reason:    node.Reason,
		status:    node.Status,
		hosts:     node.Hosts,
		age:       formatAge(node.CreatedAt),
		inactive:  nodeInactive,
	})

	for i, child := range node.Children {
		isLast := i == len(node.Children)-1

		var childConnector string
		var childPrefix string
		if isLast {
			childConnector = continuation + "└─"
			childPrefix = continuation + "  "
		} else {
			childConnector = continuation + "├─"
			childPrefix = continuation + "│ "
		}

		childRows := flattenChild(child, childConnector, childPrefix, nodeInactive)
		rows = append(rows, childRows...)
	}

	return rows
}

// formatAge converts a timestamp to a human-readable age string like kubectl.
func formatAge(t time.Time) string {
	if t.IsZero() {
		return "-"
	}

	d := time.Since(t)
	if d < 0 {
		d = 0
	}

	seconds := int(math.Floor(d.Seconds()))
	if seconds < 60 {
		return fmt.Sprintf("%ds", seconds)
	}

	minutes := seconds / 60
	if minutes < 60 {
		return fmt.Sprintf("%dm", minutes)
	}

	hours := minutes / 60
	if hours < 24 {
		return fmt.Sprintf("%dh", hours)
	}

	days := hours / 24
	if days < 365 {
		return fmt.Sprintf("%dd", days)
	}

	years := days / 365
	remainDays := days % 365
	if remainDays == 0 {
		return fmt.Sprintf("%dy", years)
	}
	return fmt.Sprintf("%dy%dd", years, remainDays)
}

// runeWidth returns the display width of a string.
func runeWidth(s string) int {
	return len([]rune(s))
}

// padRight pads a string with spaces to reach the desired rune width.
func padRight(s string, width int) string {
	n := runeWidth(s)
	if n >= width {
		return s
	}
	return s + strings.Repeat(" ", width-n)
}
