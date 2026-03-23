package cmd

import (
	"context"
	"strings"

	"github.com/kurt/kurt/pkg/client"
	"github.com/spf13/cobra"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// completeResourceNames returns a ValidArgsFunction that lists resource names
// from the cluster, filtering out names already provided as arguments and
// matching the current prefix (toComplete).
func completeResourceNames(listFn func(ctx context.Context, clients *client.Clients, ns string) ([]string, error)) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		clients, err := client.New(kubeconfig, kubeContext)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		ns := namespace
		if ns == "" {
			ns = resolveNamespace()
		}

		names, err := listFn(context.Background(), clients, ns)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		// Build set of already-provided args to exclude from suggestions.
		used := make(map[string]bool, len(args))
		for _, a := range args {
			used[a] = true
		}

		var completions []string
		for _, name := range names {
			if used[name] {
				continue
			}
			if toComplete == "" || strings.HasPrefix(name, toComplete) {
				completions = append(completions, name)
			}
		}

		return completions, cobra.ShellCompDirectiveNoFileComp
	}
}

func listDeploymentNames(ctx context.Context, clients *client.Clients, ns string) ([]string, error) {
	list, err := clients.Kubernetes.AppsV1().Deployments(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	names := make([]string, len(list.Items))
	for i, item := range list.Items {
		names[i] = item.Name
	}
	return names, nil
}

func listStatefulSetNames(ctx context.Context, clients *client.Clients, ns string) ([]string, error) {
	list, err := clients.Kubernetes.AppsV1().StatefulSets(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	names := make([]string, len(list.Items))
	for i, item := range list.Items {
		names[i] = item.Name
	}
	return names, nil
}

func listServiceNames(ctx context.Context, clients *client.Clients, ns string) ([]string, error) {
	list, err := clients.Kubernetes.CoreV1().Services(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	names := make([]string, len(list.Items))
	for i, item := range list.Items {
		names[i] = item.Name
	}
	return names, nil
}

func listIngressNames(ctx context.Context, clients *client.Clients, ns string) ([]string, error) {
	list, err := clients.Kubernetes.NetworkingV1().Ingresses(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	names := make([]string, len(list.Items))
	for i, item := range list.Items {
		names[i] = item.Name
	}
	return names, nil
}

func listVirtualServiceNames(ctx context.Context, clients *client.Clients, ns string) ([]string, error) {
	gvr := schema.GroupVersionResource{
		Group:    "networking.istio.io",
		Version:  "v1",
		Resource: "virtualservices",
	}
	list, err := clients.Dynamic.Resource(gvr).Namespace(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		// CRD might not be installed — return empty rather than error.
		return nil, nil
	}
	names := make([]string, len(list.Items))
	for i, item := range list.Items {
		names[i] = item.GetName()
	}
	return names, nil
}

func listAuthorizationPolicyNames(ctx context.Context, clients *client.Clients, ns string) ([]string, error) {
	gvr := schema.GroupVersionResource{
		Group:    "security.istio.io",
		Version:  "v1",
		Resource: "authorizationpolicies",
	}
	list, err := clients.Dynamic.Resource(gvr).Namespace(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		// CRD might not be installed — return empty rather than error.
		return nil, nil
	}
	names := make([]string, len(list.Items))
	for i, item := range list.Items {
		names[i] = item.GetName()
	}
	return names, nil
}
