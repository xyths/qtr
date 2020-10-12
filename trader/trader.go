package trader

import "context"

type Trader interface {
	Start(ctx context.Context, dry bool)
	Stop(ctx context.Context)
	Print(ctx context.Context)
	Clear(ctx context.Context)
	Close(ctx context.Context)
}
