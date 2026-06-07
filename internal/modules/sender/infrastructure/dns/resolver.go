package dns

import (
	"context"
	"net"
)

type Resolver struct {
	resolver *net.Resolver
}

func NewResolver() *Resolver {
	return &Resolver{resolver: net.DefaultResolver}
}

func (r *Resolver) LookupTXT(ctx context.Context, host string) ([]string, error) {
	return r.resolver.LookupTXT(ctx, host)
}

func (r *Resolver) LookupCNAME(ctx context.Context, host string) (string, error) {
	return r.resolver.LookupCNAME(ctx, host)
}
