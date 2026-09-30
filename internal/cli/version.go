package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newVersionCommand(build BuildInfo) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version, commit and build date",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintf(cmd.OutOrStdout(), "minio-manager %s (commit %s, built %s)\n",
				build.Version, build.Commit, build.Date)
			return nil
		},
	}
}
