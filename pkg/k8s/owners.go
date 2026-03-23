package k8s

import (
	"context"
	"fmt"

	"github.com/kurt/kurt/pkg/model"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
)

func FindOwnedReplicaSets(ctx context.Context, client kubernetes.Interface, namespace string, deployment *appsv1.Deployment) ([]*model.Node, error) {
	rsList, err := client.AppsV1().ReplicaSets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing replicasets: %w", err)
	}

	var nodes []*model.Node
	for i := range rsList.Items {
		rs := &rsList.Items[i]
		if !isOwnedBy(rs.OwnerReferences, deployment.UID) {
			continue
		}

		rsNode := model.NewNode(model.KindReplicaSet, rs.Name, rs.Namespace)
		rsNode.CreatedAt = rs.CreationTimestamp.Time

		pods, err := FindOwnedPods(ctx, client, namespace, rs.UID)
		if err != nil {
			return nil, err
		}
		for _, pod := range pods {
			rsNode.AddChild(pod)
		}

		nodes = append(nodes, rsNode)
	}
	return nodes, nil
}

func FindOwnedPodsByStatefulSet(ctx context.Context, client kubernetes.Interface, namespace string, sts *appsv1.StatefulSet) ([]*model.Node, error) {
	return FindOwnedPods(ctx, client, namespace, sts.UID)
}

func FindOwnedPods(ctx context.Context, client kubernetes.Interface, namespace string, ownerUID types.UID) ([]*model.Node, error) {
	podList, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing pods: %w", err)
	}

	var nodes []*model.Node
	for i := range podList.Items {
		pod := &podList.Items[i]
		if !isOwnedBy(pod.OwnerReferences, ownerUID) {
			continue
		}

		n := model.NewNode(model.KindPod, pod.Name, pod.Namespace)
		n.CreatedAt = pod.CreationTimestamp.Time
		n.Ready = podReadyString(pod)
		n.Reason = podReason(pod)
		n.Status = podStatusString(pod)
		nodes = append(nodes, n)
	}
	return nodes, nil
}

func isOwnedBy(refs []metav1.OwnerReference, uid types.UID) bool {
	for _, ref := range refs {
		if ref.UID == uid {
			return true
		}
	}
	return false
}

func podReadyString(pod *corev1.Pod) string {
	ready := 0
	total := len(pod.Status.ContainerStatuses)
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.Ready {
			ready++
		}
	}
	return fmt.Sprintf("%d/%d", ready, total)
}

func podReason(pod *corev1.Pod) string {
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.State.Waiting != nil && cs.State.Waiting.Reason != "" {
			return cs.State.Waiting.Reason
		}
		if cs.State.Terminated != nil && cs.State.Terminated.Reason != "" {
			return cs.State.Terminated.Reason
		}
	}
	return ""
}

func podStatusString(pod *corev1.Pod) string {
	return string(pod.Status.Phase)
}
