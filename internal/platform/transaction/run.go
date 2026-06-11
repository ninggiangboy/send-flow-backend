package transaction

import (
	"context"
	"fmt"
)

func RunInTx(ctx context.Context, uow UnitOfWork, fn func(ctx context.Context) error) error {
	if uow == nil {
		return fmt.Errorf("RunInTx: UnitOfWork is nil")
	}
	return uow.WithinTx(ctx, fn)
}
