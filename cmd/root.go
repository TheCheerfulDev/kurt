package cmd

import (
	"strings"
	"time"

	"github.com/kurt/kurt/pkg/model"
	"github.com/kurt/kurt/pkg/output"
	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	kubeconfig    string
	kubeContext   string
	namespace     string
	NoColor       bool
	excludeRaw    []string
	ShowHosts     bool
	Watch         bool
	WatchInterval time.Duration
	ShowInactive  bool
)

var rootCmd = &cobra.Command{
	Use:   "kurt",
	Short: "Visualize Kubernetes resource relationships as a tree",
	Long: `kurt shows all objects related to a Deployment, StatefulSet, or
VirtualService. It traverses ownerReferences, Service label selectors, Istio
VirtualService route destinations, and Gateway references to build a full
resource relationship tree.`,
	Version: "0.1.0",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if namespace == "" {
			namespace = resolveNamespace()
		}
		output.NoColor = NoColor
		output.ExcludeKinds = parseExcludeKinds(excludeRaw)
		output.ShowHosts = ShowHosts
		output.ShowInactive = ShowInactive
	},
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVar(&kubeconfig, "kubeconfig", "", "path to kubeconfig file (default: ~/.kube/config)")
	rootCmd.PersistentFlags().StringVar(&kubeContext, "context", "", "kubeconfig context to use")
	rootCmd.PersistentFlags().StringVarP(&namespace, "namespace", "n", "", "kubernetes namespace (default: current context's namespace)")
	rootCmd.PersistentFlags().BoolVar(&NoColor, "no-color", false, "disable color output")
	rootCmd.PersistentFlags().StringSliceVar(&excludeRaw, "exclude", nil, "comma-separated list of resource kinds to hide (e.g. replicaset,virtualservice)")
	rootCmd.PersistentFlags().BoolVar(&ShowHosts, "hosts", false, "show HOSTS column for VirtualService and Ingress resources")
	rootCmd.PersistentFlags().BoolVarP(&Watch, "watch", "w", false, "continuously re-render the tree at a fixed interval")
	rootCmd.PersistentFlags().DurationVar(&WatchInterval, "interval", 5*time.Second, "refresh interval for --watch (e.g. 1s, 5s)")
	rootCmd.PersistentFlags().BoolVar(&ShowInactive, "show-inactive", false, "show inactive (zero-pod) ReplicaSets that are hidden by default")
}

func resolveNamespace() string {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if kubeconfig != "" {
		loadingRules.ExplicitPath = kubeconfig
	}
	overrides := &clientcmd.ConfigOverrides{}
	if kubeContext != "" {
		overrides.CurrentContext = kubeContext
	}

	ns, _, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides).Namespace()
	if err != nil || ns == "" {
		return "default"
	}
	return ns
}

// parseExcludeKinds normalises the raw --exclude values into a set of
// model.ResourceKind values. Input is case-insensitive.
func parseExcludeKinds(raw []string) map[model.ResourceKind]bool {
	if len(raw) == 0 {
		return nil
	}
	lookup := map[string]model.ResourceKind{
		"deployment":          model.KindDeployment,
		"deploy":              model.KindDeployment,
		"statefulset":         model.KindStatefulSet,
		"sts":                 model.KindStatefulSet,
		"replicaset":          model.KindReplicaSet,
		"rs":                  model.KindReplicaSet,
		"pod":                 model.KindPod,
		"service":             model.KindService,
		"svc":                 model.KindService,
		"virtualservice":      model.KindVirtualService,
		"vs":                  model.KindVirtualService,
		"gateway":             model.KindGateway,
		"gw":                  model.KindGateway,
		"ingress":             model.KindIngress,
		"ing":                 model.KindIngress,
		"authorizationpolicy": model.KindAuthorizationPolicy,
		"ap":                  model.KindAuthorizationPolicy,
		"destinationrule":     model.KindDestinationRule,
		"dr":                  model.KindDestinationRule,
	}
	set := make(map[model.ResourceKind]bool)
	for _, v := range raw {
		if kind, ok := lookup[strings.ToLower(strings.TrimSpace(v))]; ok {
			set[kind] = true
		}
	}
	if len(set) == 0 {
		return nil
	}
	return set
}
