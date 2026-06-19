package worker

import "context"

type aliasRunner struct {
	alias string
	key   string
	inner Runner
}

func newAliasRunner(alias string, inner Runner) Runner {
	key := inner.Name()
	if keyed, ok := inner.(keyedRunner); ok {
		key = keyed.RunnerKey()
	}
	return aliasRunner{
		alias: alias,
		key:   key,
		inner: inner,
	}
}

func (r aliasRunner) Name() string {
	return r.alias
}

func (r aliasRunner) RunnerKey() string {
	return r.key
}

func (r aliasRunner) Run(ctx context.Context) error {
	return r.inner.Run(ctx)
}
