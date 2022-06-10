package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

// NewCompletionCmd returns a cobra.Command that generates bash and zsh
// completion scripts for the given root command. If hidden is true, the
// command will not show up in the root command's list of available commands.
func NewCompletionCmd(rootCmd *cobra.Command, hidden bool) *cobra.Command {
	flagZsh := "zsh"
	cmd := &cobra.Command{
		Use:   "completion",
		Short: "Generate shell completion scripts",
		Long: fmt.Sprintf(`Generate Bash and Zsh completion scripts and print them to STDOUT.

Once saved to file, a completion script can be loaded in the shell's
current session as shown:

   $ . <(%s completion)

To configure your bash shell to load completions for each session add to
your $HOME/.bashrc or $HOME/.profile the following instruction:

   . <(%s completion)
`, rootCmd.Use, rootCmd.Use),
		RunE: func(cmd *cobra.Command, _ []string) error {
			zsh, err := cmd.Flags().GetBool(flagZsh)
			if err != nil {
				return err
			}
			if zsh {
				return rootCmd.GenZshCompletion(cmd.OutOrStdout())
			}
			return rootCmd.GenBashCompletion(cmd.OutOrStdout())
		},
		Hidden: hidden,
		Args:   cobra.NoArgs,
	}

	cmd.Flags().Bool(flagZsh, false, "Generate Zsh completion script")

	return cmd
}
