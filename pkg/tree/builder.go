package tree

import (
	"context"
	"fmt"

	"github.com/kurt/kurt/pkg/k8s"
	"github.com/kurt/kurt/pkg/model"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

type Builder struct {
	k8sClient     kubernetes.Interface
	dynamicClient dynamic.Interface
}

func NewBuilder(k8sClient kubernetes.Interface, dynamicClient dynamic.Interface) *Builder {
	return &Builder{
		k8sClient:     k8sClient,
		dynamicClient: dynamicClient,
	}
}

func (b *Builder) BuildDeploymentTree(ctx context.Context, namespace, name string) (*model.Node, error) {
	deployment, err := b.k8sClient.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting deployment %s/%s: %w", namespace, name, err)
	}

	root := model.NewNode(model.KindDeployment, deployment.Name, deployment.Namespace)
	root.CreatedAt = deployment.CreationTimestamp.Time

	replicaSets, err := k8s.FindOwnedReplicaSets(ctx, b.k8sClient, namespace, deployment)
	if err != nil {
		return nil, err
	}
	for _, rs := range replicaSets {
		root.AddChild(rs)
	}

	podLabels := deployment.Spec.Template.Labels
	if err := b.attachServices(ctx, root, namespace, podLabels); err != nil {
		return nil, err
	}

	if err := b.attachAuthorizationPolicies(ctx, root, namespace, podLabels); err != nil {
		return nil, err
	}

	return root, nil
}

func (b *Builder) BuildStatefulSetTree(ctx context.Context, namespace, name string) (*model.Node, error) {
	sts, err := b.k8sClient.AppsV1().StatefulSets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting statefulset %s/%s: %w", namespace, name, err)
	}

	root := model.NewNode(model.KindStatefulSet, sts.Name, sts.Namespace)
	root.CreatedAt = sts.CreationTimestamp.Time

	pods, err := k8s.FindOwnedPodsByStatefulSet(ctx, b.k8sClient, namespace, sts)
	if err != nil {
		return nil, err
	}
	for _, pod := range pods {
		root.AddChild(pod)
	}

	podLabels := sts.Spec.Template.Labels
	if err := b.attachServices(ctx, root, namespace, podLabels); err != nil {
		return nil, err
	}

	if err := b.attachAuthorizationPolicies(ctx, root, namespace, podLabels); err != nil {
		return nil, err
	}

	return root, nil
}

func (b *Builder) attachServices(ctx context.Context, root *model.Node, namespace string, podLabels map[string]string) error {
	services, err := k8s.FindServicesForLabels(ctx, b.k8sClient, namespace, podLabels)
	if err != nil {
		return err
	}

	for _, svc := range services {
		svcNode := k8s.ServiceToNode(svc)

		virtualServices, err := k8s.FindVirtualServicesForService(ctx, b.dynamicClient, namespace, svc.Name)
		if err != nil {
			return err
		}
		for _, vs := range virtualServices {
			svcNode.AddChild(vs)
		}

		destinationRules, err := k8s.FindDestinationRulesForService(ctx, b.dynamicClient, namespace, svc.Name)
		if err != nil {
			return err
		}
		for _, dr := range destinationRules {
			svcNode.AddChild(dr)
		}

		root.AddChild(svcNode)
	}

	return nil
}

func (b *Builder) attachAuthorizationPolicies(ctx context.Context, root *model.Node, namespace string, podLabels map[string]string) error {
	aps, err := k8s.FindAuthorizationPoliciesForLabels(ctx, b.dynamicClient, namespace, podLabels)
	if err != nil {
		return err
	}
	for _, ap := range aps {
		root.AddChild(ap)
	}
	return nil
}

