package email

// The emersion mail stack is pinned here so it stays a direct require in
// go.mod ahead of the mailbox/message nodes: the mail adapter (the only
// file that will import these modules for real) is delivered by a later
// change, and without these blank imports `go mod tidy` would drop them.
import (
	_ "github.com/emersion/go-imap/v2" // IMAP client (message read)
	_ "github.com/emersion/go-message" // MIME parse/build
	_ "github.com/emersion/go-smtp"    // SMTP client (message send)
)
