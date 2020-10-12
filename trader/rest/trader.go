package rest

import "context"

type Trader interface {
	Init(ctx context.Context)
	Close(ctx context.Context)
	Print(ctx context.Context) error
	Clear(ctx context.Context) error
	Start(ctx context.Context) error
}
