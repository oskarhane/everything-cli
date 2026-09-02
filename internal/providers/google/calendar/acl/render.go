package acl

import (
	"github.com/spf13/cobra"

	calendar "google.golang.org/api/calendar/v3"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/output"
)

// aclFields is the acl rule field order for table output; the same names
// are the snake_case JSON and TOON keys. go-pretty's StyleLight upper-cases
// the headers when rendering.
var aclFields = []string{"id", "scope_type", "scope_value", "role"}

// aclRow maps one ACL rule to its output row. A rule without a scope leaves
// the scope fields empty rather than crashing on a nil pointer.
func aclRow(r *calendar.AclRule) map[string]any {
	row := map[string]any{
		"id":          r.Id,
		"scope_type":  "",
		"scope_value": "",
		"role":        r.Role,
	}
	if r.Scope != nil {
		row["scope_type"] = r.Scope.Type
		row["scope_value"] = r.Scope.Value
	}
	return row
}

// printAclRules renders zero or more rules: a JSON/TOON array, or a table
// with one row per rule, in the resolved output format.
func printAclRules(cmd *cobra.Command, cfg *app.Config, rules []*calendar.AclRule) {
	rows := make([]map[string]any, 0, len(rules))
	for _, r := range rules {
		rows = append(rows, aclRow(r))
	}
	output.Print(cmd.OutOrStdout(), output.ResolveOutput(cfg.Format), aclFields, rows, rows)
}

// printAclRule renders a single rule: an object in JSON/TOON, a one-row
// table.
func printAclRule(cmd *cobra.Command, cfg *app.Config, row map[string]any) {
	output.Print(cmd.OutOrStdout(), output.ResolveOutput(cfg.Format), aclFields, row, []map[string]any{row})
}
