package cmd

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/dorkitude/dhctl/internal/client"
	"github.com/dorkitude/dhctl/internal/ui"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var (
	listZone   string
	listType   string
	listRecord string
	addComment string
	rmYes      bool
)

var dnsCmd = &cobra.Command{
	Use:   "dns",
	Short: "List, add and remove DNS records",
	Long: `List, add and remove DNS records on domains whose DNS DreamHost hosts.

The DreamHost API has no update: to change a record, add the new one, then remove the
old one. Records DreamHost manages itself (editable=0) can't be changed through the API.`,
}

var dnsListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List DNS records (optionally filtered)",
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := newClient()
		if err != nil {
			return err
		}
		cx, cancel := ctx()
		defer cancel()
		recs, err := c.ListRecords(cx)
		if err != nil {
			return err
		}
		recs = filterRecords(recs, listZone, listType, listRecord)
		sort.SliceStable(recs, func(i, j int) bool {
			if recs[i].Zone != recs[j].Zone {
				return recs[i].Zone < recs[j].Zone
			}
			if recs[i].Record != recs[j].Record {
				return recs[i].Record < recs[j].Record
			}
			return recs[i].Type < recs[j].Type
		})
		if jsonOutput {
			return printJSON(recs)
		}
		if len(recs) == 0 {
			fmt.Println(ui.SubtleStyle.Render("No matching records."))
			return nil
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, ui.HeaderStyle.Render("RECORD")+"\t"+ui.HeaderStyle.Render("TYPE")+"\t"+ui.HeaderStyle.Render("VALUE")+"\t"+ui.HeaderStyle.Render("EDITABLE")+"\t"+ui.HeaderStyle.Render("COMMENT"))
		for _, r := range recs {
			ed := "yes"
			if !r.IsEditable() {
				ed = "no"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", r.Record, r.Type, r.Value, ed, r.Comment)
		}
		return w.Flush()
	},
}

var dnsAddCmd = &cobra.Command{
	Use:   "add <record> <type> <value>",
	Short: "Add a DNS record (record is the full name, e.g. www.example.com)",
	Example: `  dhctl dns add www.example.com A 203.0.113.10
  dhctl dns add example.com TXT "v=spf1 -all" --comment "no mail"`,
	Args: cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := newClient()
		if err != nil {
			return err
		}
		cx, cancel := ctx()
		defer cancel()
		if err := c.AddRecord(cx, args[0], args[1], args[2], addComment); err != nil {
			return err
		}
		if jsonOutput {
			return printJSON(map[string]string{"added": args[0], "type": strings.ToUpper(args[1]), "value": args[2]})
		}
		fmt.Println(ui.OK(fmt.Sprintf("Added %s %s %s", args[0], strings.ToUpper(args[1]), args[2])))
		return nil
	},
}

var dnsRmCmd = &cobra.Command{
	Use:     "rm <record> <type> <value>",
	Aliases: []string{"remove", "delete"},
	Short:   "Remove the DNS record matching record, type and value exactly",
	Example: `  dhctl dns rm old.example.com A 203.0.113.10 --yes`,
	Args:    cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		record, typ, value := args[0], strings.ToUpper(args[1]), args[2]
		c, err := newClient()
		if err != nil {
			return err
		}
		cx, cancel := ctx()
		defer cancel()

		// Look first, so a typo fails clearly instead of as a bare API error.
		recs, err := c.ListRecords(cx)
		if err != nil {
			return err
		}
		var match *client.Record
		for i := range recs {
			r := recs[i]
			if strings.EqualFold(r.Record, record) && r.Type == typ && r.Value == value {
				match = &r
				break
			}
		}
		if match == nil {
			return fmt.Errorf("no record %s %s %s (see `dhctl dns list --record %s`)", record, typ, value, record)
		}
		if !match.IsEditable() {
			return fmt.Errorf("%s %s %s is managed by DreamHost (editable=0); the API can't remove it", record, typ, value)
		}
		if !rmYes {
			if !term.IsTerminal(int(os.Stdin.Fd())) {
				return fmt.Errorf("refusing to remove without --yes (not a terminal)")
			}
			fmt.Fprintf(os.Stderr, "Remove %s %s %s? Type the record name to confirm: ", record, typ, value)
			line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
			if !strings.EqualFold(strings.TrimSpace(line), record) {
				return fmt.Errorf("not confirmed; nothing removed")
			}
		}
		if err := c.RemoveRecord(cx, record, typ, value); err != nil {
			return err
		}
		if jsonOutput {
			return printJSON(map[string]string{"removed": record, "type": typ, "value": value})
		}
		fmt.Println(ui.OK(fmt.Sprintf("Removed %s %s %s", record, typ, value)))
		return nil
	},
}

func filterRecords(recs []client.Record, zone, typ, record string) []client.Record {
	out := recs[:0:0]
	for _, r := range recs {
		if zone != "" && !strings.EqualFold(r.Zone, zone) {
			continue
		}
		if typ != "" && !strings.EqualFold(r.Type, typ) {
			continue
		}
		if record != "" && !strings.EqualFold(r.Record, record) {
			continue
		}
		out = append(out, r)
	}
	return out
}

func init() {
	dnsListCmd.Flags().StringVar(&listZone, "zone", "", "Only records in this zone (e.g. example.com)")
	dnsListCmd.Flags().StringVar(&listType, "type", "", "Only records of this type (A, CNAME, TXT, ...)")
	dnsListCmd.Flags().StringVar(&listRecord, "record", "", "Only records with this exact name")
	dnsAddCmd.Flags().StringVar(&addComment, "comment", "", "Optional comment")
	dnsRmCmd.Flags().BoolVarP(&rmYes, "yes", "y", false, "Don't ask for confirmation")
	dnsCmd.AddCommand(dnsListCmd, dnsAddCmd, dnsRmCmd)
	rootCmd.AddCommand(dnsCmd)
}
