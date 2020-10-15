package trader

import "context"

type Trader interface {
	// Close release resource allocated by New or Init
	Close(ctx context.Context)
	Print(ctx context.Context)
	Clear(ctx context.Context)
	// ws, start / stop
	Start(ctx context.Context)
	Stop(ctx context.Context)
	// Restful, run once
	Run(ctx context.Context)
}
