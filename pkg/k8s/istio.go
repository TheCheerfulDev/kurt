package k8s

import (
	"context"
	"fmt"
	"strings"

	"github.com/kurt/kurt/pkg/model"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

var (
	virtualServiceGVR = schema.GroupVersionResource{
		Group:    "networking.istio.io",
		Version:  "v1",
		Resource: "virtualservices",
	}
	gatewayGVR = schema.GroupVersionResource{
		Group:    "networking.istio.io",
		Version:  "v1",
		Resource: "gateways",
	}
	authorizationPolicyGVR = schema.GroupVersionResource{
		Group:    "security.istio.io",
		Version:  "v1",
		Resource: "authorizationpolicies",
	}
	destinationRuleGVR = schema.GroupVersionResource{
		Group:    "networking.istio.io",
		Version:  "v1",
		Resource: "destinationrules",
	}
)

func FindVirtualServicesForService(ctx context.Context, client dynamic.Interface, namespace, serviceName string) ([]*model.Node, error) {
	vsList, err := client.Resource(virtualServiceGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		if isNotFoundOrCRDMissing(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("listing virtualservices: %w", err)
	}

	var nodes []*model.Node
	for i := range vsList.Items {
		vs := &vsList.Items[i]
		if !virtualServiceRoutesToService(vs, serviceName, namespace) {
			continue
		}

		vsNode := model.NewNode(model.KindVirtualService, vs.GetName(), vs.GetNamespace())
		vsNode.CreatedAt = vs.GetCreationTimestamp().Time
		vsNode.Hosts = ExtractVirtualServiceHosts(vs)

		gateways, err := FindGatewaysForVirtualService(ctx, client, namespace, vs)
		if err != nil {
			return nil, err
		}
		for _, gw := range gateways {
			vsNode.AddChild(gw)
		}

		nodes = append(nodes, vsNode)
	}
	return nodes, nil
}

func FindGatewaysForVirtualService(ctx context.Context, client dynamic.Interface, namespace string, vs *unstructured.Unstructured) ([]*model.Node, error) {
	gateways, found, err := unstructured.NestedStringSlice(vs.Object, "spec", "gateways")
	if err != nil || !found {
		return nil, nil
	}

	var nodes []*model.Node
	for _, gwRef := range gateways {
		if gwRef == "mesh" {
			continue
		}

		gwNamespace, gwName := parseGatewayRef(gwRef, namespace)
		gw, err := client.Resource(gatewayGVR).Namespace(gwNamespace).Get(ctx, gwName, metav1.GetOptions{})
		if err != nil {
			n := model.NewNode(model.KindGateway, gwRef, gwNamespace)
			n.Status = "NotFound"
			nodes = append(nodes, n)
			continue
		}

		n := model.NewNode(model.KindGateway, gw.GetName(), gw.GetNamespace())
		n.CreatedAt = gw.GetCreationTimestamp().Time
		nodes = append(nodes, n)
	}
	return nodes, nil
}

func virtualServiceRoutesToService(vs *unstructured.Unstructured, serviceName, namespace string) bool {
	for _, protocol := range []string{"http", "tcp", "tls"} {
		routes, found, _ := unstructured.NestedSlice(vs.Object, "spec", protocol)
		if found && routeSliceMatchesService(routes, serviceName, namespace) {
			return true
		}
	}
	return false
}

func routeSliceMatchesService(routes []interface{}, serviceName, namespace string) bool {
	for _, r := range routes {
		route, ok := r.(map[string]interface{})
		if !ok {
			continue
		}
		routeEntries, found, _ := unstructured.NestedSlice(route, "route")
		if !found {
			continue
		}
		for _, re := range routeEntries {
			entry, ok := re.(map[string]interface{})
			if !ok {
				continue
			}
			host, found, _ := unstructured.NestedString(entry, "destination", "host")
			if !found {
				continue
			}
			if matchesServiceHost(host, serviceName, namespace) {
				return true
			}
		}
	}
	return false
}

// Supports short name, namespace-qualified, and FQDN:
// "my-svc", "my-svc.ns", "my-svc.ns.svc.cluster.local"
func matchesServiceHost(host, serviceName, namespace string) bool {
	if host == serviceName {
		return true
	}
	if host == serviceName+"."+namespace {
		return true
	}
	if host == serviceName+"."+namespace+".svc.cluster.local" {
		return true
	}
	return false
}

func parseGatewayRef(ref, defaultNamespace string) (namespace, name string) {
	parts := strings.SplitN(ref, "/", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return defaultNamespace, ref
}

func isNotFoundOrCRDMissing(err error) bool {
	errStr := err.Error()
	return strings.Contains(errStr, "not found") ||
		strings.Contains(errStr, "the server could not find the requested resource") ||
		strings.Contains(errStr, "no matches for kind")
}

// GetVirtualService fetches a single VirtualService by name as an unstructured object.
func GetVirtualService(ctx context.Context, client dynamic.Interface, namespace, name string) (*unstructured.Unstructured, error) {
	vs, err := client.Resource(virtualServiceGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting virtualservice %s/%s: %w", namespace, name, err)
	}
	return vs, nil
}

// ExtractDestinationServiceNames returns the unique set of destination service
// names (short names resolved against the VS namespace) from a VirtualService.
func ExtractDestinationServiceNames(vs *unstructured.Unstructured, namespace string) []string {
	seen := make(map[string]bool)
	var names []string

	for _, protocol := range []string{"http", "tcp", "tls"} {
		routes, found, _ := unstructured.NestedSlice(vs.Object, "spec", protocol)
		if !found {
			continue
		}
		for _, r := range routes {
			route, ok := r.(map[string]interface{})
			if !ok {
				continue
			}
			routeEntries, found, _ := unstructured.NestedSlice(route, "route")
			if !found {
				continue
			}
			for _, re := range routeEntries {
				entry, ok := re.(map[string]interface{})
				if !ok {
					continue
				}
				host, found, _ := unstructured.NestedString(entry, "destination", "host")
				if !found || host == "" {
					continue
				}
				svcName := resolveServiceName(host, namespace)
				if !seen[svcName] {
					seen[svcName] = true
					names = append(names, svcName)
				}
			}
		}
	}
	return names
}

// resolveServiceName extracts the short service name from a host reference.
// Handles: "my-svc", "my-svc.ns", "my-svc.ns.svc.cluster.local"
func resolveServiceName(host, defaultNamespace string) string {
	// Strip .svc.cluster.local suffix if present.
	host = strings.TrimSuffix(host, ".svc.cluster.local")
	// Strip namespace suffix if present.
	host = strings.TrimSuffix(host, "."+defaultNamespace)
	return host
}

// GetAuthorizationPolicy fetches a single AuthorizationPolicy by name.
func GetAuthorizationPolicy(ctx context.Context, client dynamic.Interface, namespace, name string) (*unstructured.Unstructured, error) {
	ap, err := client.Resource(authorizationPolicyGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting authorizationpolicy %s/%s: %w", namespace, name, err)
	}
	return ap, nil
}

// AuthorizationPolicyToNode converts an unstructured AuthorizationPolicy to a model Node.
// The action (ALLOW/DENY/AUDIT/CUSTOM) is shown in the Status field.
func AuthorizationPolicyToNode(ap *unstructured.Unstructured) *model.Node {
	n := model.NewNode(model.KindAuthorizationPolicy, ap.GetName(), ap.GetNamespace())
	n.CreatedAt = ap.GetCreationTimestamp().Time
	action, _, _ := unstructured.NestedString(ap.Object, "spec", "action")
	if action != "" {
		n.Status = action
	}
	return n
}

// FindAuthorizationPoliciesForLabels returns AuthorizationPolicy nodes whose
// spec.selector.matchLabels match the given pod labels, plus namespace-wide
// policies (those without a selector).
func FindAuthorizationPoliciesForLabels(ctx context.Context, client dynamic.Interface, namespace string, podLabels map[string]string) ([]*model.Node, error) {
	apList, err := client.Resource(authorizationPolicyGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		if isNotFoundOrCRDMissing(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("listing authorizationpolicies: %w", err)
	}

	var nodes []*model.Node
	for i := range apList.Items {
		ap := &apList.Items[i]
		if !authorizationPolicyMatchesLabels(ap, podLabels) {
			continue
		}
		nodes = append(nodes, AuthorizationPolicyToNode(ap))
	}
	return nodes, nil
}

// authorizationPolicyMatchesLabels returns true if the AP has no selector
// (namespace-wide) or if all matchLabels entries exist in podLabels.
func authorizationPolicyMatchesLabels(ap *unstructured.Unstructured, podLabels map[string]string) bool {
	selectorMap, found, _ := unstructured.NestedStringMap(ap.Object, "spec", "selector", "matchLabels")
	if !found || len(selectorMap) == 0 {
		// No selector means namespace-wide policy — matches everything.
		return true
	}
	for k, v := range selectorMap {
		if podLabels[k] != v {
			return false
		}
	}
	return true
}

// ExtractAuthorizationPolicySelectorLabels returns the matchLabels from the AP's
// spec.selector, or nil if there is no selector (namespace-wide policy).
func ExtractAuthorizationPolicySelectorLabels(ap *unstructured.Unstructured) map[string]string {
	selectorMap, found, _ := unstructured.NestedStringMap(ap.Object, "spec", "selector", "matchLabels")
	if !found || len(selectorMap) == 0 {
		return nil
	}
	return selectorMap
}

// ExtractVirtualServiceHosts returns a comma-separated string of hosts from
// the VirtualService spec.hosts field.
func ExtractVirtualServiceHosts(vs *unstructured.Unstructured) string {
	hosts, found, _ := unstructured.NestedStringSlice(vs.Object, "spec", "hosts")
	if !found || len(hosts) == 0 {
		return ""
	}
	return strings.Join(hosts, ",")
}

// FindDestinationRulesForService returns DestinationRule nodes whose spec.host
// matches the given service name (short name, namespace-qualified, or FQDN).
func FindDestinationRulesForService(ctx context.Context, client dynamic.Interface, namespace, serviceName string) ([]*model.Node, error) {
	drList, err := client.Resource(destinationRuleGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		if isNotFoundOrCRDMissing(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("listing destinationrules: %w", err)
	}

	var nodes []*model.Node
	for i := range drList.Items {
		dr := &drList.Items[i]
		host, _, _ := unstructured.NestedString(dr.Object, "spec", "host")
		if host == "" || !matchesServiceHost(host, serviceName, namespace) {
			continue
		}
		nodes = append(nodes, DestinationRuleToNode(dr))
	}
	return nodes, nil
}

// DestinationRuleToNode converts an unstructured DestinationRule to a model Node.
// The mTLS mode (MUTUAL, ISTIO_MUTUAL, PERMISSIVE, DISABLE, SIMPLE) is shown in the Status field.
func DestinationRuleToNode(dr *unstructured.Unstructured) *model.Node {
	n := model.NewNode(model.KindDestinationRule, dr.GetName(), dr.GetNamespace())
	n.CreatedAt = dr.GetCreationTimestamp().Time
	mode, _, _ := unstructured.NestedString(dr.Object, "spec", "trafficPolicy", "tls", "mode")
	if mode != "" {
		n.Status = mode
	}
	return n
}
