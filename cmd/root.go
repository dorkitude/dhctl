// Package cmd holds dhctl's Cobra commands.
package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/dorkitude/dhctl/internal/client"
	"github.com/dorkitude/dhctl/internal/config"
	"github.com/dorkitude/dhctl/internal/ui"
	"github.com/spf13/cobra"
)

// Version is set at build time by GoReleaser.
var Version = "dev"

var (
	jsonOutput bool
	baseURL    string // hidden; tests point it at a fake server
)

var rootCmd = &cobra.Command{
	Use:   "dhctl",
	Short: "A CLI for the DreamHost API (DNS records and more)",
	Long: ui.TitleStyle.Render("🌙 dhctl") + `
A CLI for the DreamHost API.

` + ui.SubtleStyle.Render("Commands:") + `
  auth        Save, check or remove your DreamHost API key
  whoami      Show the API commands your key may call
  dns         List, add and remove DNS records
  api         Call any DreamHost API command directly
`,
	SilenceUsage:  true,
	SilenceErrors: true,
	Version:       Version,
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, ui.Err(err.Error()))
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	rootCmd.PersistentFlags().StringVar(&baseURL, "base-url", client.DefaultBaseURL, "DreamHost API endpoint")
	_ = rootCmd.PersistentFlags().MarkHidden("base-url")
	rootCmd.CompletionOptions.DisableDefaultCmd = true
}

func newClient() (*client.Client, error) {
	key, err := config.LoadKey()
	if err != nil {
		return nil, err
	}
	c := client.New(key)
	c.BaseURL = baseURL
	return c, nil
}

func ctx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 90*time.Second)
}

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
