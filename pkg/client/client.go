package client

import (
	"fmt"
	"path/filepath"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

// Clients holds both the typed Kubernetes clientset and the dynamic client for CRDs.
type Clients struct {
	Kubernetes kubernetes.Interface
	Dynamic    dynamic.Interface
}

// New creates Kubernetes clients from the given kubeconfig path and context.
// If kubeconfigPath is empty, it falls back to the default ~/.kube/config or in-cluster config.
func New(kubeconfigPath, kubeContext string) (*Clients, error) {
	config, err := buildConfig(kubeconfigPath, kubeContext)
	if err != nil {
		return nil, fmt.Errorf("building kubeconfig: %w", err)
	}

	k8s, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("creating kubernetes client: %w", err)
	}

	dyn, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("creating dynamic client: %w", err)
	}

	return &Clients{
		Kubernetes: k8s,
		Dynamic:    dyn,
	}, nil
}

func buildConfig(kubeconfigPath, kubeContext string) (*rest.Config, error) {
	if kubeconfigPath == "" {
		kubeconfigPath = defaultKubeconfigPath()
	}

	loadingRules := &clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfigPath}
	overrides := &clientcmd.ConfigOverrides{}

	if kubeContext != "" {
		overrides.CurrentContext = kubeContext
	}

	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides).ClientConfig()
	if err != nil {
		// Fall back to in-cluster config.
		inCluster, inClusterErr := rest.InClusterConfig()
		if inClusterErr != nil {
			return nil, fmt.Errorf("kubeconfig error: %w; in-cluster error: %w", err, inClusterErr)
		}
		return inCluster, nil
	}

	return config, nil
}

func defaultKubeconfigPath() string {
	if home := homedir.HomeDir(); home != "" {
		return filepath.Join(home, ".kube", "config")
	}
	return ""
}
