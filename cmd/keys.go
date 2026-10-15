package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/dorkitude/dhctl/internal/ui"
	"github.com/spf13/cobra"
)

var keysCmd = &cobra.Command{
	Use:   "keys",
	Short: "API key metacommands",
}

var keysListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List every API key the account has created (needs an all-functions key)",
	Long: `Run api-list_keys. DreamHost only allows it for keys with access to all functions
("*"); a narrower key gets this_key_cannot_access_this_cmd. Output is the API's rows as JSON.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := newClient()
		if err != nil {
			return err
		}
		cx, cancel := ctx()
		defer cancel()
		rows, err := c.ListKeys(cx)
		if err != nil {
			return err
		}
		if !jsonOutput {
			fmt.Println(ui.HeaderStyle.Render(fmt.Sprintf("%d API keys:", len(rows))))
		}
		out := make([]any, 0, len(rows))
		for _, r := range rows {
			var v any
			_ = json.Unmarshal(r, &v)
			out = append(out, v)
		}
		return printJSON(out)
	},
}

func init() {
	keysCmd.AddCommand(keysListCmd)
	rootCmd.AddCommand(keysCmd)
}
