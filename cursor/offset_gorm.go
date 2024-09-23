package cursor

import (
	"context"

	"github.com/molon/gorelay/pagination"
	"github.com/pkg/errors"
	"gorm.io/gorm"
)

func NewGormOffsetFinder[T any](db *gorm.DB) OffsetFinder[T] {
	return OffsetFinderFunc[T](func(ctx context.Context, skip, limit int) ([]T, error) {
		var nodes []T

		if limit == 0 {
			return nodes, nil
		}

		db := db
		if db.Statement.Context != ctx {
			db = db.WithContext(ctx)
		}

		if skip > 0 {
			db = db.Offset(skip)
		}

		if err := db.Limit(limit).Find(&nodes).Error; err != nil {
			return nil, errors.Wrap(err, "find")
		}
		return nodes, nil
	})
}

type GormOffsetCounter[T any] struct {
	db     *gorm.DB
	finder OffsetFinder[T]
}

func NewGormOffsetCounter[T any](db *gorm.DB) *GormOffsetCounter[T] {
	return &GormOffsetCounter[T]{
		db:     db,
		finder: NewGormOffsetFinder[T](db),
	}
}

func (a *GormOffsetCounter[T]) Find(ctx context.Context, skip, limit int) ([]T, error) {
	return a.finder.Find(ctx, skip, limit)
}

func (a *GormOffsetCounter[T]) Count(ctx context.Context) (int, error) {
	db := a.db
	if db.Statement.Context != ctx {
		db = db.WithContext(ctx)
	}

	var totalCount int64
	if err := db.Count(&totalCount).Error; err != nil {
		return 0, errors.Wrap(err, "count")
	}
	return int(totalCount), nil
}

func NewGormOffsetAdapter[T any](db *gorm.DB) pagination.ApplyCursorsFunc[T] {
	return NewOffsetAdapter(NewGormOffsetCounter[T](db))
}
