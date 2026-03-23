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
	"k8s.io/client-go/kubernetes/fake"
)

// ---------- ServiceToNode tests ----------

func TestServiceToNode(t *testing.T) {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-svc",
			Namespace: "production",
		},
	}
	n := ServiceToNode(svc)
	assert.Equal(t, model.KindService, n.Kind)
	assert.Equal(t, "my-svc", n.Name)
	assert.Equal(t, "production", n.Namespace)
}

// ---------- FindServicesForLabels tests ----------

func TestFindServicesForLabels_Matching(t *testing.T) {
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

	client := fake.NewSimpleClientset(svc)

	matched, err := FindServicesForLabels(ctx, client, "default", map[string]string{"app": "my-app", "version": "v1"})
	require.NoError(t, err)
	require.Len(t, matched, 1)
	assert.Equal(t, "my-svc", matched[0].Name)
}

func TestFindServicesForLabels_NoMatch(t *testing.T) {
	ctx := context.Background()

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-svc",
			Namespace: "default",
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"app": "other-app"},
		},
	}

	client := fake.NewSimpleClientset(svc)

	matched, err := FindServicesForLabels(ctx, client, "default", map[string]string{"app": "my-app"})
	require.NoError(t, err)
	assert.Empty(t, matched)
}

func TestFindServicesForLabels_EmptySelector(t *testing.T) {
	ctx := context.Background()

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "headless-svc",
			Namespace: "default",
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{},
		},
	}

	client := fake.NewSimpleClientset(svc)

	matched, err := FindServicesForLabels(ctx, client, "default", map[string]string{"app": "my-app"})
	require.NoError(t, err)
	assert.Empty(t, matched, "services with empty selector should not match")
}

// ---------- GetServiceByName tests ----------

func TestGetServiceByName_Found(t *testing.T) {
	ctx := context.Background()

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-svc",
			Namespace: "default",
		},
	}

	client := fake.NewSimpleClientset(svc)

	result, err := GetServiceByName(ctx, client, "default", "my-svc")
	require.NoError(t, err)
	assert.Equal(t, "my-svc", result.Name)
}

func TestGetServiceByName_NotFound(t *testing.T) {
	ctx := context.Background()
	client := fake.NewSimpleClientset()

	_, err := GetServiceByName(ctx, client, "default", "nonexistent")
	assert.Error(t, err)
}

// ---------- FindDeploymentsForService tests ----------

func TestFindDeploymentsForService_Matching(t *testing.T) {
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

	nonMatchDep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "other-app",
			Namespace: "default",
			UID:       "other-uid",
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "other"},
				},
			},
		},
	}

	client := fake.NewSimpleClientset(svc, dep, nonMatchDep)

	nodes, err := FindDeploymentsForService(ctx, client, newFakeDynamicClient(), "default", svc)
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	assert.Equal(t, "my-app", nodes[0].Name)
	assert.Equal(t, model.KindDeployment, nodes[0].Kind)
}

func TestFindDeploymentsForService_EmptySelector(t *testing.T) {
	ctx := context.Background()

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "headless",
			Namespace: "default",
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{},
		},
	}

	client := fake.NewSimpleClientset(svc)

	nodes, err := FindDeploymentsForService(ctx, client, nil, "default", svc)
	require.NoError(t, err)
	assert.Empty(t, nodes)
}

// ---------- FindStatefulSetsForService tests ----------

func TestFindStatefulSetsForService_Matching(t *testing.T) {
	ctx := context.Background()

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-svc",
			Namespace: "default",
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"app": "my-sts"},
		},
	}

	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-sts",
			Namespace: "default",
			UID:       "sts-uid",
		},
		Spec: appsv1.StatefulSetSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "my-sts"},
				},
			},
		},
	}

	client := fake.NewSimpleClientset(svc, sts)

	nodes, err := FindStatefulSetsForService(ctx, client, newFakeDynamicClient(), "default", svc)
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	assert.Equal(t, "my-sts", nodes[0].Name)
	assert.Equal(t, model.KindStatefulSet, nodes[0].Kind)
}

func TestFindStatefulSetsForService_EmptySelector(t *testing.T) {
	ctx := context.Background()

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "headless",
			Namespace: "default",
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{},
		},
	}

	client := fake.NewSimpleClientset(svc)

	nodes, err := FindStatefulSetsForService(ctx, client, nil, "default", svc)
	require.NoError(t, err)
	assert.Empty(t, nodes)
}
