package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/dorkitude/dhctl/internal/config"
	"github.com/dorkitude/dhctl/internal/ui"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Save your DreamHost API key (read from stdin or a hidden prompt)",
	Long: `Save your DreamHost API key to ~/.config/dhctl/key (0600).

Create a key at https://panel.dreamhost.com/?tree=home.api and grant it only the
commands you need (for DNS: dns-list_records, dns-add_record, dns-remove_record).
The key is read from stdin, never from an argument, so it stays out of shell history:

  dhctl auth < keyfile
  dhctl auth            # hidden prompt on a terminal

$DREAMHOST_API_KEY, when set, overrides the saved key.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		key, err := readKey()
		if err != nil {
			return err
		}
		path, err := config.SaveKey(key)
		if err != nil {
			return err
		}
		c, err := newClient()
		if err != nil {
			return err
		}
		cx, cancel := ctx()
		defer cancel()
		cmds, err := c.AccessibleCmds(cx)
		if err != nil {
			return fmt.Errorf("saved to %s, but the key didn't work: %w", path, err)
		}
		fmt.Println(ui.OK(fmt.Sprintf("Saved API key to %s (it can call %d commands)", path, len(cmds))))
		return nil
	},
}

var authStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check that the saved key works",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		path, _ := config.KeyPath()
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
			return printJSON(map[string]any{"key_file": path, "ok": true, "accessible_cmds": len(cmds)})
		}
		fmt.Println(ui.OK(fmt.Sprintf("Key works (%s): %d commands allowed", source(path), len(cmds))))
		return nil
	},
}

var authLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Delete the saved key file",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := config.RemoveKey(); err != nil {
			return err
		}
		fmt.Println(ui.OK("Removed the saved key (revoke it in the DreamHost panel too)"))
		return nil
	},
}

func source(path string) string {
	if strings.TrimSpace(os.Getenv("DREAMHOST_API_KEY")) != "" {
		return "$DREAMHOST_API_KEY"
	}
	return path
}

func readKey() (string, error) {
	fd := int(os.Stdin.Fd())
	if term.IsTerminal(fd) {
		fmt.Fprint(os.Stderr, "DreamHost API key: ")
		b, err := term.ReadPassword(fd)
		fmt.Fprintln(os.Stderr)
		return strings.TrimSpace(string(b)), err
	}
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

func init() {
	authCmd.AddCommand(authStatusCmd, authLogoutCmd)
	rootCmd.AddCommand(authCmd)
}
