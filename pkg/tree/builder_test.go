package tree

import (
	"context"
	"testing"
	"time"

	"github.com/kurt/kurt/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
)

// newFakeDynamicClient creates a fake dynamic client with Istio CRD types registered.
func newFakeDynamicClient(objects ...runtime.Object) *dynamicfake.FakeDynamicClient {
	scheme := runtime.NewScheme()
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

func makeVS(name, namespace, destHost string, gateways []string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "networking.istio.io/v1",
			"kind":       "VirtualService",
			"metadata": map[string]interface{}{
				"name":              name,
				"namespace":         namespace,
				"creationTimestamp": time.Now().Format(time.RFC3339),
			},
			"spec": map[string]interface{}{
				"http": []interface{}{
					map[string]interface{}{
						"route": []interface{}{
							map[string]interface{}{
								"destination": map[string]interface{}{
									"host": destHost,
								},
							},
						},
					},
				},
			},
		},
	}
	if len(gateways) > 0 {
		gwSlice := make([]string, len(gateways))
		copy(gwSlice, gateways)
		_ = unstructured.SetNestedStringSlice(obj.Object, gwSlice, "spec", "gateways")
	}
	return obj
}

func makeGW(name, namespace string) *unstructured.Unstructured {
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

func makeAP(name, namespace, action string, matchLabels map[string]interface{}) *unstructured.Unstructured {
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

func makeDR(name, namespace, host, mode string) *unstructured.Unstructured {
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
	if mode != "" {
		_ = unstructured.SetNestedField(obj.Object, mode, "spec", "trafficPolicy", "tls", "mode")
	}
	return obj
}

// ---------- labelsMatch tests ----------

func TestLabelsMatch_NilSelector(t *testing.T) {
	assert.True(t, labelsMatch(nil, map[string]string{"app": "my-app"}))
}

func TestLabelsMatch_EmptySelector(t *testing.T) {
	assert.True(t, labelsMatch(map[string]string{}, map[string]string{"app": "my-app"}))
}

func TestLabelsMatch_Match(t *testing.T) {
	assert.True(t, labelsMatch(
		map[string]string{"app": "my-app"},
		map[string]string{"app": "my-app", "version": "v1"},
	))
}

func TestLabelsMatch_NoMatch(t *testing.T) {
	assert.False(t, labelsMatch(
		map[string]string{"app": "my-app"},
		map[string]string{"app": "other-app"},
	))
}

func TestLabelsMatch_MissingKey(t *testing.T) {
	assert.False(t, labelsMatch(
		map[string]string{"app": "my-app"},
		map[string]string{"version": "v1"},
	))
}

// ---------- BuildDeploymentTree tests ----------

func TestBuildDeploymentTree_Basic(t *testing.T) {
	ctx := context.Background()
	deployUID := types.UID("deploy-uid-1")

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-app",
			Namespace: "default",
			UID:       deployUID,
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "my-app"},
				},
			},
		},
	}

	rs := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-app-abc",
			Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{
				{UID: deployUID},
			},
		},
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-app-abc-xyz",
			Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{
				{UID: rs.UID},
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{
				{Ready: true},
			},
		},
	}

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-svc",
			Namespace: "default",
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"app": "my-app"},
		},
	}

	k8sClient := fake.NewSimpleClientset(dep, rs, pod, svc)

	vs := makeVS("my-vs", "default", "my-svc", []string{"my-gw"})
	gw := makeGW("my-gw", "default")
	dr := makeDR("my-dr", "default", "my-svc", "MUTUAL")
	ap := makeAP("allow-app", "default", "ALLOW", map[string]interface{}{"app": "my-app"})

	dynClient := newFakeDynamicClient(vs, gw, dr, ap)

	builder := NewBuilder(k8sClient, dynClient)
	root, err := builder.BuildDeploymentTree(ctx, "default", "my-app")
	require.NoError(t, err)

	assert.Equal(t, model.KindDeployment, root.Kind)
	assert.Equal(t, "my-app", root.Name)

	// Should have: ReplicaSet, Service, AuthorizationPolicy.
	require.True(t, len(root.Children) >= 3, "expected at least RS, Service, AP children")

	// Find each child by kind.
	var rsNode, svcNode, apNode *model.Node
	for _, child := range root.Children {
		switch child.Kind {
		case model.KindReplicaSet:
			rsNode = child
		case model.KindService:
			svcNode = child
		case model.KindAuthorizationPolicy:
			apNode = child
		}
	}

	require.NotNil(t, rsNode)
	assert.Equal(t, "my-app-abc", rsNode.Name)

	require.NotNil(t, svcNode)
	assert.Equal(t, "my-svc", svcNode.Name)
	// Service should have VirtualService and DestinationRule children.
	var vsNode, drNode *model.Node
	for _, child := range svcNode.Children {
		switch child.Kind {
		case model.KindVirtualService:
			vsNode = child
		case model.KindDestinationRule:
			drNode = child
		}
	}
	require.NotNil(t, vsNode)
	assert.Equal(t, "my-vs", vsNode.Name)
	// VS should have Gateway child.
	require.Len(t, vsNode.Children, 1)
	assert.Equal(t, model.KindGateway, vsNode.Children[0].Kind)

	require.NotNil(t, drNode)
	assert.Equal(t, "MUTUAL", drNode.Status)

	require.NotNil(t, apNode)
	assert.Equal(t, "ALLOW", apNode.Status)
}

