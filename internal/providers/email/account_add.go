package email

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/output"
)

// addedAccount is the rendered shape of a successful account add: the
// name only. The password is never part of any output.
type addedAccount struct {
	Name string `json:"name"`
}

// newAccountAddCmd builds `email account add`: capture the IMAP/SMTP
// endpoints and a username/password credential (--password flag, then
// EMAIL_PASSWORD, then a hidden prompt — never echoed) and persist them
// as a provider-scoped account. The password is registered for redaction
// at capture, before anything could print it.
func newAccountAddCmd(cfg *app.Config) *cobra.Command {
	var opts addOptions
	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Add an email account from IMAP/SMTP endpoints and credentials",
		Example: `# Add the "work" account, entering the password at a hidden prompt
everything-cli email account add work --imap-host imap.example.com --smtp-host smtp.example.com --username me@example.com

# Add it non-interactively from the environment
EMAIL_PASSWORD=... everything-cli email account add work --imap-host imap.example.com --smtp-host smtp.example.com --username me@example.com

# Or pass the password directly (careful: shell history)
everything-cli email account add work --imap-host imap.example.com --smtp-host smtp.example.com --username me@example.com --password ...`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := cfg.Store()
			if err != nil {
				return err
			}
			opts.Name = args[0]
			opts.IMAPPortSet = cmd.Flags().Changed("imap-port")
			opts.SMTPPortSet = cmd.Flags().Changed("smtp-port")
			acct, err := addAccount(store, opts)
			if err != nil {
				return fmt.Errorf("adding email account %q: %w", args[0], err)
			}
			view := addedAccount{Name: acct.Name}
			output.Print(cmd.OutOrStdout(), output.ResolveOutput(cfg.Format),
				[]string{"name"}, view,
				[]map[string]any{{"name": view.Name}})
			return nil
		},
	}
	cmd.Flags().StringVar(&opts.IMAPHost, "imap-host", "", "IMAP server host, optionally host:port (required)")
	cmd.Flags().IntVar(&opts.IMAPPort, "imap-port", defaultIMAPPort, "IMAP server port (implicit TLS on 993, STARTTLS otherwise); overrides a port embedded in --imap-host")
	cmd.Flags().StringVar(&opts.SMTPHost, "smtp-host", "", "SMTP server host, optionally host:port (required)")
	cmd.Flags().IntVar(&opts.SMTPPort, "smtp-port", defaultSMTPPort, "SMTP server port (STARTTLS submission); overrides a port embedded in --smtp-host")
	cmd.Flags().StringVar(&opts.Username, "username", "", "Login username, usually the email address (required)")
	cmd.Flags().StringVar(&opts.Password, "password", "",
		"Login password (empty = EMAIL_PASSWORD env var, then a hidden prompt)")
	return cmd
}
