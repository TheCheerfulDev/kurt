package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewNode(t *testing.T) {
	n := NewNode(KindDeployment, "my-app", "default")

	assert.Equal(t, KindDeployment, n.Kind)
	assert.Equal(t, "my-app", n.Name)
	assert.Equal(t, "default", n.Namespace)
	assert.Equal(t, "-", n.Ready)
	assert.Equal(t, "-", n.Status)
	assert.Empty(t, n.Reason)
	assert.Empty(t, n.Hosts)
	assert.True(t, n.CreatedAt.IsZero())
	assert.NotNil(t, n.Children, "Children slice should be initialized, not nil")
	assert.Empty(t, n.Children)
}

func TestAddChild(t *testing.T) {
	parent := NewNode(KindDeployment, "my-app", "default")
	child1 := NewNode(KindReplicaSet, "my-app-abc123", "default")
	child2 := NewNode(KindReplicaSet, "my-app-def456", "default")

	parent.AddChild(child1)
	assert.Len(t, parent.Children, 1)
	assert.Equal(t, child1, parent.Children[0])

	parent.AddChild(child2)
	assert.Len(t, parent.Children, 2)
	assert.Equal(t, child2, parent.Children[1])
}

func TestFilterTree_NoExclusions(t *testing.T) {
	root := NewNode(KindDeployment, "my-app", "default")
	rs := NewNode(KindReplicaSet, "my-app-abc", "default")
	pod := NewNode(KindPod, "my-app-abc-xyz", "default")
	rs.AddChild(pod)
	root.AddChild(rs)

	// Empty map — returns same tree.
	result := FilterTree(root, map[ResourceKind]bool{})
	assert.Equal(t, root, result, "empty exclude map should return the original root")
}

func TestFilterTree_NilExclusions(t *testing.T) {
	root := NewNode(KindDeployment, "my-app", "default")
	rs := NewNode(KindReplicaSet, "my-app-abc", "default")
	root.AddChild(rs)

	result := FilterTree(root, nil)
	assert.Equal(t, root, result, "nil exclude map should return the original root")
}

func TestFilterTree_ExcludesKindAndPromotesChildren(t *testing.T) {
	root := NewNode(KindDeployment, "my-app", "default")
	rs := NewNode(KindReplicaSet, "my-app-abc", "default")
	pod := NewNode(KindPod, "my-app-abc-xyz", "default")
	rs.AddChild(pod)
	root.AddChild(rs)

	exclude := map[ResourceKind]bool{KindReplicaSet: true}
	result := FilterTree(root, exclude)

	// Root should still be Deployment.
	assert.Equal(t, KindDeployment, result.Kind)
	// ReplicaSet should be removed; Pod promoted to root's children.
	require.Len(t, result.Children, 1)
	assert.Equal(t, KindPod, result.Children[0].Kind)
	assert.Equal(t, "my-app-abc-xyz", result.Children[0].Name)
}

func TestFilterTree_RootNeverExcluded(t *testing.T) {
	root := NewNode(KindDeployment, "my-app", "default")
	rs := NewNode(KindReplicaSet, "my-app-abc", "default")
	root.AddChild(rs)

	exclude := map[ResourceKind]bool{KindDeployment: true}
	result := FilterTree(root, exclude)

	// Root kind is in exclude set, but root itself must never be excluded.
	assert.Equal(t, KindDeployment, result.Kind)
	assert.Equal(t, "my-app", result.Name)
}

func TestFilterTree_NestedExclusions(t *testing.T) {
	// Tree: Deployment → Service → VirtualService → Gateway
	root := NewNode(KindDeployment, "my-app", "default")
	svc := NewNode(KindService, "my-svc", "default")
	vs := NewNode(KindVirtualService, "my-vs", "default")
	gw := NewNode(KindGateway, "my-gw", "default")
	vs.AddChild(gw)
	svc.AddChild(vs)
	root.AddChild(svc)

	// Exclude both Service and VirtualService — Gateway should be promoted to root.
	exclude := map[ResourceKind]bool{KindService: true, KindVirtualService: true}
	result := FilterTree(root, exclude)

	require.Len(t, result.Children, 1)
	assert.Equal(t, KindGateway, result.Children[0].Kind)
	assert.Equal(t, "my-gw", result.Children[0].Name)
}

func TestFilterTree_EmptyExcludeMap(t *testing.T) {
	root := NewNode(KindStatefulSet, "my-sts", "default")
	pod := NewNode(KindPod, "my-sts-0", "default")
	root.AddChild(pod)

	result := FilterTree(root, map[ResourceKind]bool{})
	// Should be the same pointer since no exclusions.
	assert.Equal(t, root, result)
}

func TestFilterTree_ExcludeLeafNode(t *testing.T) {
	root := NewNode(KindDeployment, "my-app", "default")
	rs := NewNode(KindReplicaSet, "my-app-abc", "default")
	pod := NewNode(KindPod, "my-app-abc-xyz", "default")
	rs.AddChild(pod)
	root.AddChild(rs)

	// Exclude Pod (leaf node) — RS should remain with no children.
	exclude := map[ResourceKind]bool{KindPod: true}
	result := FilterTree(root, exclude)

	require.Len(t, result.Children, 1)
	assert.Equal(t, KindReplicaSet, result.Children[0].Kind)
	assert.Empty(t, result.Children[0].Children)
}

func TestFilterTree_DoesNotMutateOriginal(t *testing.T) {
	root := NewNode(KindDeployment, "my-app", "default")
	rs := NewNode(KindReplicaSet, "my-app-abc", "default")
	pod := NewNode(KindPod, "my-app-abc-xyz", "default")
	rs.AddChild(pod)
	root.AddChild(rs)

	exclude := map[ResourceKind]bool{KindReplicaSet: true}
	_ = FilterTree(root, exclude)

	// Original tree should remain untouched.
	require.Len(t, root.Children, 1)
	assert.Equal(t, KindReplicaSet, root.Children[0].Kind)
	require.Len(t, root.Children[0].Children, 1)
	assert.Equal(t, KindPod, root.Children[0].Children[0].Kind)
}

func TestFilterTree_MultipleChildrenWithExclusion(t *testing.T) {
	root := NewNode(KindDeployment, "my-app", "default")
	rs1 := NewNode(KindReplicaSet, "rs1", "default")
	rs2 := NewNode(KindReplicaSet, "rs2", "default")
	pod1 := NewNode(KindPod, "pod1", "default")
	pod2 := NewNode(KindPod, "pod2", "default")
	svc := NewNode(KindService, "my-svc", "default")

	rs1.AddChild(pod1)
	rs2.AddChild(pod2)
	root.AddChild(rs1)
	root.AddChild(rs2)
	root.AddChild(svc)

	exclude := map[ResourceKind]bool{KindReplicaSet: true}
	result := FilterTree(root, exclude)

	// Should have: pod1, pod2 (promoted), svc (kept).
	require.Len(t, result.Children, 3)
	assert.Equal(t, KindPod, result.Children[0].Kind)
	assert.Equal(t, "pod1", result.Children[0].Name)
	assert.Equal(t, KindPod, result.Children[1].Kind)
	assert.Equal(t, "pod2", result.Children[1].Name)
	assert.Equal(t, KindService, result.Children[2].Kind)
}
