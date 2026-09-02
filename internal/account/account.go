// Package account is the shared builder behind every provider's
// `<provider> account` subtree: given a provider Spec it emits the
// list/get/use/remove leaves, so the verbs, their output shapes, and the
// remove-of-default policy (promote another account of the same provider)
// stay identical across providers. Providers wire these leaves next to
// their own strategy-specific add; the read-only cross-provider aggregate
// lives in internal/subcommands/account.
package account

// Spec describes one provider's account subtree to the shared builder.
type Spec struct {
	// ProviderID is the store scope (accounts/<provider>/) and the
	// provider's command path segment.
	ProviderID string
	// DisplayName is the provider's human name in help prose ("Google").
	DisplayName string
	// Identity marks providers whose accounts carry an email identity and
	// OAuth token metadata (google): list and get gain the email column,
	// get additionally shows scopes and token_expiry.
	Identity bool
	// Credential names, in prose, what remove deletes: "cached token" for
	// OAuth providers, "stored API key" for key-based ones.
	Credential string
}
