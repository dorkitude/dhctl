package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var apiCmd = &cobra.Command{
	Use:   "api <cmd> [param=value ...]",
	Short: "Call any DreamHost API command and print its data as JSON",
	Long: `Call any DreamHost API command your key may call (see ` + "`dhctl whoami`" + `)
and print the response's "data" field as JSON. dhctl adds key, format and unique_id.`,
	Example: `  dhctl api domain-list_domains
  dhctl api dns-list_records`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		params := map[string]string{}
		for _, kv := range args[1:] {
			k, v, ok := strings.Cut(kv, "=")
			if !ok || k == "" {
				return fmt.Errorf("parameter %q must look like name=value", kv)
			}
			params[k] = v
		}
		c, err := newClient()
		if err != nil {
			return err
		}
		cx, cancel := ctx()
		defer cancel()
		data, err := c.Call(cx, args[0], params)
		if err != nil {
			return err
		}
		var v any
		if json.Unmarshal(data, &v) != nil {
			_, err = os.Stdout.Write(append(data, '\n'))
			return err
		}
		return printJSON(v)
	},
}

func init() { rootCmd.AddCommand(apiCmd) }
