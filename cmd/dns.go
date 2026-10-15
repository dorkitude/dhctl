package cmd

import (
	"bufio"
	"context"
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
	listWide   bool
)

// recordTypes are the types dns-add_record accepts.
var recordTypes = []string{"A", "AAAA", "CNAME", "NAPTR", "NS", "SRV", "TXT"}

func checkType(t string) (string, error) {
	t = strings.ToUpper(t)
	for _, ok := range recordTypes {
		if t == ok {
			return t, nil
		}
	}
	return "", fmt.Errorf("type %q isn't supported by the DreamHost API (use one of %s)", t, strings.Join(recordTypes, ", "))
}

func truncate(s string, n int) string {
	if listWide || len([]rune(s)) <= n {
		return s
	}
	return string([]rune(s)[:n-1]) + "…"
}

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
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", r.Record, r.Type, truncate(r.Value, 60), ed, truncate(r.Comment, 30))
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
		if _, err := checkType(args[1]); err != nil {
			return err
		}
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

		if _, err := findEditable(cx, c, record, typ, value); err != nil {
			return err
		}
		if !rmYes {
			if err := confirm(fmt.Sprintf("Remove %s %s %s?", record, typ, value), record); err != nil {
				return err
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

var dnsUpdateCmd = &cobra.Command{
	Use:   "update <record> <type> <old-value> <new-value>",
	Short: "Change a record's value (adds the new record, then removes the old one)",
	Long: `The DreamHost API has no update, so this adds the new record first and then removes
the old one. If the add fails, nothing changes; if the remove fails, both records exist
and dhctl says so.`,
	Example: `  dhctl dns update www.example.com A 203.0.113.10 203.0.113.20 --yes`,
	Args:    cobra.ExactArgs(4),
	RunE: func(cmd *cobra.Command, args []string) error {
		record, oldV, newV := args[0], args[2], args[3]
		typ, err := checkType(args[1])
		if err != nil {
			return err
		}
		if oldV == newV {
			return fmt.Errorf("old and new values are the same; nothing to do")
		}
		c, err := newClient()
		if err != nil {
			return err
		}
		cx, cancel := ctx()
		defer cancel()
		old, err := findEditable(cx, c, record, typ, oldV)
		if err != nil {
			return err
		}
		if !rmYes {
			if err := confirm(fmt.Sprintf("Change %s %s from %s to %s?", record, typ, oldV, newV), record); err != nil {
				return err
			}
		}
		comment := addComment
		if comment == "" {
			comment = old.Comment
		}
		if err := c.AddRecord(cx, record, typ, newV, comment); err != nil {
			return fmt.Errorf("adding the new record failed, nothing changed: %w", err)
		}
		if err := c.RemoveRecord(cx, record, typ, oldV); err != nil {
			return fmt.Errorf("added %s %s %s, but removing the old value failed, so both exist: %w", record, typ, newV, err)
		}
		if jsonOutput {
			return printJSON(map[string]string{"updated": record, "type": typ, "old": oldV, "new": newV})
		}
		fmt.Println(ui.OK(fmt.Sprintf("Updated %s %s: %s → %s", record, typ, oldV, newV)))
		return nil
	},
}

// findEditable returns the record matching exactly, or a clear error if it's missing or
// managed by DreamHost. Looking first turns a typo into a readable error.
func findEditable(cx context.Context, c *client.Client, record, typ, value string) (*client.Record, error) {
	recs, err := c.ListRecords(cx)
	if err != nil {
		return nil, err
	}
	for i := range recs {
		r := recs[i]
		if strings.EqualFold(r.Record, record) && r.Type == typ && r.Value == value {
			if !r.IsEditable() {
				return nil, fmt.Errorf("%s %s %s is managed by DreamHost (editable=0); the API can't change it", record, typ, value)
			}
			return &r, nil
		}
	}
	return nil, fmt.Errorf("no record %s %s %s (see `dhctl dns list --record %s`)", record, typ, value, record)
}

// confirm asks the user to type want; off a terminal it refuses (callers offer --yes).
func confirm(question, want string) error {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return fmt.Errorf("refusing without --yes (not a terminal)")
	}
	fmt.Fprintf(os.Stderr, "%s Type %q to confirm: ", question, want)
	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	if !strings.EqualFold(strings.TrimSpace(line), want) {
		return fmt.Errorf("not confirmed; nothing changed")
	}
	return nil
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
	dnsListCmd.Flags().BoolVar(&listWide, "wide", false, "Don't truncate long values")
	dnsAddCmd.Flags().StringVar(&addComment, "comment", "", "Optional comment")
	dnsUpdateCmd.Flags().StringVar(&addComment, "comment", "", "Comment for the new record (default: keep the old one)")
	dnsRmCmd.Flags().BoolVarP(&rmYes, "yes", "y", false, "Don't ask for confirmation")
	dnsUpdateCmd.Flags().BoolVarP(&rmYes, "yes", "y", false, "Don't ask for confirmation")
	dnsCmd.AddCommand(dnsListCmd, dnsAddCmd, dnsUpdateCmd, dnsRmCmd)
	rootCmd.AddCommand(dnsCmd)
}
