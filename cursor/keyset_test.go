package cursor_test

import (
	"log"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type OrderBy struct {
	Column string
	Desc   bool
}

func Paginate(orderBys []OrderBy, keyset map[string]any) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		ors := make([]clause.Expression, 0, len(orderBys))
		eqs := make([]clause.Expression, 0, len(orderBys))
		for i, orderBy := range orderBys {
			v, ok := keyset[orderBy.Column]
			if !ok {
				db.AddError(errors.Errorf("missing key %q in cursor", orderBy.Column))
				return db
			}

			var expr clause.Expression
			if orderBy.Desc {
				expr = clause.Lt{Column: orderBy.Column, Value: v}
			} else {
				expr = clause.Gt{Column: orderBy.Column, Value: v}
			}

			ands := make([]clause.Expression, len(eqs)+1)
			copy(ands, eqs)
			ands[len(eqs)] = expr
			ors = append(ors, clause.And(ands...))

			if i < len(orderBys)-1 {
				eqs = append(eqs, clause.Eq{Column: orderBy.Column, Value: v})
			}
		}

		orderByColumns := make([]clause.OrderByColumn, 0, len(orderBys))
		for _, orderBy := range orderBys {
			orderByColumns = append(orderByColumns, clause.OrderByColumn{
				Column: clause.Column{Name: orderBy.Column},
				Desc:   orderBy.Desc,
			})
		}

		// Example:
		// db.Clauses(
		// 	clause.Or(
		// 		clause.And(
		// 			clause.Gt{Column: "age", Value: 85}, // ASC
		// 		),
		// 		clause.And(
		// 			clause.Eq{Column: "age", Value: 85},
		// 			clause.Lt{Column: "name", Value: "name15"}, // DESC
		// 		),
		// 	),
		// 	clause.OrderBy{
		// 		Columns: []clause.OrderByColumn{
		// 			{Column: clause.Column{Name: "age"}, Desc: false},
		// 			{Column: clause.Column{Name: "name"}, Desc: true},
		// 		},
		// 	},
		// )
		return db.Clauses(
			clause.Or(ors...),
			clause.OrderBy{Columns: orderByColumns},
		)
	}
}

func TestGormClause(t *testing.T) {
	resetDB(t)
	{
		orderBys := []OrderBy{
			{Column: "age", Desc: false},
			{Column: "name", Desc: true},
		}
		keyset := map[string]any{
			"age":  85,
			"name": "name15",
		}
		var users []*User
		err := db.Where("name LIKE ?", "name1%").Scopes(Paginate(orderBys, keyset)).Find(&users).Error
		require.NoError(t, err)
		log.Println(MustJsonString(users))
	}
}
