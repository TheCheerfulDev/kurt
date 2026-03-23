package k8s

import (
	"context"
	"testing"

	"github.com/kurt/kurt/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
)

// ---------- isOwnedBy tests ----------

func TestIsOwnedBy_Match(t *testing.T) {
	uid := types.UID("abc-123")
	refs := []metav1.OwnerReference{
		{UID: "other-uid"},
		{UID: uid},
	}
	assert.True(t, isOwnedBy(refs, uid))
}

func TestIsOwnedBy_NoMatch(t *testing.T) {
	refs := []metav1.OwnerReference{
		{UID: "other-uid"},
	}
	assert.False(t, isOwnedBy(refs, "abc-123"))
}

func TestIsOwnedBy_EmptyRefs(t *testing.T) {
	assert.False(t, isOwnedBy(nil, "abc-123"))
}

// ---------- podReadyString tests ----------

func TestPodReadyString_AllReady(t *testing.T) {
	pod := &corev1.Pod{
		Status: corev1.PodStatus{
			ContainerStatuses: []corev1.ContainerStatus{
				{Ready: true},
				{Ready: true},
				{Ready: true},
			},
		},
	}
	assert.Equal(t, "3/3", podReadyString(pod))
}

func TestPodReadyString_PartialReady(t *testing.T) {
	pod := &corev1.Pod{
		Status: corev1.PodStatus{
			ContainerStatuses: []corev1.ContainerStatus{
				{Ready: true},
				{Ready: false},
				{Ready: true},
			},
		},
	}
	assert.Equal(t, "2/3", podReadyString(pod))
}

func TestPodReadyString_NoContainers(t *testing.T) {
	pod := &corev1.Pod{}
	assert.Equal(t, "0/0", podReadyString(pod))
}

// ---------- podReason tests ----------

func TestPodReason_Waiting(t *testing.T) {
	pod := &corev1.Pod{
		Status: corev1.PodStatus{
			ContainerStatuses: []corev1.ContainerStatus{
				{
					State: corev1.ContainerState{
						Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"},
					},
				},
			},
		},
	}
	assert.Equal(t, "CrashLoopBackOff", podReason(pod))
}

func TestPodReason_Terminated(t *testing.T) {
	pod := &corev1.Pod{
		Status: corev1.PodStatus{
			ContainerStatuses: []corev1.ContainerStatus{
				{
					State: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{Reason: "OOMKilled"},
					},
				},
			},
		},
	}
	assert.Equal(t, "OOMKilled", podReason(pod))
}

func TestPodReason_NoReason(t *testing.T) {
	pod := &corev1.Pod{
		Status: corev1.PodStatus{
			ContainerStatuses: []corev1.ContainerStatus{
				{
					State: corev1.ContainerState{
						Running: &corev1.ContainerStateRunning{},
					},
				},
			},
		},
	}
	assert.Equal(t, "", podReason(pod))
}

// ---------- podStatusString tests ----------

func TestPodStatusString_Running(t *testing.T) {
	pod := &corev1.Pod{
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}
	assert.Equal(t, "Running", podStatusString(pod))
}

func TestPodStatusString_Pending(t *testing.T) {
	pod := &corev1.Pod{
		Status: corev1.PodStatus{Phase: corev1.PodPending},
	}
	assert.Equal(t, "Pending", podStatusString(pod))
}

func TestPodStatusString_Failed(t *testing.T) {
	pod := &corev1.Pod{
		Status: corev1.PodStatus{Phase: corev1.PodFailed},
	}
	assert.Equal(t, "Failed", podStatusString(pod))
}

func TestPodStatusString_Succeeded(t *testing.T) {
	pod := &corev1.Pod{
		Status: corev1.PodStatus{Phase: corev1.PodSucceeded},
	}
	assert.Equal(t, "Succeeded", podStatusString(pod))
}

// ---------- FindOwnedReplicaSets tests ----------

func TestFindOwnedReplicaSets(t *testing.T) {
	ctx := context.Background()
	deployUID := types.UID("deploy-uid-1")

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-app",
			Namespace: "default",
			UID:       deployUID,
		},
	}

	ownedRS := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-app-abc",
			Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{
				{UID: deployUID},
			},
		},
	}

	unownedRS := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "other-app-xyz",
			Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{
				{UID: "other-uid"},
			},
		},
	}

	client := fake.NewSimpleClientset(ownedRS, unownedRS)

	nodes, err := FindOwnedReplicaSets(ctx, client, "default", deployment)
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	assert.Equal(t, model.KindReplicaSet, nodes[0].Kind)
	assert.Equal(t, "my-app-abc", nodes[0].Name)
}

func TestFindOwnedReplicaSets_NoMatches(t *testing.T) {
	ctx := context.Background()
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-app",
			Namespace: "default",
			UID:       "deploy-uid-1",
		},
	}

	client := fake.NewSimpleClientset()
	nodes, err := FindOwnedReplicaSets(ctx, client, "default", deployment)
	require.NoError(t, err)
	assert.Empty(t, nodes)
}

// ---------- FindOwnedPods tests ----------

func TestFindOwnedPods(t *testing.T) {
	ctx := context.Background()
	rsUID := types.UID("rs-uid-1")

	ownedPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-pod-abc",
			Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{
				{UID: rsUID},
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{
				{Ready: true},
			},
		},
	}

	unownedPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "other-pod",
			Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{
				{UID: "other-uid"},
			},
		},
	}

	client := fake.NewSimpleClientset(ownedPod, unownedPod)

	nodes, err := FindOwnedPods(ctx, client, "default", rsUID)
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	assert.Equal(t, model.KindPod, nodes[0].Kind)
	assert.Equal(t, "my-pod-abc", nodes[0].Name)
	assert.Equal(t, "1/1", nodes[0].Ready)
	assert.Equal(t, "Running", nodes[0].Status)
}

func TestFindOwnedPods_NoMatches(t *testing.T) {
	ctx := context.Background()
	client := fake.NewSimpleClientset()

	nodes, err := FindOwnedPods(ctx, client, "default", "nonexistent-uid")
	require.NoError(t, err)
	assert.Empty(t, nodes)
}

// ---------- FindOwnedPods with waiting reason ----------

func TestFindOwnedPods_WithWaitingReason(t *testing.T) {
	ctx := context.Background()
	rsUID := types.UID("rs-uid-1")

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "crash-pod",
			Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{
				{UID: rsUID},
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Ready: false,
					State: corev1.ContainerState{
						Waiting: &corev1.ContainerStateWaiting{
							Reason: "CrashLoopBackOff",
						},
					},
				},
			},
		},
	}

	client := fake.NewSimpleClientset(pod)

	nodes, err := FindOwnedPods(ctx, client, "default", rsUID)
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	assert.Equal(t, "0/1", nodes[0].Ready)
	assert.Equal(t, "CrashLoopBackOff", nodes[0].Reason)
	assert.Equal(t, "Running", nodes[0].Status)
}