func TestBuildDeploymentTree_NotFound(t *testing.T) {
	ctx := context.Background()
	k8sClient := fake.NewSimpleClientset()
	dynClient := newFakeDynamicClient()

	builder := NewBuilder(k8sClient, dynClient)
	_, err := builder.BuildDeploymentTree(ctx, "default", "nonexistent")
	assert.Error(t, err)
}

// ---------- BuildStatefulSetTree tests ----------

func TestBuildStatefulSetTree_Basic(t *testing.T) {
	ctx := context.Background()
	stsUID := types.UID("sts-uid-1")

	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-sts",
			Namespace: "default",
			UID:       stsUID,
		},
		Spec: appsv1.StatefulSetSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "my-sts"},
				},
			},
		},
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-sts-0",
			Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{
				{UID: stsUID},
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
		},
	}

	k8sClient := fake.NewSimpleClientset(sts, pod)
	dynClient := newFakeDynamicClient()

	builder := NewBuilder(k8sClient, dynClient)
	root, err := builder.BuildStatefulSetTree(ctx, "default", "my-sts")
	require.NoError(t, err)

	assert.Equal(t, model.KindStatefulSet, root.Kind)
	assert.Equal(t, "my-sts", root.Name)

	// Should have pod child.
	var podNode *model.Node
	for _, child := range root.Children {
		if child.Kind == model.KindPod {
			podNode = child
		}
	}
	require.NotNil(t, podNode)
	assert.Equal(t, "my-sts-0", podNode.Name)
}

// ---------- BuildServiceTree tests ----------

func TestBuildServiceTree_Basic(t *testing.T) {
	ctx := context.Background()

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-svc",
			Namespace: "default",
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"app": "my-app"},
		},
	}

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-app",
			Namespace: "default",
			UID:       "dep-uid",
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "my-app"},
				},
			},
		},
	}

	k8sClient := fake.NewSimpleClientset(svc, dep)
	dynClient := newFakeDynamicClient()

	builder := NewBuilder(k8sClient, dynClient)
	root, err := builder.BuildServiceTree(ctx, "default", "my-svc")
	require.NoError(t, err)

	assert.Equal(t, model.KindService, root.Kind)
	assert.Equal(t, "my-svc", root.Name)

	// Should have Deployment child.
	var depNode *model.Node
	for _, child := range root.Children {
		if child.Kind == model.KindDeployment {
			depNode = child
		}
	}
	require.NotNil(t, depNode)
	assert.Equal(t, "my-app", depNode.Name)
}

func TestBuildServiceTree_NotFound(t *testing.T) {
	ctx := context.Background()
	k8sClient := fake.NewSimpleClientset()
	dynClient := newFakeDynamicClient()

	builder := NewBuilder(k8sClient, dynClient)
	_, err := builder.BuildServiceTree(ctx, "default", "nonexistent")
	assert.Error(t, err)
}

// ---------- BuildVirtualServiceTree tests ----------

func TestBuildVirtualServiceTree_Basic(t *testing.T) {
	ctx := context.Background()

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-svc",
			Namespace: "default",
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"app": "my-app"},
		},
	}

	k8sClient := fake.NewSimpleClientset(svc)

	vs := makeVS("my-vs", "default", "my-svc", []string{"my-gw"})
	_ = unstructured.SetNestedSlice(vs.Object, []interface{}{"app.example.com"}, "spec", "hosts")
	gw := makeGW("my-gw", "default")
	dynClient := newFakeDynamicClient(vs, gw)

	builder := NewBuilder(k8sClient, dynClient)
	root, err := builder.BuildVirtualServiceTree(ctx, "default", "my-vs")
	require.NoError(t, err)

	assert.Equal(t, model.KindVirtualService, root.Kind)
	assert.Equal(t, "my-vs", root.Name)
	assert.Equal(t, "app.example.com", root.Hosts)

	// Should have Gateway and Service children.
	var gwNode, svcNode *model.Node
	for _, child := range root.Children {
		switch child.Kind {
		case model.KindGateway:
			gwNode = child
		case model.KindService:
			svcNode = child
		}
	}
	require.NotNil(t, gwNode, "expected Gateway child")
	require.NotNil(t, svcNode, "expected Service child")
	assert.Equal(t, "my-svc", svcNode.Name)
}

