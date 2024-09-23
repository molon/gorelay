package gormrelay

import (
	"context"

	"github.com/molon/gorelay/cursor"
	"github.com/molon/gorelay/pagination"
	"github.com/pkg/errors"
	"github.com/samber/lo"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func createWhereExpr(orderBys []pagination.OrderBy, keyset map[string]any, reverse bool) (clause.Expression, error) {
	ors := make([]clause.Expression, 0, len(orderBys))
	eqs := make([]clause.Expression, 0, len(orderBys))
	for i, orderBy := range orderBys {
		v, ok := keyset[orderBy.Field]
		if !ok {
			return nil, errors.Errorf("missing field %q in keyset", orderBy.Field)
		}

		// TODO: should use gorm schema to convert column name
		column := lo.SnakeCase(orderBy.Field)

		desc := orderBy.Desc
		if reverse {
			desc = !desc
		}

		var expr clause.Expression
		if desc {
			expr = clause.Lt{Column: column, Value: v}
		} else {
			expr = clause.Gt{Column: column, Value: v}
		}

		ands := make([]clause.Expression, len(eqs)+1)
		copy(ands, eqs)
		ands[len(eqs)] = expr
		ors = append(ors, clause.And(ands...))

		if i < len(orderBys)-1 {
			eqs = append(eqs, clause.Eq{Column: column, Value: v})
		}
	}
	return clause.Or(ors...), nil
}

func scopeKeyset(after, before *map[string]any, orderBys []pagination.OrderBy, limit int, fromLast bool) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		var exprs []clause.Expression

		// TODO: need direction up or down ?

		if after != nil {
			expr, err := createWhereExpr(orderBys, *after, false)
			if err != nil {
				db.AddError(err)
				return db
			}
			exprs = append(exprs, expr)
		}

		if before != nil {
			expr, err := createWhereExpr(orderBys, *before, true)
			if err != nil {
				db.AddError(err)
				return db
			}
			exprs = append(exprs, expr)
		}

		if len(orderBys) > 0 {
			orderByColumns := make([]clause.OrderByColumn, 0, len(orderBys))
			for _, orderBy := range orderBys {
				// TODO: should use gorm schema to convert column name
				column := lo.SnakeCase(orderBy.Field)

				desc := orderBy.Desc
				if fromLast {
					desc = !desc
				}
				orderByColumns = append(orderByColumns, clause.OrderByColumn{
					Column: clause.Column{Name: column},
					Desc:   desc,
				})
			}
			exprs = append(exprs, clause.OrderBy{Columns: orderByColumns})
		}

		// Example:
		// db.Clauses(
		// 	clause.Or( // after
		// 		clause.And(
		// 			clause.Gt{Column: "age", Value: 85}, // ASC
		// 		),
		// 		clause.And(
		// 			clause.Eq{Column: "age", Value: 85},
		// 			clause.Lt{Column: "name", Value: "name15"}, // DESC
		// 		),
		// 	),
		// 	clause.Or( // before
		// 		clause.And(
		// 			clause.Lt{Column: "age", Value: 88},
		// 		),
		// 		clause.And(
		// 			clause.Eq{Column: "age", Value: 88},
		// 			clause.Gt{Column: "name", Value: "name12"},
		// 		),
		// 	),
		// 	clause.OrderBy{
		// 		Columns: []clause.OrderByColumn{
		// 			{Column: clause.Field{Name: "age"}, Desc: false},
		// 			{Column: clause.Field{Name: "name"}, Desc: true},
		// 		},
		// 	},
		// )
		return db.Clauses(exprs...).Limit(limit)
	}
}

func NewKeysetFinder[T any](db *gorm.DB) cursor.KeysetFinder[T] {
	return cursor.KeysetFinderFunc[T](func(ctx context.Context, after, before *map[string]any, orderBys []pagination.OrderBy, limit int, fromLast bool) ([]T, error) {
		var nodes []T

		if limit == 0 {
			return nodes, nil
		}

		db := db
		if db.Statement.Context != ctx {
			db = db.WithContext(ctx)
		}

		if err := db.Scopes(scopeKeyset(after, before, orderBys, limit, fromLast)).Find(&nodes).Error; err != nil {
			return nil, errors.Wrap(err, "find")
		}

		if fromLast {
			lo.Reverse(nodes) // TODO: should reverse in db query ?
		}
		return nodes, nil
	})
}

type KeysetCounter[T any] struct {
	db     *gorm.DB
	finder cursor.KeysetFinder[T]
}

func NewKeysetCounter[T any](db *gorm.DB) *KeysetCounter[T] {
	return &KeysetCounter[T]{
		db:     db,
		finder: NewKeysetFinder[T](db),
	}
}

func (a *KeysetCounter[T]) Find(ctx context.Context, after, before *map[string]any, orderBys []pagination.OrderBy, limit int, fromLast bool) ([]T, error) {
	return a.finder.Find(ctx, after, before, orderBys, limit, fromLast)
}

func (a *KeysetCounter[T]) Count(ctx context.Context) (int, error) {
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

func NewKeysetAdapter[T any](db *gorm.DB) pagination.ApplyCursorsFunc[T] {
	return cursor.NewKeysetAdapter(NewKeysetCounter[T](db))
}
