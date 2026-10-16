package cli

import (
	"github.com/spf13/cobra"

	"github.com/mariyzx/capwatch/internal/cli/run"
)

func NewCmdRoot() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "capwatch",
		Short: "Claim an Oracle Cloud Always Free A1 instance as soon as capacity frees up",
	}
	cmd.AddCommand(run.NewCmdRun())
	return cmd
}
