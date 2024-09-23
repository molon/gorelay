package gormrelay

import (
	"context"

	"github.com/molon/gorelay/cursor"
	"github.com/molon/gorelay/pagination"
	"github.com/pkg/errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func NewOffsetFinder[T any](db *gorm.DB) cursor.OffsetFinder[T] {
	return cursor.OffsetFinderFunc[T](func(ctx context.Context, orderBys []pagination.OrderBy, skip, limit int) ([]T, error) {
		var nodes []T

		if limit == 0 {
			return nodes, nil
		}

		db := db
		if db.Statement.Context != ctx {
			db = db.WithContext(ctx)
		}

		if db.Statement.Model == nil {
			var t T
			db = db.Model(t)
		}

		if skip > 0 {
			db = db.Offset(skip)
		}

		if len(orderBys) > 0 {
			s, err := parseSchema(db, db.Statement.Model)
			if err != nil {
				return nil, errors.Wrap(err, "parse schema")
			}

			orderByColumns := make([]clause.OrderByColumn, 0, len(orderBys))
			for _, orderBy := range orderBys {
				field, ok := s.FieldsByName[orderBy.Field]
				if !ok {
					return nil, errors.Errorf("missing field %q in schema", orderBy.Field)
				}

				orderByColumns = append(orderByColumns, clause.OrderByColumn{
					Column: clause.Column{Name: field.DBName},
					Desc:   orderBy.Desc,
				})
			}
			db = db.Order(clause.OrderBy{Columns: orderByColumns})
		}

		if err := db.Limit(limit).Find(&nodes).Error; err != nil {
			return nil, errors.Wrap(err, "find")
		}
		return nodes, nil
	})
}

type OffsetCounter[T any] struct {
	db     *gorm.DB
	finder cursor.OffsetFinder[T]
}

func NewOffsetCounter[T any](db *gorm.DB) *OffsetCounter[T] {
	return &OffsetCounter[T]{
		db:     db,
		finder: NewOffsetFinder[T](db),
	}
}

func (a *OffsetCounter[T]) Find(ctx context.Context, orderBys []pagination.OrderBy, skip, limit int) ([]T, error) {
	return a.finder.Find(ctx, orderBys, skip, limit)
}

func (a *OffsetCounter[T]) Count(ctx context.Context) (int, error) {
	db := a.db
	if db.Statement.Context != ctx {
		db = db.WithContext(ctx)
	}

	if db.Statement.Model == nil {
		var t T
		db = db.Model(t)
	}

	var totalCount int64
	if err := db.Count(&totalCount).Error; err != nil {
		return 0, errors.Wrap(err, "count")
	}
	return int(totalCount), nil
}

func NewOffsetAdapter[T any](db *gorm.DB) pagination.ApplyCursorsFunc[T] {
	return cursor.NewOffsetAdapter(NewOffsetCounter[T](db))
}
