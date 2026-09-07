package root

import (
	"context"
	"os"

	"github.com/spf13/cobra"

	"github.com/scottia/IMDb-Trakt-TMDb-Sync/cmd"
	"github.com/scottia/IMDb-Trakt-TMDb-Sync/cmd/configure"
	"github.com/scottia/IMDb-Trakt-TMDb-Sync/cmd/sync"
)

func NewCommand(ctx context.Context) *cobra.Command {
	command := &cobra.Command{
		Use:     cmd.CommandNameRoot,
		Aliases: []string{cmd.CommandAliasRoot},
		Short:   "IMDb-Trakt-TMDb-Sync command line interface",
		PersistentPreRun: func(c *cobra.Command, _ []string) {
			c.SetOut(os.Stdout)
			c.SetErr(os.Stderr)
		},
		RunE: func(c *cobra.Command, _ []string) error {
			return c.Help()
		},
		CompletionOptions: cobra.CompletionOptions{
			DisableDefaultCmd: true,
		},
		SilenceUsage: true,
	}
	command.SetHelpCommand(&cobra.Command{
		Hidden: true,
	})
	command.AddCommand(
		configure.NewCommand(ctx),
		sync.NewCommand(ctx),
	)
	command.SetOut(os.Stdout)
	command.SetErr(os.Stderr)
	return command
}
