package cmd

import (
	"fmt"
	"sort"

	"github.com/dorkitude/dhctl/internal/ui"
	"github.com/spf13/cobra"
)

var whoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Show the API commands your key may call",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := newClient()
		if err != nil {
			return err
		}
		cx, cancel := ctx()
		defer cancel()
		cmds, err := c.AccessibleCmds(cx)
		if err != nil {
			return err
		}
		if jsonOutput {
			return printJSON(cmds)
		}
		names := make([]string, 0, len(cmds))
		for _, x := range cmds {
			names = append(names, x.Cmd)
		}
		sort.Strings(names)
		fmt.Println(ui.HeaderStyle.Render(fmt.Sprintf("This key can call %d commands:", len(names))))
		for _, n := range names {
			fmt.Println("  " + n)
		}
		return nil
	},
}

func init() { rootCmd.AddCommand(whoamiCmd) }
