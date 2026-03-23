package k8s

import (
	"context"
	"fmt"
	"strings"

	"github.com/kurt/kurt/pkg/model"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// GetIngressByName fetches a single Ingress by name.
func GetIngressByName(ctx context.Context, client kubernetes.Interface, namespace, name string) (*networkingv1.Ingress, error) {
	ing, err := client.NetworkingV1().Ingresses(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting ingress %s/%s: %w", namespace, name, err)
	}
	return ing, nil
}

// IngressToNode converts an Ingress to a model Node.
func IngressToNode(ing *networkingv1.Ingress) *model.Node {
	n := model.NewNode(model.KindIngress, ing.Name, ing.Namespace)
	n.CreatedAt = ing.CreationTimestamp.Time
	n.Hosts = extractIngressHosts(ing)
	return n
}

// ExtractIngressServiceNames returns the unique set of backend service names
// referenced by an Ingress (both default backend and rule paths).
func ExtractIngressServiceNames(ing *networkingv1.Ingress) []string {
	seen := make(map[string]bool)
	var names []string

	add := func(name string) {
		if name != "" && !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}

	// Default backend.
	if ing.Spec.DefaultBackend != nil && ing.Spec.DefaultBackend.Service != nil {
		add(ing.Spec.DefaultBackend.Service.Name)
	}

	// Rule-based backends.
	for _, rule := range ing.Spec.Rules {
		if rule.HTTP == nil {
			continue
		}
		for _, path := range rule.HTTP.Paths {
			if path.Backend.Service != nil {
				add(path.Backend.Service.Name)
			}
		}
	}

	return names
}

// extractIngressHosts returns a comma-separated string of hosts from the
// Ingress spec.rules.
func extractIngressHosts(ing *networkingv1.Ingress) string {
	var hosts []string
	seen := make(map[string]bool)
	for _, rule := range ing.Spec.Rules {
		if rule.Host != "" && !seen[rule.Host] {
			seen[rule.Host] = true
			hosts = append(hosts, rule.Host)
		}
	}
	if len(hosts) == 0 {
		return ""
	}
	return strings.Join(hosts, ",")
}

// ListIngresses lists all Ingresses in a namespace.
func ListIngresses(ctx context.Context, client kubernetes.Interface, namespace string) ([]*networkingv1.Ingress, error) {
	list, err := client.NetworkingV1().Ingresses(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing ingresses: %w", err)
	}
	result := make([]*networkingv1.Ingress, len(list.Items))
	for i := range list.Items {
		result[i] = &list.Items[i]
	}
	return result, nil
}
