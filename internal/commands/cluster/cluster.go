package cluster

import (
	"github.com/spf13/cobra"
)

// NewClusterCommand creates the cluster command
func NewClusterCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cluster",
		Short: "Manage ROSA clusters",
		Long: `Manage ROSA hosted clusters.

This command provides subcommands for creating and managing clusters
by combining IAM and VPC resources.`,
	}

	cmd.AddCommand(newCreateCommand())
	cmd.AddCommand(newListCommand())
	cmd.AddCommand(newDeleteCommand())
	cmd.AddCommand(newKubeconfigCommand())
	cmd.AddCommand(newGetTokenCommand())

	return cmd
}
