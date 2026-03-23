package cmd

import (
	"context"
	"fmt"

	"github.com/kurt/kurt/pkg/client"
	"github.com/kurt/kurt/pkg/fzf"
)

// resolveArgs returns the resource names to operate on. If args were provided
// on the command line they are returned as-is. When no args are given and fzf
// is available on an interactive terminal, it presents a picker with the names
// returned by listFn. Otherwise it returns an error asking for at least one
// name.
func resolveArgs(args []string, resourceKind string, listFn func(ctx context.Context, clients *client.Clients, ns string) ([]string, error)) ([]string, error) {
	if len(args) > 0 {
		return args, nil
	}

	if !fzf.Available() {
		return nil, fmt.Errorf("at least one %s name is required (or install fzf for interactive selection)", resourceKind)
	}

	clients, err := client.New(kubeconfig, kubeContext)
	if err != nil {
		return nil, fmt.Errorf("initializing kubernetes client: %w", err)
	}

	names, err := listFn(context.Background(), clients, namespace)
	if err != nil {
		return nil, fmt.Errorf("listing %ss: %w", resourceKind, err)
	}

	selected, err := fzf.Pick(names, fmt.Sprintf("Select %s:", resourceKind))
	if err != nil {
		return nil, err
	}

	return []string{selected}, nil
}
