// Package api builds the `linear api` command: an authenticated GraphQL
// passthrough to the Linear API, gh api-style.
package api

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/output"
	"github.com/oskarhane/everything-cli/internal/providers/linear/service"
)

// NewCmd returns `linear api`: run one raw GraphQL query or mutation and
// print the raw data document as JSON.
func NewCmd(cfg *app.Config, newSvc service.Dialer[service.GraphQLService]) *cobra.Command {
	var variables string
	cmd := &cobra.Command{
		Use:   "api <query>",
		Short: "Run a raw GraphQL query against the Linear API",
		Example: `# Query the current viewer
everything-cli linear api '{ viewer { id name email } }'

# Pass variables as inline JSON
everything-cli linear api 'query($id: String!) { issue(id: $id) { id title } }' --variables '{"id": "ENG-1"}'

# Read variables from a file
everything-cli linear api 'query($id: String!) { issue(id: $id) { id title } }' --variables @vars.json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			vars, err := parseVariables(cfg, variables)
			if err != nil {
				return err
			}
			svc, err := newSvc(cmd.Context())
			if err != nil {
				return err
			}
			data, err := svc.ExecGraphQL(cmd.Context(), args[0], vars)
			if err != nil {
				return err
			}
			// PrintJSON needs a value, not raw bytes; a missing data
			// document prints as null.
			var doc any
			if len(data) > 0 {
				if err := json.Unmarshal(data, &doc); err != nil {
					return fmt.Errorf("decoding linear data document: %w", err)
				}
			}
			output.PrintJSON(cmd.OutOrStdout(), doc)
			return nil
		},
	}
	cmd.Flags().StringVar(&variables, "variables", "", "GraphQL variables as inline JSON or @file.json")
	return cmd
}

// parseVariables decodes --variables: empty means none, a leading @ reads
// the JSON document from the file, anything else is inline JSON. Malformed
// input errors here, before any API call.
func parseVariables(cfg *app.Config, raw string) (map[string]any, error) {
	if raw == "" {
		return nil, nil
	}
	if strings.HasPrefix(raw, "@") {
		data, err := afero.ReadFile(cfg.Fs, strings.TrimPrefix(raw, "@"))
		if err != nil {
			return nil, fmt.Errorf("reading variables file: %w", err)
		}
		raw = string(data)
	}
	var vars map[string]any
	if err := json.Unmarshal([]byte(raw), &vars); err != nil {
		return nil, fmt.Errorf("parsing --variables: %w", err)
	}
	return vars, nil
}
