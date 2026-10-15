package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/dorkitude/dhctl/internal/client"
	"github.com/dorkitude/dhctl/internal/ui"
	"github.com/spf13/cobra"
)

var (
	subName     string
	postName    string
	postStamp   string
	postCharset string
	postType    string
	postDupOK   bool
	postMsgFile string
	announceYes bool
)

var announceCmd = &cobra.Command{
	Use:     "announce",
	Aliases: []string{"lists"},
	Short:   "Announcement lists: lists, subscribers, posts",
	Long: `Announcement lists (DreamHost's one-way mailing lists). A list is named by its
address, listname@domain.`,
}

func splitList(addr string) (string, string, error) {
	name, domain, ok := strings.Cut(addr, "@")
	if !ok || name == "" || domain == "" {
		return "", "", fmt.Errorf("list %q must look like listname@domain", addr)
	}
	return name, domain, nil
}

var announceListsCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List announcement lists",
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := newClient()
		if err != nil {
			return err
		}
		cx, cancel := ctx()
		defer cancel()
		ls, err := c.ListLists(cx)
		if err != nil {
			return err
		}
		if jsonOutput {
			return printJSON(ls)
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, ui.HeaderStyle.Render("LIST")+"\t"+ui.HeaderStyle.Render("SUBSCRIBERS")+"\t"+ui.HeaderStyle.Render("SINCE")+"\t"+ui.HeaderStyle.Render("NAME"))
		for _, l := range ls {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", l.Address(), l.NumSubscribers, l.StartDate, l.Name)
		}
		return w.Flush()
	},
}

var announceSubsCmd = &cobra.Command{
	Use:   "subscribers <listname@domain>",
	Short: "List a list's subscribers",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name, domain, err := splitList(args[0])
		if err != nil {
			return err
		}
		c, err := newClient()
		if err != nil {
			return err
		}
		cx, cancel := ctx()
		defer cancel()
		subs, err := c.ListSubscribers(cx, name, domain)
		if err != nil {
			return err
		}
		if jsonOutput {
			return printJSON(subs)
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, ui.HeaderStyle.Render("EMAIL")+"\t"+ui.HeaderStyle.Render("NAME")+"\t"+ui.HeaderStyle.Render("CONFIRMED")+"\t"+ui.HeaderStyle.Render("BOUNCES")+"\t"+ui.HeaderStyle.Render("SUBSCRIBED"))
		for _, s := range subs {
			fmt.Fprintf(w, "%s\t%s\t%v\t%v\t%s\n", s.Email, s.Name, s.Confirmed, s.NumBounces, s.SubscribeDate)
		}
		return w.Flush()
	},
}

var announceSubscribeCmd = &cobra.Command{
	Use:   "subscribe <listname@domain> <email>",
	Short: "Add a subscriber",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		name, domain, err := splitList(args[0])
		if err != nil {
			return err
		}
		c, err := newClient()
		if err != nil {
			return err
		}
		cx, cancel := ctx()
		defer cancel()
		if err := c.AddSubscriber(cx, name, domain, args[1], subName); err != nil {
			return err
		}
		if jsonOutput {
			return printJSON(map[string]string{"subscribed": args[1], "list": args[0]})
		}
		fmt.Println(ui.OK(fmt.Sprintf("Subscribed %s to %s", args[1], args[0])))
		return nil
	},
}

var announceUnsubscribeCmd = &cobra.Command{
	Use:   "unsubscribe <listname@domain> <email>",
	Short: "Remove a subscriber",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		name, domain, err := splitList(args[0])
		if err != nil {
			return err
		}
		if !announceYes {
			if err := confirm(fmt.Sprintf("Unsubscribe %s from %s?", args[1], args[0]), args[1]); err != nil {
				return err
			}
		}
		c, err := newClient()
		if err != nil {
			return err
		}
		cx, cancel := ctx()
		defer cancel()
		if err := c.RemoveSubscriber(cx, name, domain, args[1]); err != nil {
			return err
		}
		if jsonOutput {
			return printJSON(map[string]string{"unsubscribed": args[1], "list": args[0]})
		}
		fmt.Println(ui.OK(fmt.Sprintf("Unsubscribed %s from %s", args[1], args[0])))
		return nil
	},
}

var announcePostCmd = &cobra.Command{
	Use:   "post <listname@domain> <subject>",
	Short: "Send (or schedule) an announcement to every subscriber",
	Long: `Send an announcement. The message body comes from --file, or stdin when --file is
"-" or omitted. --stamp schedules it ("YYYY-MM-DD HH:MM:SS"). This emails real people, so
it asks for confirmation unless --yes.`,
	Example: `  dhctl announce post news@example.com "October update" --file update.txt --type text
  echo "<p>Hi</p>" | dhctl announce post news@example.com "Hi" --type html --yes`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		name, domain, err := splitList(args[0])
		if err != nil {
			return err
		}
		if postType != "" && postType != "text" && postType != "html" {
			return fmt.Errorf("--type must be text or html")
		}
		var body []byte
		if postMsgFile == "" || postMsgFile == "-" {
			body, err = io.ReadAll(os.Stdin)
		} else {
			body, err = os.ReadFile(postMsgFile)
		}
		if err != nil {
			return err
		}
		if strings.TrimSpace(string(body)) == "" {
			return fmt.Errorf("message body is empty")
		}
		if !announceYes {
			return fmt.Errorf("posting emails every subscriber of %s; re-run with --yes to send", args[0])
		}
		c, err := newClient()
		if err != nil {
			return err
		}
		cx, cancel := ctx()
		defer cancel()
		a := client.Announcement{Listname: name, Domain: domain, Subject: args[1], Message: string(body),
			Name: postName, Stamp: postStamp, Charset: postCharset, Type: postType, DuplicateOK: postDupOK}
		if err := c.PostAnnouncement(cx, a); err != nil {
			return err
		}
		if jsonOutput {
			return printJSON(map[string]string{"posted": args[1], "list": args[0], "stamp": postStamp})
		}
		when := "now"
		if postStamp != "" {
			when = "at " + postStamp
		}
		fmt.Println(ui.OK(fmt.Sprintf("Posted %q to %s (%s)", args[1], args[0], when)))
		return nil
	},
}

func init() {
	announceSubscribeCmd.Flags().StringVar(&subName, "name", "", "Subscriber's display name")
	announceUnsubscribeCmd.Flags().BoolVarP(&announceYes, "yes", "y", false, "Don't ask for confirmation")
	announcePostCmd.Flags().StringVar(&postMsgFile, "file", "", `Message body file ("-" or omitted: stdin)`)
	announcePostCmd.Flags().StringVar(&postName, "name", "", "Sender display name")
	announcePostCmd.Flags().StringVar(&postStamp, "stamp", "", `Schedule for "YYYY-MM-DD HH:MM:SS" (default: now)`)
	announcePostCmd.Flags().StringVar(&postCharset, "charset", "", "Character set (e.g. utf-8)")
	announcePostCmd.Flags().StringVar(&postType, "type", "", "text or html")
	announcePostCmd.Flags().BoolVar(&postDupOK, "duplicate-ok", false, "Allow sending a message identical to an earlier one")
	announcePostCmd.Flags().BoolVarP(&announceYes, "yes", "y", false, "Actually send")
	announceCmd.AddCommand(announceListsCmd, announceSubsCmd, announceSubscribeCmd, announceUnsubscribeCmd, announcePostCmd)
	rootCmd.AddCommand(announceCmd)
}
