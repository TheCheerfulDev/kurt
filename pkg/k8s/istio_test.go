package k8s

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/kurt/kurt/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

// newFakeDynamicClient creates a fake dynamic client with optional objects.
func newFakeDynamicClient(objects ...runtime.Object) *dynamicfake.FakeDynamicClient {
	scheme := runtime.NewScheme()
	// Register GVKs for Istio resources so the fake client can handle List operations.
	scheme.AddKnownTypeWithName(schema.GroupVersionKind{Group: "networking.istio.io", Version: "v1", Kind: "VirtualService"}, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(schema.GroupVersionKind{Group: "networking.istio.io", Version: "v1", Kind: "VirtualServiceList"}, &unstructured.UnstructuredList{})
	scheme.AddKnownTypeWithName(schema.GroupVersionKind{Group: "networking.istio.io", Version: "v1", Kind: "Gateway"}, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(schema.GroupVersionKind{Group: "networking.istio.io", Version: "v1", Kind: "GatewayList"}, &unstructured.UnstructuredList{})
	scheme.AddKnownTypeWithName(schema.GroupVersionKind{Group: "security.istio.io", Version: "v1", Kind: "AuthorizationPolicy"}, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(schema.GroupVersionKind{Group: "security.istio.io", Version: "v1", Kind: "AuthorizationPolicyList"}, &unstructured.UnstructuredList{})
	scheme.AddKnownTypeWithName(schema.GroupVersionKind{Group: "networking.istio.io", Version: "v1", Kind: "DestinationRule"}, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(schema.GroupVersionKind{Group: "networking.istio.io", Version: "v1", Kind: "DestinationRuleList"}, &unstructured.UnstructuredList{})

	return dynamicfake.NewSimpleDynamicClient(scheme, objects...)
}

func makeVirtualService(name, namespace string, hosts []interface{}, httpRoutes []interface{}) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "networking.istio.io/v1",
			"kind":       "VirtualService",
			"metadata": map[string]interface{}{
				"name":              name,
				"namespace":         namespace,
				"creationTimestamp": time.Now().Format(time.RFC3339),
			},
			"spec": map[string]interface{}{},
		},
	}
	if hosts != nil {
		_ = unstructured.SetNestedSlice(obj.Object, hosts, "spec", "hosts")
	}
	if httpRoutes != nil {
		_ = unstructured.SetNestedSlice(obj.Object, httpRoutes, "spec", "http")
	}
	return obj
}

func makeGateway(name, namespace string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "networking.istio.io/v1",
			"kind":       "Gateway",
			"metadata": map[string]interface{}{
				"name":              name,
				"namespace":         namespace,
				"creationTimestamp": time.Now().Format(time.RFC3339),
			},
			"spec": map[string]interface{}{},
		},
	}
}

func makeAuthorizationPolicy(name, namespace, action string, matchLabels map[string]interface{}) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "security.istio.io/v1",
			"kind":       "AuthorizationPolicy",
			"metadata": map[string]interface{}{
				"name":              name,
				"namespace":         namespace,
				"creationTimestamp": time.Now().Format(time.RFC3339),
			},
			"spec": map[string]interface{}{},
		},
	}
	if action != "" {
		_ = unstructured.SetNestedField(obj.Object, action, "spec", "action")
	}
	if matchLabels != nil {
		_ = unstructured.SetNestedField(obj.Object, matchLabels, "spec", "selector", "matchLabels")
	}
	return obj
}

func makeDestinationRule(name, namespace, host, tlsMode string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "networking.istio.io/v1",
			"kind":       "DestinationRule",
			"metadata": map[string]interface{}{
				"name":              name,
				"namespace":         namespace,
				"creationTimestamp": time.Now().Format(time.RFC3339),
			},
			"spec": map[string]interface{}{
				"host": host,
			},
		},
	}
	if tlsMode != "" {
		_ = unstructured.SetNestedField(obj.Object, tlsMode, "spec", "trafficPolicy", "tls", "mode")
	}
	return obj
}

// ---------- matchesServiceHost tests ----------

func TestMatchesServiceHost_ShortName(t *testing.T) {
	assert.True(t, matchesServiceHost("my-svc", "my-svc", "default"))
}

