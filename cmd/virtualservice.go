package cmd

import (
	"context"
	"fmt"
	"io"

	"github.com/kurt/kurt/pkg/client"
	"github.com/kurt/kurt/pkg/output"
	"github.com/kurt/kurt/pkg/tree"
	"github.com/spf13/cobra"
)

var virtualServiceCmd = &cobra.Command{
	Use:               "virtualservice [name...]",
	Aliases:           []string{"vs"},
	Short:             "Show resource tree for a VirtualService",
	Args:              cobra.ArbitraryArgs,
	ValidArgsFunction: completeResourceNames(listVirtualServiceNames),
	RunE: func(cmd *cobra.Command, args []string) error {
		args, err := resolveArgs(args, "virtualservice", listVirtualServiceNames)
		if err != nil {
			return err
		}

		clients, err := client.New(kubeconfig, kubeContext)
		if err != nil {
			return fmt.Errorf("initializing kubernetes client: %w", err)
		}

		builder := tree.NewBuilder(clients.Kubernetes, clients.Dynamic)

		return runWatch(func(w io.Writer) error {
			for i, name := range args {
				if i > 0 {
					fmt.Fprintln(w)
				}
				root, err := builder.BuildVirtualServiceTree(context.Background(), namespace, name)
				if err != nil {
					return fmt.Errorf("building virtualservice tree: %w", err)
				}
				output.PrintTree(w, root)
			}
			return nil
		})
	},
}

func init() {
	rootCmd.AddCommand(virtualServiceCmd)
}
