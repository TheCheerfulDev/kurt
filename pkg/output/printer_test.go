package output

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/kurt/kurt/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// resetGlobals resets all package-level vars to their defaults between tests.
func resetGlobals(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		NoColor = false
		ExcludeKinds = nil
		ShowHosts = false
		ShowInactive = false
	})
}

// ---------- formatAge tests ----------

func TestFormatAge_ZeroTime(t *testing.T) {
	assert.Equal(t, "-", formatAge(time.Time{}))
}

func TestFormatAge_Seconds(t *testing.T) {
	ts := time.Now().Add(-30 * time.Second)
	result := formatAge(ts)
	assert.Regexp(t, `^\d+s$`, result)
}

func TestFormatAge_Minutes(t *testing.T) {
	ts := time.Now().Add(-5 * time.Minute)
	result := formatAge(ts)
	assert.Regexp(t, `^\d+m$`, result)
}

func TestFormatAge_Hours(t *testing.T) {
	ts := time.Now().Add(-3 * time.Hour)
	result := formatAge(ts)
	assert.Regexp(t, `^\d+h$`, result)
}

func TestFormatAge_Days(t *testing.T) {
	ts := time.Now().Add(-78 * 24 * time.Hour)
	result := formatAge(ts)
	assert.Regexp(t, `^\d+d$`, result)
}

func TestFormatAge_Years(t *testing.T) {
	ts := time.Now().Add(-400 * 24 * time.Hour)
	result := formatAge(ts)
	assert.Regexp(t, `^\d+y\d*d?$`, result)
}

// ---------- colorizeReady tests ----------

func TestColorizeReady_NoColor(t *testing.T) {
	NoColor = true
	defer func() { NoColor = false }()
	result := colorizeReady("3/3", 10)
	assert.NotContains(t, result, "\033[")
	assert.Contains(t, result, "3/3")
}

func TestColorizeReady_Dash(t *testing.T) {
	result := colorizeReady("-", 10)
	// Dash is returned as-is (no color).
	assert.NotContains(t, result, colorGreen)
	assert.NotContains(t, result, colorRed)
}

func TestColorizeReady_AllReady(t *testing.T) {
	result := colorizeReady("3/3", 10)
	assert.Contains(t, result, colorGreen)
}

func TestColorizeReady_PartialReady(t *testing.T) {
	result := colorizeReady("1/3", 10)
	assert.Contains(t, result, colorRed)
}

func TestColorizeReady_True(t *testing.T) {
	result := colorizeReady("True", 10)
	assert.Contains(t, result, colorGreen)
}

func TestColorizeReady_False(t *testing.T) {
	result := colorizeReady("False", 10)
	assert.Contains(t, result, colorRed)
}

// ---------- colorizeReason tests ----------

func TestColorizeReason_NoColor(t *testing.T) {
	NoColor = true
	defer func() { NoColor = false }()
	result := colorizeReason("CrashLoopBackOff", 20)
	assert.NotContains(t, result, "\033[")
}

func TestColorizeReason_Empty(t *testing.T) {
	result := colorizeReason("", 10)
	assert.NotContains(t, result, colorRed)
	assert.NotContains(t, result, colorYellow)
}

func TestColorizeReason_Dash(t *testing.T) {
	result := colorizeReason("-", 10)
	assert.NotContains(t, result, colorRed)
}

func TestColorizeReason_CrashLoopBackOff(t *testing.T) {
	result := colorizeReason("CrashLoopBackOff", 20)
	assert.Contains(t, result, colorRed)
}

func TestColorizeReason_OOMKilled(t *testing.T) {
	result := colorizeReason("OOMKilled", 20)
	assert.Contains(t, result, colorRed)
}

func TestColorizeReason_ImagePullBackOff(t *testing.T) {
	result := colorizeReason("ImagePullBackOff", 20)
	assert.Contains(t, result, colorRed)
}

func TestColorizeReason_UnknownReason(t *testing.T) {
	result := colorizeReason("ContainerCreating", 20)
	assert.Contains(t, result, colorYellow)
}

// ---------- colorizeStatus tests ----------

func TestColorizeStatus_NoColor(t *testing.T) {
	NoColor = true
	defer func() { NoColor = false }()
	result := colorizeStatus("Running", 10)
	assert.NotContains(t, result, "\033[")
}

func TestColorizeStatus_Running(t *testing.T) {
	result := colorizeStatus("Running", 10)
	assert.Contains(t, result, colorGreen)
}

func TestColorizeStatus_Allow(t *testing.T) {
	result := colorizeStatus("ALLOW", 10)
	assert.Contains(t, result, colorGreen)
}