func TestMatchesServiceHost_NamespaceQualified(t *testing.T) {
	assert.True(t, matchesServiceHost("my-svc.default", "my-svc", "default"))
}

func TestMatchesServiceHost_FQDN(t *testing.T) {
	assert.True(t, matchesServiceHost("my-svc.default.svc.cluster.local", "my-svc", "default"))
}

func TestMatchesServiceHost_NoMatch(t *testing.T) {
	assert.False(t, matchesServiceHost("other-svc", "my-svc", "default"))
}

func TestMatchesServiceHost_DifferentNamespace(t *testing.T) {
	assert.False(t, matchesServiceHost("my-svc.other-ns", "my-svc", "default"))
}

// ---------- parseGatewayRef tests ----------

func TestParseGatewayRef_WithNamespace(t *testing.T) {
	ns, name := parseGatewayRef("istio-system/my-gw", "default")
	assert.Equal(t, "istio-system", ns)
	assert.Equal(t, "my-gw", name)
}

func TestParseGatewayRef_WithoutNamespace(t *testing.T) {
	ns, name := parseGatewayRef("my-gw", "default")
	assert.Equal(t, "default", ns)
	assert.Equal(t, "my-gw", name)
}

// ---------- resolveServiceName tests ----------

func TestResolveServiceName_ShortName(t *testing.T) {
	assert.Equal(t, "my-svc", resolveServiceName("my-svc", "default"))
}

func TestResolveServiceName_FQDN(t *testing.T) {
	assert.Equal(t, "my-svc", resolveServiceName("my-svc.default.svc.cluster.local", "default"))
}

func TestResolveServiceName_NamespaceQualified(t *testing.T) {
	assert.Equal(t, "my-svc", resolveServiceName("my-svc.default", "default"))
}

// ---------- isNotFoundOrCRDMissing tests ----------

func TestIsNotFoundOrCRDMissing_NotFound(t *testing.T) {
	err := fmt.Errorf("resource not found")
	assert.True(t, isNotFoundOrCRDMissing(err))
}

func TestIsNotFoundOrCRDMissing_CRDMissing(t *testing.T) {
	err := fmt.Errorf("the server could not find the requested resource")
	assert.True(t, isNotFoundOrCRDMissing(err))
}

func TestIsNotFoundOrCRDMissing_NoMatchesForKind(t *testing.T) {
	err := fmt.Errorf("no matches for kind \"VirtualService\"")
	assert.True(t, isNotFoundOrCRDMissing(err))
}

func TestIsNotFoundOrCRDMissing_OtherError(t *testing.T) {
	err := fmt.Errorf("connection refused")
	assert.False(t, isNotFoundOrCRDMissing(err))
}

// ---------- ExtractVirtualServiceHosts tests ----------

func TestExtractVirtualServiceHosts_WithHosts(t *testing.T) {
	vs := makeVirtualService("my-vs", "default", []interface{}{"app.example.com", "api.example.com"}, nil)
	assert.Equal(t, "app.example.com,api.example.com", ExtractVirtualServiceHosts(vs))
}

func TestExtractVirtualServiceHosts_NoHosts(t *testing.T) {
	vs := makeVirtualService("my-vs", "default", nil, nil)
	assert.Equal(t, "", ExtractVirtualServiceHosts(vs))
}

func TestExtractVirtualServiceHosts_EmptyHosts(t *testing.T) {
	vs := makeVirtualService("my-vs", "default", []interface{}{}, nil)
	assert.Equal(t, "", ExtractVirtualServiceHosts(vs))
}

// ---------- ExtractDestinationServiceNames tests ----------

func TestExtractDestinationServiceNames_HTTP(t *testing.T) {
	httpRoutes := []interface{}{
		map[string]interface{}{
			"route": []interface{}{
				map[string]interface{}{
					"destination": map[string]interface{}{
						"host": "my-svc",
					},
				},
			},
		},
	}
	vs := makeVirtualService("my-vs", "default", nil, httpRoutes)
	names := ExtractDestinationServiceNames(vs, "default")
	require.Len(t, names, 1)
	assert.Equal(t, "my-svc", names[0])
}

