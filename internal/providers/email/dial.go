package email

import (
	"context"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/auth"
)

// dialMail is the service seam handed to the mailbox/message read leaves:
// it resolves the acting account through the canonical provider-scoped
// resolver, loads the stored credentials (which re-registers the password
// with the redaction registry at read point, AGENTS.md rule), and dials
// IMAP over TLS. Leaves call it from RunE, so tests substitute a fake.
var dialMail = func(ctx context.Context, cfg *app.Config) (MailService, error) {
	store, err := cfg.Store()
	if err != nil {
		return nil, err
	}
	acct, err := auth.ResolveAccountFor(cfg, store, providerID)
	if err != nil {
		return nil, err
	}
	creds, err := loadCredentials(acct)
	if err != nil {
		return nil, err
	}
	return newMailService(ctx, creds)
}

// dialSendMail is the send path's seam: same account resolution and
// credential loading (redaction re-registration included) as dialMail, but
// no IMAP dial or login — `email message send` needs SMTP only, and must
// work even when the IMAP server is unreachable.
var dialSendMail = func(_ context.Context, cfg *app.Config) (MailService, error) {
	store, err := cfg.Store()
	if err != nil {
		return nil, err
	}
	acct, err := auth.ResolveAccountFor(cfg, store, providerID)
	if err != nil {
		return nil, err
	}
	creds, err := loadCredentials(acct)
	if err != nil {
		return nil, err
	}
	return newSendMailService(creds), nil
}
