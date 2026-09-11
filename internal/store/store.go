package store

import (
	"context"
	"errors"

	"example.com/config-snapshot-registry/internal/domain"
)

var ErrConflict = errors.New("store revision conflict")

type Store interface {
	Read(context.Context) (domain.State, error)
	CompareAndSwap(context.Context, uint64, domain.State) error
}