func TestColorizeStatus_Mutual(t *testing.T) {
	result := colorizeStatus("MUTUAL", 10)
	assert.Contains(t, result, colorGreen)
}

func TestColorizeStatus_IstioMutual(t *testing.T) {
	result := colorizeStatus("ISTIO_MUTUAL", 15)
	assert.Contains(t, result, colorGreen)
}

func TestColorizeStatus_Failed(t *testing.T) {
	result := colorizeStatus("Failed", 10)
	assert.Contains(t, result, colorRed)
}

func TestColorizeStatus_Deny(t *testing.T) {
	result := colorizeStatus("DENY", 10)
	assert.Contains(t, result, colorRed)
}

func TestColorizeStatus_Disable(t *testing.T) {
	result := colorizeStatus("DISABLE", 10)
	assert.Contains(t, result, colorRed)
}

func TestColorizeStatus_Pending(t *testing.T) {
	result := colorizeStatus("Pending", 10)
	assert.Contains(t, result, colorYellow)
}

func TestColorizeStatus_Permissive(t *testing.T) {
	result := colorizeStatus("PERMISSIVE", 12)
	assert.Contains(t, result, colorYellow)
}

func TestColorizeStatus_Simple(t *testing.T) {
	result := colorizeStatus("SIMPLE", 10)
	assert.Contains(t, result, colorYellow)
}

func TestColorizeStatus_DefaultNoColor(t *testing.T) {
	result := colorizeStatus("-", 10)
	assert.NotContains(t, result, colorGreen)
	assert.NotContains(t, result, colorRed)
	assert.NotContains(t, result, colorYellow)
}

// ---------- isUnusedReplicaSet tests ----------

func TestIsUnusedReplicaSet_NoPods(t *testing.T) {
	rs := model.NewNode(model.KindReplicaSet, "my-rs", "default")
	assert.True(t, isUnusedReplicaSet(rs))
}

func TestIsUnusedReplicaSet_WithPods(t *testing.T) {
	rs := model.NewNode(model.KindReplicaSet, "my-rs", "default")
	rs.AddChild(model.NewNode(model.KindPod, "pod-1", "default"))
	assert.False(t, isUnusedReplicaSet(rs))
}

func TestIsUnusedReplicaSet_NotReplicaSet(t *testing.T) {
	dep := model.NewNode(model.KindDeployment, "my-dep", "default")
	assert.False(t, isUnusedReplicaSet(dep))
}

func TestIsUnusedReplicaSet_WithNonPodChildren(t *testing.T) {
	// RS with only non-pod children is still "unused" (no pods).
	rs := model.NewNode(model.KindReplicaSet, "my-rs", "default")
	rs.AddChild(model.NewNode(model.KindService, "svc", "default"))
	assert.True(t, isUnusedReplicaSet(rs))
}

// ---------- padRight / runeWidth tests ----------

func TestPadRight(t *testing.T) {
	assert.Equal(t, "hello     ", padRight("hello", 10))
	assert.Equal(t, "hello", padRight("hello", 5))
	assert.Equal(t, "hello", padRight("hello", 3))
}

func TestRuneWidth(t *testing.T) {
	assert.Equal(t, 5, runeWidth("hello"))
	assert.Equal(t, 0, runeWidth(""))
}

// ---------- PrintTree tests ----------

func TestPrintTree_NoColor_BasicOutput(t *testing.T) {
	resetGlobals(t)
	NoColor = true

	root := model.NewNode(model.KindDeployment, "my-app", "production")
	root.CreatedAt = time.Now().Add(-24 * time.Hour)
	rs := model.NewNode(model.KindReplicaSet, "my-app-abc", "production")
	rs.CreatedAt = time.Now().Add(-24 * time.Hour)
	pod := model.NewNode(model.KindPod, "my-app-abc-xyz", "production")
	pod.Ready = "3/3"
	pod.Status = "Running"
	pod.CreatedAt = time.Now().Add(-12 * time.Hour)
	rs.AddChild(pod)
	root.AddChild(rs)

	var buf bytes.Buffer
	PrintTree(&buf, root)
	output := buf.String()

	// Header present.
	assert.Contains(t, output, "NAMESPACE")
	assert.Contains(t, output, "NAME")
	assert.Contains(t, output, "READY")
	assert.Contains(t, output, "REASON")
	assert.Contains(t, output, "STATUS")
	assert.Contains(t, output, "AGE")

	// Data rows present.
	assert.Contains(t, output, "Deployment/my-app")
	assert.Contains(t, output, "ReplicaSet/my-app-abc")
	assert.Contains(t, output, "Pod/my-app-abc-xyz")
	assert.Contains(t, output, "3/3")
	assert.Contains(t, output, "Running")

	// No ANSI codes.
	assert.NotContains(t, output, "\033[")

	// No HOSTS column when ShowHosts is false.
	lines := strings.Split(output, "\n")
	require.True(t, len(lines) > 0)
	assert.NotContains(t, lines[0], "HOSTS")
}