func TestExtractDestinationServiceNames_FQDN(t *testing.T) {
	httpRoutes := []interface{}{
		map[string]interface{}{
			"route": []interface{}{
				map[string]interface{}{
					"destination": map[string]interface{}{
						"host": "my-svc.default.svc.cluster.local",
					},
				},
			},
		},
	}
	vs := makeVirtualService("my-vs", "default", nil, httpRoutes)
	names := ExtractDestinationServiceNames(vs, "default")
	require.Len(t, names, 1)
	assert.Equal(t, "my-svc", names[0])
}

func TestExtractDestinationServiceNames_Dedup(t *testing.T) {
	httpRoutes := []interface{}{
		map[string]interface{}{
			"route": []interface{}{
				map[string]interface{}{
					"destination": map[string]interface{}{"host": "my-svc"},
				},
				map[string]interface{}{
					"destination": map[string]interface{}{"host": "my-svc"},
				},
			},
		},
	}
	vs := makeVirtualService("my-vs", "default", nil, httpRoutes)
	names := ExtractDestinationServiceNames(vs, "default")
	assert.Len(t, names, 1)
}

// ---------- AuthorizationPolicyToNode tests ----------

func TestAuthorizationPolicyToNode_WithAction(t *testing.T) {
	ap := makeAuthorizationPolicy("allow-all", "default", "ALLOW", nil)
	n := AuthorizationPolicyToNode(ap)
	assert.Equal(t, model.KindAuthorizationPolicy, n.Kind)
	assert.Equal(t, "allow-all", n.Name)
	assert.Equal(t, "ALLOW", n.Status)
}

func TestAuthorizationPolicyToNode_NoAction(t *testing.T) {
	ap := makeAuthorizationPolicy("no-action", "default", "", nil)
	n := AuthorizationPolicyToNode(ap)
	assert.Equal(t, "-", n.Status) // default from NewNode
}

// ---------- DestinationRuleToNode tests ----------

func TestDestinationRuleToNode_WithTLS(t *testing.T) {
	dr := makeDestinationRule("my-dr", "default", "my-svc", "MUTUAL")
	n := DestinationRuleToNode(dr)
	assert.Equal(t, model.KindDestinationRule, n.Kind)
	assert.Equal(t, "my-dr", n.Name)
	assert.Equal(t, "MUTUAL", n.Status)
}

func TestDestinationRuleToNode_NoTLS(t *testing.T) {
	dr := makeDestinationRule("my-dr", "default", "my-svc", "")
	n := DestinationRuleToNode(dr)
	assert.Equal(t, "-", n.Status)
}

// ---------- authorizationPolicyMatchesLabels tests ----------

func TestAuthorizationPolicyMatchesLabels_ExactMatch(t *testing.T) {
	ap := makeAuthorizationPolicy("test", "default", "ALLOW", map[string]interface{}{"app": "my-app"})
	assert.True(t, authorizationPolicyMatchesLabels(ap, map[string]string{"app": "my-app", "version": "v1"}))
}

func TestAuthorizationPolicyMatchesLabels_NoSelector(t *testing.T) {
	ap := makeAuthorizationPolicy("test", "default", "ALLOW", nil)
	assert.True(t, authorizationPolicyMatchesLabels(ap, map[string]string{"app": "anything"}))
}

func TestAuthorizationPolicyMatchesLabels_PartialMismatch(t *testing.T) {
	ap := makeAuthorizationPolicy("test", "default", "ALLOW", map[string]interface{}{"app": "my-app"})
	assert.False(t, authorizationPolicyMatchesLabels(ap, map[string]string{"app": "other-app"}))
}

func TestAuthorizationPolicyMatchesLabels_MissingLabel(t *testing.T) {
	ap := makeAuthorizationPolicy("test", "default", "ALLOW", map[string]interface{}{"app": "my-app"})
	assert.False(t, authorizationPolicyMatchesLabels(ap, map[string]string{"version": "v1"}))
}

// ---------- ExtractAuthorizationPolicySelectorLabels tests ----------

func TestExtractAuthorizationPolicySelectorLabels_WithSelector(t *testing.T) {
	ap := makeAuthorizationPolicy("test", "default", "ALLOW", map[string]interface{}{"app": "my-app"})
	labels := ExtractAuthorizationPolicySelectorLabels(ap)
	assert.Equal(t, map[string]string{"app": "my-app"}, labels)
}