func (b *Builder) BuildVirtualServiceTree(ctx context.Context, namespace, name string) (*model.Node, error) {
	vs, err := k8s.GetVirtualService(ctx, b.dynamicClient, namespace, name)
	if err != nil {
		return nil, err
	}

	root := model.NewNode(model.KindVirtualService, vs.GetName(), vs.GetNamespace())
	root.CreatedAt = vs.GetCreationTimestamp().Time
	root.Hosts = k8s.ExtractVirtualServiceHosts(vs)

	// Attach gateways.
	gateways, err := k8s.FindGatewaysForVirtualService(ctx, b.dynamicClient, namespace, vs)
	if err != nil {
		return nil, err
	}
	for _, gw := range gateways {
		root.AddChild(gw)
	}

	// Find destination services and attach their workloads.
	svcNames := k8s.ExtractDestinationServiceNames(vs, namespace)
	for _, svcName := range svcNames {
		svc, err := k8s.GetServiceByName(ctx, b.k8sClient, namespace, svcName)
		if err != nil {
			// Service might not exist; add a placeholder node.
			svcNode := model.NewNode(model.KindService, svcName, namespace)
			svcNode.Status = "NotFound"
			root.AddChild(svcNode)
			continue
		}

		svcNode := k8s.ServiceToNode(svc)

		// Attach DestinationRules targeting this service.
		destinationRules, err := k8s.FindDestinationRulesForService(ctx, b.dynamicClient, namespace, svc.Name)
		if err != nil {
			return nil, err
		}
		for _, dr := range destinationRules {
			svcNode.AddChild(dr)
		}
		// Find Deployments targeting this service.
		deps, err := k8s.FindDeploymentsForService(ctx, b.k8sClient, b.dynamicClient, namespace, svc)
		if err != nil {
			return nil, err
		}
		for _, dep := range deps {
			svcNode.AddChild(dep)
		}

		// Find StatefulSets targeting this service.
		stss, err := k8s.FindStatefulSetsForService(ctx, b.k8sClient, b.dynamicClient, namespace, svc)
		if err != nil {
			return nil, err
		}
		for _, sts := range stss {
			svcNode.AddChild(sts)
		}

		root.AddChild(svcNode)
	}

	return root, nil
}

func (b *Builder) BuildServiceTree(ctx context.Context, namespace, name string) (*model.Node, error) {
	svc, err := k8s.GetServiceByName(ctx, b.k8sClient, namespace, name)
	if err != nil {
		return nil, fmt.Errorf("getting service %s/%s: %w", namespace, name, err)
	}

	root := k8s.ServiceToNode(svc)

	// Attach VirtualServices that point to this service.
	virtualServices, err := k8s.FindVirtualServicesForService(ctx, b.dynamicClient, namespace, svc.Name)
	if err != nil {
		return nil, err
	}
	for _, vs := range virtualServices {
		root.AddChild(vs)
	}

	// Attach DestinationRules targeting this service.
	destinationRules, err := k8s.FindDestinationRulesForService(ctx, b.dynamicClient, namespace, svc.Name)
	if err != nil {
		return nil, err
	}
	for _, dr := range destinationRules {
		root.AddChild(dr)
	}

	// Attach Deployments whose pod template labels match the service selector.
	deps, err := k8s.FindDeploymentsForService(ctx, b.k8sClient, b.dynamicClient, namespace, svc)
	if err != nil {
		return nil, err
	}
	for _, dep := range deps {
		root.AddChild(dep)
	}

	// Attach StatefulSets whose pod template labels match the service selector.
	stss, err := k8s.FindStatefulSetsForService(ctx, b.k8sClient, b.dynamicClient, namespace, svc)
	if err != nil {
		return nil, err
	}
	for _, sts := range stss {
		root.AddChild(sts)
	}

	return root, nil
}

func (b *Builder) BuildIngressTree(ctx context.Context, namespace, name string) (*model.Node, error) {
	ing, err := k8s.GetIngressByName(ctx, b.k8sClient, namespace, name)
	if err != nil {
		return nil, err
	}

	root := k8s.IngressToNode(ing)

	// Find backend services and attach their workloads.
	svcNames := k8s.ExtractIngressServiceNames(ing)
	for _, svcName := range svcNames {
		svc, err := k8s.GetServiceByName(ctx, b.k8sClient, namespace, svcName)
		if err != nil {
			svcNode := model.NewNode(model.KindService, svcName, namespace)
			svcNode.Status = "NotFound"
			root.AddChild(svcNode)
			continue
		}

		svcNode := k8s.ServiceToNode(svc)

		// Attach VirtualServices that point to this service.
		virtualServices, err := k8s.FindVirtualServicesForService(ctx, b.dynamicClient, namespace, svc.Name)
		if err != nil {
			return nil, err
		}
		for _, vs := range virtualServices {
			svcNode.AddChild(vs)
		}

		// Attach DestinationRules targeting this service.
		destinationRules, err := k8s.FindDestinationRulesForService(ctx, b.dynamicClient, namespace, svc.Name)
		if err != nil {
			return nil, err
		}
		for _, dr := range destinationRules {
			svcNode.AddChild(dr)
		}

		// Attach Deployments.
		deps, err := k8s.FindDeploymentsForService(ctx, b.k8sClient, b.dynamicClient, namespace, svc)
		if err != nil {
			return nil, err
		}
		for _, dep := range deps {
			svcNode.AddChild(dep)
		}

		// Attach StatefulSets.
		stss, err := k8s.FindStatefulSetsForService(ctx, b.k8sClient, b.dynamicClient, namespace, svc)
		if err != nil {
			return nil, err
		}
		for _, sts := range stss {
			svcNode.AddChild(sts)
		}

		root.AddChild(svcNode)
	}

	return root, nil
}

