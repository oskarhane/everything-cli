package auth

// ScopeUserEmail is requested on every flow so the account's email can be
// resolved from the userinfo endpoint after authorization.
const ScopeUserEmail = "https://www.googleapis.com/auth/userinfo.email"

// ScopesGmail grants full Gmail access: modify, send and compose.
var ScopesGmail = []string{
	"https://www.googleapis.com/auth/gmail.modify",
	"https://www.googleapis.com/auth/gmail.send",
	"https://www.googleapis.com/auth/gmail.compose",
}

// ScopesCalendar grants full Google Calendar access.
var ScopesCalendar = []string{
	"https://www.googleapis.com/auth/calendar",
}