func TestPrintTree_ShowHosts(t *testing.T) {
	resetGlobals(t)
	NoColor = true
	ShowHosts = true

	root := model.NewNode(model.KindVirtualService, "my-vs", "default")
	root.Hosts = "app.example.com"
	root.CreatedAt = time.Now()

	var buf bytes.Buffer
	PrintTree(&buf, root)
	output := buf.String()

	// HOSTS column in header.
	lines := strings.Split(output, "\n")
	require.True(t, len(lines) > 0)
	assert.Contains(t, lines[0], "HOSTS")

	// Host value in data row.
	assert.Contains(t, output, "app.example.com")
}

func TestPrintTree_ExcludeKinds(t *testing.T) {
	resetGlobals(t)
	NoColor = true
	ExcludeKinds = map[model.ResourceKind]bool{model.KindReplicaSet: true}

	root := model.NewNode(model.KindDeployment, "my-app", "default")
	rs := model.NewNode(model.KindReplicaSet, "my-app-abc", "default")
	pod := model.NewNode(model.KindPod, "my-app-abc-xyz", "default")
	pod.CreatedAt = time.Now()
	rs.AddChild(pod)
	root.AddChild(rs)
	root.CreatedAt = time.Now()

	var buf bytes.Buffer
	PrintTree(&buf, root)
	output := buf.String()

	assert.NotContains(t, output, "ReplicaSet/")
	assert.Contains(t, output, "Pod/my-app-abc-xyz")
}

func TestPrintTree_InactiveHiddenByDefault(t *testing.T) {
	resetGlobals(t)
	NoColor = true
	ShowInactive = false

	root := model.NewNode(model.KindDeployment, "my-app", "default")
	root.CreatedAt = time.Now()

	// Active RS with a pod.
	activeRS := model.NewNode(model.KindReplicaSet, "active-rs", "default")
	activeRS.CreatedAt = time.Now()
	pod := model.NewNode(model.KindPod, "pod-1", "default")
	pod.CreatedAt = time.Now()
	pod.Ready = "1/1"
	pod.Status = "Running"
	activeRS.AddChild(pod)

	// Inactive RS with no pods.
	inactiveRS := model.NewNode(model.KindReplicaSet, "inactive-rs", "default")
	inactiveRS.CreatedAt = time.Now()

	root.AddChild(activeRS)
	root.AddChild(inactiveRS)

	var buf bytes.Buffer
	PrintTree(&buf, root)
	output := buf.String()

	assert.Contains(t, output, "ReplicaSet/active-rs")
	assert.NotContains(t, output, "ReplicaSet/inactive-rs")
}

func TestPrintTree_InactiveShownWithFlag(t *testing.T) {
	resetGlobals(t)
	NoColor = true
	ShowInactive = true

	root := model.NewNode(model.KindDeployment, "my-app", "default")
	root.CreatedAt = time.Now()

	inactiveRS := model.NewNode(model.KindReplicaSet, "inactive-rs", "default")
	inactiveRS.CreatedAt = time.Now()
	root.AddChild(inactiveRS)

	var buf bytes.Buffer
	PrintTree(&buf, root)
	output := buf.String()

	assert.Contains(t, output, "ReplicaSet/inactive-rs")
}

func TestPrintTree_EmptyTree(t *testing.T) {
	resetGlobals(t)
	NoColor = true

	// This really shouldn't produce output if root is excluded.
	// But since root is never excluded, a single-node tree should produce a header + 1 row.
	root := model.NewNode(model.KindDeployment, "my-app", "default")
	root.CreatedAt = time.Now()

	var buf bytes.Buffer
	PrintTree(&buf, root)
	output := buf.String()

	assert.Contains(t, output, "Deployment/my-app")
}

