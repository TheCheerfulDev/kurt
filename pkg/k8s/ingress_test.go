package k8s

import (
	"context"
	"testing"

	"github.com/kurt/kurt/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// ---------- IngressToNode tests ----------

func TestIngressToNode(t *testing.T) {
	ing := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-ingress",
			Namespace: "production",
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{
				{Host: "app.example.com"},
			},
		},
	}
	n := IngressToNode(ing)
	assert.Equal(t, model.KindIngress, n.Kind)
	assert.Equal(t, "my-ingress", n.Name)
	assert.Equal(t, "production", n.Namespace)
	assert.Equal(t, "app.example.com", n.Hosts)
}

// ---------- ExtractIngressServiceNames tests ----------

func TestExtractIngressServiceNames_DefaultBackend(t *testing.T) {
	ing := &networkingv1.Ingress{
		Spec: networkingv1.IngressSpec{
			DefaultBackend: &networkingv1.IngressBackend{
				Service: &networkingv1.IngressServiceBackend{
					Name: "default-svc",
				},
			},
		},
	}
	names := ExtractIngressServiceNames(ing)
	require.Len(t, names, 1)
	assert.Equal(t, "default-svc", names[0])
}

func TestExtractIngressServiceNames_RulePaths(t *testing.T) {
	pathType := networkingv1.PathTypePrefix
	ing := &networkingv1.Ingress{
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{
				{
					Host: "app.example.com",
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     "/api",
									PathType: &pathType,
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: "api-svc",
										},
									},
								},
								{
									Path:     "/web",
									PathType: &pathType,
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: "web-svc",
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
	names := ExtractIngressServiceNames(ing)
	require.Len(t, names, 2)
	assert.Contains(t, names, "api-svc")
	assert.Contains(t, names, "web-svc")
}

func TestExtractIngressServiceNames_Dedup(t *testing.T) {
	pathType := networkingv1.PathTypePrefix
	ing := &networkingv1.Ingress{
		Spec: networkingv1.IngressSpec{
			DefaultBackend: &networkingv1.IngressBackend{
				Service: &networkingv1.IngressServiceBackend{
					Name: "my-svc",
				},
			},
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
	names := ExtractIngressServiceNames(ing)
	assert.Len(t, names, 1, "duplicate services should be deduplicated")
}

func TestExtractIngressServiceNames_Empty(t *testing.T) {
	ing := &networkingv1.Ingress{}
	names := ExtractIngressServiceNames(ing)
	assert.Empty(t, names)
}

// ---------- extractIngressHosts tests ----------

func TestExtractIngressHosts_MultipleRules(t *testing.T) {
	ing := &networkingv1.Ingress{
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{
				{Host: "app.example.com"},
				{Host: "api.example.com"},
			},
		},
	}
	assert.Equal(t, "app.example.com,api.example.com", extractIngressHosts(ing))
}

func TestExtractIngressHosts_Dedup(t *testing.T) {
	ing := &networkingv1.Ingress{
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{
				{Host: "app.example.com"},
				{Host: "app.example.com"},
			},
		},
	}
	assert.Equal(t, "app.example.com", extractIngressHosts(ing))
}

func TestExtractIngressHosts_Empty(t *testing.T) {
	ing := &networkingv1.Ingress{}
	assert.Equal(t, "", extractIngressHosts(ing))
}

func TestExtractIngressHosts_EmptyHost(t *testing.T) {
	ing := &networkingv1.Ingress{
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{
				{Host: ""},
			},
		},
	}
	assert.Equal(t, "", extractIngressHosts(ing))
}

// ---------- GetIngressByName tests ----------

func TestGetIngressByName_Found(t *testing.T) {
	ctx := context.Background()

	ing := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-ingress",
			Namespace: "default",
		},
	}

	client := fake.NewSimpleClientset(ing)

	result, err := GetIngressByName(ctx, client, "default", "my-ingress")
	require.NoError(t, err)
	assert.Equal(t, "my-ingress", result.Name)
}

func TestGetIngressByName_NotFound(t *testing.T) {
	ctx := context.Background()
	client := fake.NewSimpleClientset()

	_, err := GetIngressByName(ctx, client, "default", "nonexistent")
	assert.Error(t, err)
}

// ---------- ListIngresses tests ----------

func TestListIngresses(t *testing.T) {
	ctx := context.Background()

	ing1 := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{Name: "ing1", Namespace: "default"},
	}
	ing2 := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{Name: "ing2", Namespace: "default"},
	}

	client := fake.NewSimpleClientset(ing1, ing2)

	result, err := ListIngresses(ctx, client, "default")
	require.NoError(t, err)
	assert.Len(t, result, 2)
}

func TestListIngresses_Empty(t *testing.T) {
	ctx := context.Background()
	client := fake.NewSimpleClientset()

	result, err := ListIngresses(ctx, client, "default")
	require.NoError(t, err)
	assert.Empty(t, result)
}