func (b *Builder) BuildAllTrees(ctx context.Context, namespace string) ([]*model.Node, error) {
	var roots []*model.Node

	// List all Deployments.
	depList, err := b.k8sClient.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing deployments: %w", err)
	}
	for i := range depList.Items {
		dep := &depList.Items[i]
		root, err := b.BuildDeploymentTree(ctx, namespace, dep.Name)
		if err != nil {
			return nil, err
		}
		roots = append(roots, root)
	}

	// List all StatefulSets.
	stsList, err := b.k8sClient.AppsV1().StatefulSets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing statefulsets: %w", err)
	}
	for i := range stsList.Items {
		sts := &stsList.Items[i]
		root, err := b.BuildStatefulSetTree(ctx, namespace, sts.Name)
		if err != nil {
			return nil, err
		}
		roots = append(roots, root)
	}

	return roots, nil
}

func (b *Builder) BuildAuthorizationPolicyTree(ctx context.Context, namespace, name string) (*model.Node, error) {
	ap, err := k8s.GetAuthorizationPolicy(ctx, b.dynamicClient, namespace, name)
	if err != nil {
		return nil, err
	}

	root := k8s.AuthorizationPolicyToNode(ap)

	selectorLabels := k8s.ExtractAuthorizationPolicySelectorLabels(ap)

	// Find Deployments matching the AP selector (or all if namespace-wide).
	depList, err := b.k8sClient.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing deployments: %w", err)
	}
	for i := range depList.Items {
		dep := &depList.Items[i]
		if !labelsMatch(selectorLabels, dep.Spec.Template.Labels) {
			continue
		}
		depNode := model.NewNode(model.KindDeployment, dep.Name, dep.Namespace)
		depNode.CreatedAt = dep.CreationTimestamp.Time

		replicaSets, err := k8s.FindOwnedReplicaSets(ctx, b.k8sClient, namespace, dep)
		if err != nil {
			return nil, err
		}
		for _, rs := range replicaSets {
			depNode.AddChild(rs)
		}

		root.AddChild(depNode)
	}

	// Find StatefulSets matching the AP selector (or all if namespace-wide).
	stsList, err := b.k8sClient.AppsV1().StatefulSets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing statefulsets: %w", err)
	}
	for i := range stsList.Items {
		sts := &stsList.Items[i]
		if !labelsMatch(selectorLabels, sts.Spec.Template.Labels) {
			continue
		}
		stsNode := model.NewNode(model.KindStatefulSet, sts.Name, sts.Namespace)
		stsNode.CreatedAt = sts.CreationTimestamp.Time

		pods, err := k8s.FindOwnedPodsByStatefulSet(ctx, b.k8sClient, namespace, sts)
		if err != nil {
			return nil, err
		}
		for _, pod := range pods {
			stsNode.AddChild(pod)
		}

		root.AddChild(stsNode)
	}

	return root, nil
}

// labelsMatch returns true if selectorLabels is nil (namespace-wide) or all
// entries in selectorLabels exist in targetLabels.
func labelsMatch(selectorLabels, targetLabels map[string]string) bool {
	if len(selectorLabels) == 0 {
		return true
	}
	for k, v := range selectorLabels {
		if targetLabels[k] != v {
			return false
		}
	}
	return true
}
