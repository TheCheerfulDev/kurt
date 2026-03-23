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

var serviceCmd = &cobra.Command{
	Use:               "service [name...]",
	Aliases:           []string{"svc"},
	Short:             "Show resource tree for a Service",
	Args:              cobra.ArbitraryArgs,
	ValidArgsFunction: completeResourceNames(listServiceNames),
	RunE: func(cmd *cobra.Command, args []string) error {
		args, err := resolveArgs(args, "service", listServiceNames)
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
				root, err := builder.BuildServiceTree(context.Background(), namespace, name)
				if err != nil {
					return fmt.Errorf("building service tree: %w", err)
				}
				output.PrintTree(w, root)
			}
			return nil
		})
	},
}

func init() {
	rootCmd.AddCommand(serviceCmd)
}
