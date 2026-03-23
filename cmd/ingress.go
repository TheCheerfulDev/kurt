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

var ingressCmd = &cobra.Command{
	Use:               "ingress [name...]",
	Aliases:           []string{"ing"},
	Short:             "Show resource tree for an Ingress",
	Args:              cobra.ArbitraryArgs,
	ValidArgsFunction: completeResourceNames(listIngressNames),
	RunE: func(cmd *cobra.Command, args []string) error {
		args, err := resolveArgs(args, "ingress", listIngressNames)
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
				root, err := builder.BuildIngressTree(context.Background(), namespace, name)
				if err != nil {
					return fmt.Errorf("building ingress tree: %w", err)
				}
				output.PrintTree(w, root)
			}
			return nil
		})
	},
}

func init() {
	rootCmd.AddCommand(ingressCmd)
}
