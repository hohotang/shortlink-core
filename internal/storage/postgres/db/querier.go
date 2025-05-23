// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.29.0

package db

import (
	"context"
)

type Querier interface {
	FindShortIDByURL(ctx context.Context, originalUrl string) (string, error)
	GetURL(ctx context.Context, shortID string) (string, error)
	StoreWithID(ctx context.Context, arg StoreWithIDParams) error
}

var _ Querier = (*Queries)(nil)
