package cluster

import "github.com/spf13/cobra"

// NewClusterCommand creates the cluster command
func NewClusterCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cluster",
		Short: "Manage cluster resources",
		Long: `Manage combined cluster resources for ROSA hosted clusters.

This command group provides operations that coordinate multiple resource types
(IAM and VPC) for a hosted cluster.`,
	}

	cmd.AddCommand(newDeployCommand())

	return cmd
}
