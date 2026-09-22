package service

import (
	"context"
	"encoding/json"
)

// GraphQLService is the raw passthrough surface behind `linear api`: one
// authenticated GraphQL operation in, the raw data document out.
type GraphQLService interface {
	ExecGraphQL(ctx context.Context, query string, variables map[string]any) (json.RawMessage, error)
}

// Compile-time proof that Service satisfies the passthrough surface.
var _ GraphQLService = (*Service)(nil)

// ExecGraphQL posts one arbitrary GraphQL operation and returns the raw
// data document; exec already surfaces non-200 statuses and a non-empty
// GraphQL errors array as errors.
func (s *Service) ExecGraphQL(ctx context.Context, query string, variables map[string]any) (json.RawMessage, error) {
	return s.exec(ctx, query, variables)
}