func TestPrintTree_TreeDrawingCharacters(t *testing.T) {
	resetGlobals(t)
	NoColor = true

	root := model.NewNode(model.KindDeployment, "my-app", "default")
	root.CreatedAt = time.Now()
	child1 := model.NewNode(model.KindReplicaSet, "rs1", "default")
	child1.CreatedAt = time.Now()
	pod := model.NewNode(model.KindPod, "pod1", "default")
	pod.CreatedAt = time.Now()
	child1.AddChild(pod)
	child2 := model.NewNode(model.KindService, "svc1", "default")
	child2.CreatedAt = time.Now()
	root.AddChild(child1)
	root.AddChild(child2)

	var buf bytes.Buffer
	PrintTree(&buf, root)
	output := buf.String()

	// First child uses ├─, last child uses └─.
	assert.Contains(t, output, "├─ReplicaSet/rs1")
	assert.Contains(t, output, "└─Service/svc1")
}

func TestPrintTree_NestedTreeDrawing(t *testing.T) {
	resetGlobals(t)
	NoColor = true

	root := model.NewNode(model.KindDeployment, "my-app", "default")
	root.CreatedAt = time.Now()
	rs := model.NewNode(model.KindReplicaSet, "rs1", "default")
	rs.CreatedAt = time.Now()
	pod := model.NewNode(model.KindPod, "pod1", "default")
	pod.CreatedAt = time.Now()
	rs.AddChild(pod)
	svc := model.NewNode(model.KindService, "svc1", "default")
	svc.CreatedAt = time.Now()
	root.AddChild(rs)
	root.AddChild(svc)

	var buf bytes.Buffer
	PrintTree(&buf, root)
	output := buf.String()

	// Pod under RS (which is not last child) should use "│ └─".
	assert.Contains(t, output, "│ └─Pod/pod1")
}

// ---------- colorizeName tests ----------

func TestColorizeName_WithSlash(t *testing.T) {
	result := colorizeName("Deployment/my-app", 30)
	assert.Contains(t, result, colorBold)
	assert.Contains(t, result, "my-app")
	assert.Contains(t, result, "Deployment/")
	assert.Contains(t, result, colorReset)
}

func TestColorizeName_NoColor(t *testing.T) {
	NoColor = true
	defer func() { NoColor = false }()
	result := colorizeName("Deployment/my-app", 30)
	assert.NotContains(t, result, "\033[")
	assert.Contains(t, result, "Deployment/my-app")
}

func TestColorizeName_NoSlash(t *testing.T) {
	// No slash means no bold — return plain padded.
	result := colorizeName("something", 20)
	assert.NotContains(t, result, colorBold)
	assert.Equal(t, padRight("something", 20), result)
}

func TestColorizeName_TreePrefix(t *testing.T) {
	// Tree-drawing chars before the Kind/Name.
	result := colorizeName("├─ReplicaSet/rs-abc", 30)
	assert.Contains(t, result, "├─ReplicaSet/")
	assert.Contains(t, result, colorBold+"rs-abc"+colorReset)
}

func TestColorizeName_NestedTreePrefix(t *testing.T) {
	result := colorizeName("│ └─Pod/pod-xyz", 30)
	assert.Contains(t, result, "│ └─Pod/")
	assert.Contains(t, result, colorBold+"pod-xyz"+colorReset)
}

func TestColorizeName_Padding(t *testing.T) {
	// The plain text "Deployment/my-app" is 17 runes.
	// With width=20, we expect 3 trailing spaces after the reset code.
	result := colorizeName("Deployment/my-app", 20)
	expected := "Deployment/" + colorBold + "my-app" + colorReset + "   "
	assert.Equal(t, expected, result)
}

func TestColorizeName_ExactWidth(t *testing.T) {
	// When val length == width, no extra padding.
	result := colorizeName("Pod/abc", 7)
	expected := "Pod/" + colorBold + "abc" + colorReset
	assert.Equal(t, expected, result)
}

func TestColorizeName_PrintTree_BoldInOutput(t *testing.T) {
	resetGlobals(t)
	ShowInactive = true
	// Color ON (default).
	root := model.NewNode(model.KindDeployment, "my-app", "default")
	root.CreatedAt = time.Now()
	rs := model.NewNode(model.KindReplicaSet, "rs1", "default")
	rs.CreatedAt = time.Now()
	pod := model.NewNode(model.KindPod, "pod-1", "default")
	pod.CreatedAt = time.Now()
	pod.Ready = "1/1"
	pod.Status = "Running"
	rs.AddChild(pod)
	root.AddChild(rs)

	var buf bytes.Buffer
	PrintTree(&buf, root)
	output := buf.String()

	// The resource name part should be bolded.
	assert.Contains(t, output, colorBold+"my-app"+colorReset)
	assert.Contains(t, output, colorBold+"rs1"+colorReset)
	assert.Contains(t, output, colorBold+"pod-1"+colorReset)
}
