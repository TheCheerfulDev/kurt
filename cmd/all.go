package cmd

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/kurt/kurt/pkg/client"
	"github.com/kurt/kurt/pkg/output"
	"github.com/kurt/kurt/pkg/tree"
	"github.com/spf13/cobra"
)

var allCmd = &cobra.Command{
	Use:   "all",
	Short: "Show resource trees for all Deployments and StatefulSets in the namespace",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		clients, err := client.New(kubeconfig, kubeContext)
		if err != nil {
			return fmt.Errorf("initializing kubernetes client: %w", err)
		}

		builder := tree.NewBuilder(clients.Kubernetes, clients.Dynamic)

		return runWatch(func(w io.Writer) error {
			trees, err := builder.BuildAllTrees(context.Background(), namespace)
			if err != nil {
				return fmt.Errorf("building resource trees: %w", err)
			}

			if len(trees) == 0 {
				fmt.Fprintf(os.Stderr, "No Deployments or StatefulSets found in namespace %q\n", namespace)
				return nil
			}

			for i, root := range trees {
				if i > 0 {
					fmt.Fprintln(w)
				}
				output.PrintTree(w, root)
			}
			return nil
		})
	},
}

func init() {
	rootCmd.AddCommand(allCmd)
}