func TestExtractAuthorizationPolicySelectorLabels_NoSelector(t *testing.T) {
	ap := makeAuthorizationPolicy("test", "default", "ALLOW", nil)
	labels := ExtractAuthorizationPolicySelectorLabels(ap)
	assert.Nil(t, labels)
}

// ---------- FindVirtualServicesForService tests (fake dynamic client) ----------

func TestFindVirtualServicesForService_Found(t *testing.T) {
	ctx := context.Background()

	vs := makeVirtualService("my-vs", "default",
		[]interface{}{"app.example.com"},
		[]interface{}{
			map[string]interface{}{
				"route": []interface{}{
					map[string]interface{}{
						"destination": map[string]interface{}{"host": "my-svc"},
					},
				},
			},
		})

	gw := makeGateway("my-gw", "default")

	// Add gateways to VS spec.
	_ = unstructured.SetNestedStringSlice(vs.Object, []string{"my-gw"}, "spec", "gateways")

	client := newFakeDynamicClient(vs, gw)

	nodes, err := FindVirtualServicesForService(ctx, client, "default", "my-svc")
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	assert.Equal(t, model.KindVirtualService, nodes[0].Kind)
	assert.Equal(t, "my-vs", nodes[0].Name)
	assert.Equal(t, "app.example.com", nodes[0].Hosts)

	// Should have gateway child.
	require.Len(t, nodes[0].Children, 1)
	assert.Equal(t, model.KindGateway, nodes[0].Children[0].Kind)
	assert.Equal(t, "my-gw", nodes[0].Children[0].Name)
}

func TestFindVirtualServicesForService_NoMatch(t *testing.T) {
	ctx := context.Background()

	vs := makeVirtualService("my-vs", "default",
		nil,
		[]interface{}{
			map[string]interface{}{
				"route": []interface{}{
					map[string]interface{}{
						"destination": map[string]interface{}{"host": "other-svc"},
					},
				},
			},
		})

	client := newFakeDynamicClient(vs)

	nodes, err := FindVirtualServicesForService(ctx, client, "default", "my-svc")
	require.NoError(t, err)
	assert.Empty(t, nodes)
}

// ---------- FindGatewaysForVirtualService tests ----------

func TestFindGatewaysForVirtualService_Found(t *testing.T) {
	ctx := context.Background()

	vs := makeVirtualService("my-vs", "default", nil, nil)
	_ = unstructured.SetNestedStringSlice(vs.Object, []string{"my-gw"}, "spec", "gateways")

	gw := makeGateway("my-gw", "default")
	client := newFakeDynamicClient(gw)

	nodes, err := FindGatewaysForVirtualService(ctx, client, "default", vs)
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	assert.Equal(t, model.KindGateway, nodes[0].Kind)
	assert.Equal(t, "my-gw", nodes[0].Name)
}

func TestFindGatewaysForVirtualService_NotFound(t *testing.T) {
	ctx := context.Background()

	vs := makeVirtualService("my-vs", "default", nil, nil)
	_ = unstructured.SetNestedStringSlice(vs.Object, []string{"missing-gw"}, "spec", "gateways")

	client := newFakeDynamicClient() // no gateways

	nodes, err := FindGatewaysForVirtualService(ctx, client, "default", vs)
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	assert.Equal(t, "NotFound", nodes[0].Status)
}

func TestFindGatewaysForVirtualService_MeshSkipped(t *testing.T) {
	ctx := context.Background()

	vs := makeVirtualService("my-vs", "default", nil, nil)
	_ = unstructured.SetNestedStringSlice(vs.Object, []string{"mesh"}, "spec", "gateways")

	client := newFakeDynamicClient()

	nodes, err := FindGatewaysForVirtualService(ctx, client, "default", vs)
	require.NoError(t, err)
	assert.Empty(t, nodes)
}

