package nodepool

import (
	"github.com/spf13/cobra"
)

// NewNodePoolCommand creates the nodepool command
func NewNodePoolCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "nodepool",
		Short: "Manage ROSA cluster node pools",
		Long: `Manage node pools for ROSA hosted clusters.

This command provides subcommands for creating, listing, and deleting
node pools via the platform API.`,
	}

	cmd.AddCommand(newCreateCommand())
	cmd.AddCommand(newListCommand())
	cmd.AddCommand(newDeleteCommand())

	return cmd
}
