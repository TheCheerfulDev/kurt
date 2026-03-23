package k8s

import (
	"context"
	"fmt"

	"github.com/kurt/kurt/pkg/model"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

func FindServicesForLabels(ctx context.Context, client kubernetes.Interface, namespace string, podLabels map[string]string) ([]*corev1.Service, error) {
	svcList, err := client.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing services: %w", err)
	}

	podLabelSet := labels.Set(podLabels)
	var matched []*corev1.Service

	for i := range svcList.Items {
		svc := &svcList.Items[i]
		if len(svc.Spec.Selector) == 0 {
			continue
		}

		svcSelector := labels.SelectorFromSet(labels.Set(svc.Spec.Selector))
		if svcSelector.Matches(podLabelSet) {
			matched = append(matched, svc)
		}
	}

	return matched, nil
}

func ServiceToNode(svc *corev1.Service) *model.Node {
	n := model.NewNode(model.KindService, svc.Name, svc.Namespace)
	n.CreatedAt = svc.CreationTimestamp.Time
	return n
}

// GetServiceByName fetches a single Service by name.
func GetServiceByName(ctx context.Context, client kubernetes.Interface, namespace, name string) (*corev1.Service, error) {
	svc, err := client.CoreV1().Services(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting service %s/%s: %w", namespace, name, err)
	}
	return svc, nil
}

// FindDeploymentsForService returns Deployment nodes whose pod template labels
// match the given service's selector.
func FindDeploymentsForService(ctx context.Context, client kubernetes.Interface, dynClient dynamic.Interface, namespace string, svc *corev1.Service) ([]*model.Node, error) {
	if len(svc.Spec.Selector) == 0 {
		return nil, nil
	}
	svcSelector := labels.SelectorFromSet(labels.Set(svc.Spec.Selector))

	depList, err := client.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing deployments: %w", err)
	}

	var nodes []*model.Node
	for i := range depList.Items {
		dep := &depList.Items[i]
		podLabels := labels.Set(dep.Spec.Template.Labels)
		if !svcSelector.Matches(podLabels) {
			continue
		}

		depNode := model.NewNode(model.KindDeployment, dep.Name, dep.Namespace)
		depNode.CreatedAt = dep.CreationTimestamp.Time

		replicaSets, err := FindOwnedReplicaSets(ctx, client, namespace, dep)
		if err != nil {
			return nil, err
		}
		for _, rs := range replicaSets {
			depNode.AddChild(rs)
		}
		// Attach AuthorizationPolicies matching this Deployment's pod template labels.
		aps, err := FindAuthorizationPoliciesForLabels(ctx, dynClient, namespace, dep.Spec.Template.Labels)
		if err != nil {
			return nil, err
		}
		for _, ap := range aps {
			depNode.AddChild(ap)
		}

		nodes = append(nodes, depNode)
	}
	return nodes, nil
}

// FindStatefulSetsForService returns StatefulSet nodes whose pod template labels
// match the given service's selector.
func FindStatefulSetsForService(ctx context.Context, client kubernetes.Interface, dynClient dynamic.Interface, namespace string, svc *corev1.Service) ([]*model.Node, error) {
	if len(svc.Spec.Selector) == 0 {
		return nil, nil
	}
	svcSelector := labels.SelectorFromSet(labels.Set(svc.Spec.Selector))

	stsList, err := client.AppsV1().StatefulSets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing statefulsets: %w", err)
	}

	var nodes []*model.Node
	for i := range stsList.Items {
		sts := &stsList.Items[i]
		podLabels := labels.Set(sts.Spec.Template.Labels)
		if !svcSelector.Matches(podLabels) {
			continue
		}

		stsNode := model.NewNode(model.KindStatefulSet, sts.Name, sts.Namespace)
		stsNode.CreatedAt = sts.CreationTimestamp.Time

		pods, err := FindOwnedPodsByStatefulSet(ctx, client, namespace, sts)
		if err != nil {
			return nil, err
		}
		for _, pod := range pods {
			stsNode.AddChild(pod)
		}
		// Attach AuthorizationPolicies matching this StatefulSet's pod template labels.
		aps, err := FindAuthorizationPoliciesForLabels(ctx, dynClient, namespace, sts.Spec.Template.Labels)
		if err != nil {
			return nil, err
		}
		for _, ap := range aps {
			stsNode.AddChild(ap)
		}

		nodes = append(nodes, stsNode)
	}
	return nodes, nil
}