func TestFindGatewaysForVirtualService_CrossNamespace(t *testing.T) {
	ctx := context.Background()

	vs := makeVirtualService("my-vs", "default", nil, nil)
	_ = unstructured.SetNestedStringSlice(vs.Object, []string{"istio-system/shared-gw"}, "spec", "gateways")

	gw := makeGateway("shared-gw", "istio-system")
	client := newFakeDynamicClient()
	// The fake dynamic client constructor doesn't reliably track cross-namespace objects,
	// so we Create the gateway explicitly in the istio-system namespace.
	_, err := client.Resource(schema.GroupVersionResource{Group: "networking.istio.io", Version: "v1", Resource: "gateways"}).Namespace("istio-system").Create(ctx, gw, metav1.CreateOptions{})
	require.NoError(t, err)

	nodes, err := FindGatewaysForVirtualService(ctx, client, "default", vs)
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	assert.Equal(t, "shared-gw", nodes[0].Name)
	assert.Equal(t, "istio-system", nodes[0].Namespace)
}

func TestFindGatewaysForVirtualService_NoGateways(t *testing.T) {
	ctx := context.Background()

	vs := makeVirtualService("my-vs", "default", nil, nil)
	// No spec.gateways set.

	client := newFakeDynamicClient()

	nodes, err := FindGatewaysForVirtualService(ctx, client, "default", vs)
	require.NoError(t, err)
	assert.Empty(t, nodes)
}

// ---------- FindDestinationRulesForService tests ----------

func TestFindDestinationRulesForService_Found(t *testing.T) {
	ctx := context.Background()

	dr := makeDestinationRule("my-dr", "default", "my-svc", "MUTUAL")
	client := newFakeDynamicClient(dr)

	nodes, err := FindDestinationRulesForService(ctx, client, "default", "my-svc")
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	assert.Equal(t, model.KindDestinationRule, nodes[0].Kind)
	assert.Equal(t, "my-dr", nodes[0].Name)
	assert.Equal(t, "MUTUAL", nodes[0].Status)
}

func TestFindDestinationRulesForService_NoMatch(t *testing.T) {
	ctx := context.Background()

	dr := makeDestinationRule("my-dr", "default", "other-svc", "MUTUAL")
	client := newFakeDynamicClient(dr)

	nodes, err := FindDestinationRulesForService(ctx, client, "default", "my-svc")
	require.NoError(t, err)
	assert.Empty(t, nodes)
}

func TestFindDestinationRulesForService_FQDN(t *testing.T) {
	ctx := context.Background()

	dr := makeDestinationRule("my-dr", "default", "my-svc.default.svc.cluster.local", "ISTIO_MUTUAL")
	client := newFakeDynamicClient(dr)

	nodes, err := FindDestinationRulesForService(ctx, client, "default", "my-svc")
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	assert.Equal(t, "ISTIO_MUTUAL", nodes[0].Status)
}

// ---------- FindAuthorizationPoliciesForLabels tests ----------

func TestFindAuthorizationPoliciesForLabels_MatchingSelector(t *testing.T) {
	ctx := context.Background()

	ap := makeAuthorizationPolicy("allow-app", "default", "ALLOW", map[string]interface{}{"app": "my-app"})
	client := newFakeDynamicClient(ap)

	nodes, err := FindAuthorizationPoliciesForLabels(ctx, client, "default", map[string]string{"app": "my-app"})
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	assert.Equal(t, "allow-app", nodes[0].Name)
	assert.Equal(t, "ALLOW", nodes[0].Status)
}

func TestFindAuthorizationPoliciesForLabels_NamespaceWide(t *testing.T) {
	ctx := context.Background()

	ap := makeAuthorizationPolicy("namespace-policy", "default", "DENY", nil)
	client := newFakeDynamicClient(ap)

	nodes, err := FindAuthorizationPoliciesForLabels(ctx, client, "default", map[string]string{"app": "anything"})
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	assert.Equal(t, "namespace-policy", nodes[0].Name)
}

func TestFindAuthorizationPoliciesForLabels_NoMatch(t *testing.T) {
	ctx := context.Background()

	ap := makeAuthorizationPolicy("allow-app", "default", "ALLOW", map[string]interface{}{"app": "other-app"})
	client := newFakeDynamicClient(ap)

	nodes, err := FindAuthorizationPoliciesForLabels(ctx, client, "default", map[string]string{"app": "my-app"})
	require.NoError(t, err)
	assert.Empty(t, nodes)
}