func TestBuildVirtualServiceTree_MissingService(t *testing.T) {
	ctx := context.Background()

	k8sClient := fake.NewSimpleClientset() // No services.

	vs := makeVS("my-vs", "default", "missing-svc", nil)
	dynClient := newFakeDynamicClient(vs)

	builder := NewBuilder(k8sClient, dynClient)
	root, err := builder.BuildVirtualServiceTree(ctx, "default", "my-vs")
	require.NoError(t, err)

	// Should have a placeholder service node with NotFound status.
	require.Len(t, root.Children, 1)
	assert.Equal(t, model.KindService, root.Children[0].Kind)
	assert.Equal(t, "NotFound", root.Children[0].Status)
}

// ---------- BuildIngressTree tests ----------

func TestBuildIngressTree_Basic(t *testing.T) {
	ctx := context.Background()
	pathType := networkingv1.PathTypePrefix

	ing := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-ingress",
			Namespace: "default",
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{
				{
					Host: "app.example.com",
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     "/",
									PathType: &pathType,
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: "my-svc",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-svc",
			Namespace: "default",
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"app": "my-app"},
		},
	}

	k8sClient := fake.NewSimpleClientset(ing, svc)
	dynClient := newFakeDynamicClient()

	builder := NewBuilder(k8sClient, dynClient)
	root, err := builder.BuildIngressTree(ctx, "default", "my-ingress")
	require.NoError(t, err)

	assert.Equal(t, model.KindIngress, root.Kind)
	assert.Equal(t, "my-ingress", root.Name)
	assert.Equal(t, "app.example.com", root.Hosts)

	// Should have Service child.
	require.Len(t, root.Children, 1)
	assert.Equal(t, model.KindService, root.Children[0].Kind)
	assert.Equal(t, "my-svc", root.Children[0].Name)
}

// ---------- BuildAuthorizationPolicyTree tests ----------

func TestBuildAuthorizationPolicyTree_WithSelector(t *testing.T) {
	ctx := context.Background()

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-app",
			Namespace: "default",
			UID:       "dep-uid",
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "my-app"},
				},
			},
		},
	}

	otherDep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "other-app",
			Namespace: "default",
			UID:       "other-uid",
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "other-app"},
				},
			},
		},
	}

	k8sClient := fake.NewSimpleClientset(dep, otherDep)

	ap := makeAP("allow-app", "default", "ALLOW", map[string]interface{}{"app": "my-app"})
	dynClient := newFakeDynamicClient(ap)

	builder := NewBuilder(k8sClient, dynClient)
	root, err := builder.BuildAuthorizationPolicyTree(ctx, "default", "allow-app")
	require.NoError(t, err)

	assert.Equal(t, model.KindAuthorizationPolicy, root.Kind)
	assert.Equal(t, "ALLOW", root.Status)

	// Should only match my-app, not other-app.
	require.Len(t, root.Children, 1)
	assert.Equal(t, "my-app", root.Children[0].Name)
}

func TestBuildAuthorizationPolicyTree_NamespaceWide(t *testing.T) {
	ctx := context.Background()

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "app-a",
			Namespace: "default",
			UID:       "uid-a",
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "a"},
				},
			},
		},
	}

	dep2 := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "app-b",
			Namespace: "default",
			UID:       "uid-b",
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "b"},
				},
			},
		},
	}

	k8sClient := fake.NewSimpleClientset(dep, dep2)

	// No selector = namespace-wide.
	ap := makeAP("deny-all", "default", "DENY", nil)
	dynClient := newFakeDynamicClient(ap)

	builder := NewBuilder(k8sClient, dynClient)
	root, err := builder.BuildAuthorizationPolicyTree(ctx, "default", "deny-all")
	require.NoError(t, err)

	// Should match both deployments.
	assert.GreaterOrEqual(t, len(root.Children), 2)
}

// ---------- BuildAllTrees tests ----------

func TestBuildAllTrees(t *testing.T) {
	ctx := context.Background()

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "app-a",
			Namespace: "default",
			UID:       "uid-a",
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "a"},
				},
			},
		},
	}

	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sts-b",
			Namespace: "default",
			UID:       "uid-b",
		},
		Spec: appsv1.StatefulSetSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "b"},
				},
			},
		},
	}

	k8sClient := fake.NewSimpleClientset(dep, sts)
	dynClient := newFakeDynamicClient()

	builder := NewBuilder(k8sClient, dynClient)
	roots, err := builder.BuildAllTrees(ctx, "default")
	require.NoError(t, err)

	// Should have 1 Deployment tree + 1 StatefulSet tree.
	require.Len(t, roots, 2)

	kinds := map[model.ResourceKind]bool{}
	for _, root := range roots {
		kinds[root.Kind] = true
	}
	assert.True(t, kinds[model.KindDeployment])
	assert.True(t, kinds[model.KindStatefulSet])
}

func TestBuildAllTrees_Empty(t *testing.T) {
	ctx := context.Background()

	k8sClient := fake.NewSimpleClientset()
	dynClient := newFakeDynamicClient()

	builder := NewBuilder(k8sClient, dynClient)
	roots, err := builder.BuildAllTrees(ctx, "default")
	require.NoError(t, err)
	assert.Empty(t, roots)
}
