package gormrelay

import (
	"context"
	"fmt"
	"testing"

	jsoniter "github.com/json-iterator/go"
	"github.com/molon/gorelay/cursor"
	"github.com/molon/gorelay/pagination"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
	"github.com/theplant/testenv"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var db *gorm.DB

func TestMain(m *testing.M) {
	env, err := testenv.New().DBEnable(true).SetUp()
	if err != nil {
		panic(err)
	}
	defer env.TearDown()

	db = env.DB
	db.Logger = db.Logger.LogMode(logger.Info)

	m.Run()
}

func resetDB(t *testing.T) {
	require.NoError(t, db.Exec("DROP TABLE IF EXISTS users").Error)
	require.NoError(t, db.AutoMigrate(&User{}))

	vs := []*User{}
	for i := 0; i < 100; i++ {
		vs = append(vs, &User{
			Name: fmt.Sprintf("name%d", i),
			Age:  100 - i,
		})
	}
	err := db.Session(&gorm.Session{Logger: logger.Discard}).Create(vs).Error
	require.NoError(t, err)
}

type User struct {
	ID   int    `gorm:"primarykey;not null;" json:"id"`
	Name string `gorm:"not null;" json:"name"`
	Age  int    `gorm:"index;not null;" json:"age"`
}

func MustJsonString(v any) string {
	s, err := jsoniter.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(s)
}

func mustEncodeKeysetCursor[T any](node T, keys []string) string {
	cursor, err := cursor.EncodeKeysetCursor(node, keys)
	if err != nil {
		panic(err)
	}
	return cursor
}

func TestScopeKeyset(t *testing.T) {
	{
		sql := db.ToSQL(func(tx *gorm.DB) *gorm.DB {
			tx = tx.Model(&User{}).Scopes(scopeKeyset(
				&map[string]interface{}{"Age": 85},
				nil,
				[]pagination.OrderBy{
					{Field: "Age", Desc: false},
				},
				10,
				false,
			)).Find(&User{})
			require.NoError(t, tx.Error)
			return tx
		})
		require.Equal(t, `SELECT * FROM "users" WHERE "age" > 85 ORDER BY "age" LIMIT 10`, sql)
	}
	{
		sql := db.ToSQL(func(tx *gorm.DB) *gorm.DB {
			tx = tx.Model(&User{}).Scopes(scopeKeyset(
				&map[string]interface{}{"Age": 85},
				&map[string]interface{}{"Age": 88},
				[]pagination.OrderBy{
					{Field: "Age", Desc: false},
				},
				10,
				false,
			)).Find(&User{})
			require.NoError(t, tx.Error)
			return tx
		})
		require.Equal(t, `SELECT * FROM "users" WHERE "age" > 85 AND "age" < 88 ORDER BY "age" LIMIT 10`, sql)
	}
	{
		sql := db.ToSQL(func(tx *gorm.DB) *gorm.DB {
			tx = tx.Model(&User{}).Scopes(scopeKeyset(
				&map[string]interface{}{"Age": 85, "Name": "name15"},
				&map[string]interface{}{"Age": 88, "Name": "name12"},
				[]pagination.OrderBy{
					{Field: "Age", Desc: false},
					{Field: "Name", Desc: true},
				},
				10,
				false,
			)).Find(&User{})
			require.NoError(t, tx.Error)
			return tx
		})
		require.Equal(t, `SELECT * FROM "users" WHERE ("age" > 85 OR ("age" = 85 AND "name" < 'name15')) AND ("age" < 88 OR ("age" = 88 AND "name" > 'name12')) ORDER BY "age","name" DESC LIMIT 10`, sql)
	}
	{
		sql := db.ToSQL(func(tx *gorm.DB) *gorm.DB {
			tx = tx.Model(&User{}).Scopes(scopeKeyset(
				&map[string]interface{}{"Age": 85, "Name": "name15"},
				&map[string]interface{}{"Age": 88, "Name": "name12"},
				[]pagination.OrderBy{
					{Field: "Age", Desc: false},
					{Field: "Name", Desc: true},
				},
				10,
				true, // from last
			)).Find(&User{})
			require.NoError(t, tx.Error)
			return tx
		})
		require.Equal(t, `SELECT * FROM "users" WHERE ("age" > 85 OR ("age" = 85 AND "name" < 'name15')) AND ("age" < 88 OR ("age" = 88 AND "name" > 'name12')) ORDER BY "age" DESC,"name" LIMIT 10`, sql)
	}
	{
		sql := db.ToSQL(func(tx *gorm.DB) *gorm.DB {
			// with extra where
			tx = tx.Model(&User{}).Where("name LIKE ?", "name%").
				Scopes(scopeKeyset(
					&map[string]interface{}{"Age": 85, "Name": "name15"},
					&map[string]interface{}{"Age": 88, "Name": "name12"},
					[]pagination.OrderBy{
						{Field: "Age", Desc: false},
						{Field: "Name", Desc: true},
					},
					10,
					false,
				)).Find(&User{})
			require.NoError(t, tx.Error)
			return tx
		})
		require.Equal(t, `SELECT * FROM "users" WHERE name LIKE 'name%' AND (("age" > 85 OR ("age" = 85 AND "name" < 'name15')) AND ("age" < 88 OR ("age" = 88 AND "name" > 'name12'))) ORDER BY "age","name" DESC LIMIT 10`, sql)
	}
}

func TestKeysetCursor(t *testing.T) {
	resetDB(t)

	defaultOrderBys := []pagination.OrderBy{
		{Field: "ID", Desc: false},
		{Field: "Age", Desc: true},
	}
	defaultOrderByKeys := []string{"ID", "Age"}
	applyCursorsFunc := func(ctx context.Context, req *pagination.ApplyCursorsRequest) (*pagination.ApplyCursorsResponse[*User], error) {
		return NewKeysetAdapter[*User](db)(ctx, req)
	}

	testCases := []struct {
		name             string
		limitIfNotSet    int
		maxLimit         int
		applyCursorsFunc pagination.ApplyCursorsFunc[*User]
		paginateRequest  *pagination.PaginateRequest[*User]
		expectedEdgesLen int
		expectedPageInfo *pagination.PageInfo
		expectedError    string
		expectedPanic    string
	}{
		{
			name:             "Invalid: Both First and Last",
			limitIfNotSet:    10,
			maxLimit:         20,
			applyCursorsFunc: applyCursorsFunc,
			paginateRequest: &pagination.PaginateRequest[*User]{
				First: lo.ToPtr(5),
				Last:  lo.ToPtr(5),
			},
			expectedError: "first and last cannot be used together",
		},
		{
			name:             "Invalid: Negative First",
			limitIfNotSet:    10,
			maxLimit:         20,
			applyCursorsFunc: applyCursorsFunc,
			paginateRequest: &pagination.PaginateRequest[*User]{
				First: lo.ToPtr(-5),
			},
			expectedError: "first must be a non-negative integer",
		},
		{
			name:             "Invalid: Negative Last",
			limitIfNotSet:    10,
			maxLimit:         20,
			applyCursorsFunc: applyCursorsFunc,
			paginateRequest: &pagination.PaginateRequest[*User]{
				Last: lo.ToPtr(-5),
			},
			expectedError: "last must be a non-negative integer",
		},
		{
			name:             "Invalid: No limitIfNotSet",
			limitIfNotSet:    0, // Assuming 0 indicates not set
			maxLimit:         20,
			applyCursorsFunc: applyCursorsFunc,
			paginateRequest:  &pagination.PaginateRequest[*User]{},
			expectedPanic:    "limitIfNotSet must be greater than 0",
		},
		{
			name:             "Invalid: maxLimit < limitIfNotSet",
			limitIfNotSet:    10,
			maxLimit:         8,
			applyCursorsFunc: applyCursorsFunc,
			paginateRequest:  &pagination.PaginateRequest[*User]{},
			expectedPanic:    "maxLimit must be greater than or equal to limitIfNotSet",
		},
		{
			name:             "Invalid: No applyCursorsFunc",
			limitIfNotSet:    10,
			maxLimit:         20,
			applyCursorsFunc: nil, // No ApplyCursorsFunc provided
			paginateRequest:  &pagination.PaginateRequest[*User]{},
			expectedPanic:    "applyCursorsFunc must be set",
		},
		{
			name:             "Invalid: first > maxLimit",
			limitIfNotSet:    10,
			maxLimit:         20,
			applyCursorsFunc: applyCursorsFunc,
			paginateRequest: &pagination.PaginateRequest[*User]{
				First: lo.ToPtr(21),
			},
			expectedError: "first must be less than or equal to max limit",
		},
		{
			name:             "Invalid: last > maxLimit",
			limitIfNotSet:    10,
			maxLimit:         20,
			applyCursorsFunc: applyCursorsFunc,
			paginateRequest: &pagination.PaginateRequest[*User]{
				Last: lo.ToPtr(21),
			},
			expectedError: "last must be less than or equal to max limit",
		},
		{
			name:             "Invalid: after == before",
			limitIfNotSet:    10,
			maxLimit:         20,
			applyCursorsFunc: applyCursorsFunc,
			paginateRequest: &pagination.PaginateRequest[*User]{
				After: lo.ToPtr(mustEncodeKeysetCursor(
					&User{ID: 9 + 1, Name: "name9", Age: 91}, defaultOrderByKeys,
				)),
				Before: lo.ToPtr(mustEncodeKeysetCursor(
					&User{ID: 9 + 1, Name: "name9", Age: 91}, defaultOrderByKeys,
				)),
			},
			expectedError: "after == before",
		},
		{
			name:             "Limit if not set",
			limitIfNotSet:    10,
			maxLimit:         20,
			applyCursorsFunc: applyCursorsFunc,
			paginateRequest:  &pagination.PaginateRequest[*User]{},
			expectedEdgesLen: 10,
			expectedPageInfo: &pagination.PageInfo{
				TotalCount:      100,
				HasNextPage:     true,
				HasPreviousPage: false,
				StartCursor: lo.ToPtr(mustEncodeKeysetCursor(
					&User{ID: 0 + 1, Name: "name0", Age: 100}, defaultOrderByKeys,
				)),
				EndCursor: lo.ToPtr(mustEncodeKeysetCursor(
					&User{ID: 9 + 1, Name: "name9", Age: 91}, defaultOrderByKeys,
				)),
			},
		},
		{
			name:             "First 2 after cursor 0",
			limitIfNotSet:    10,
			maxLimit:         20,
			applyCursorsFunc: applyCursorsFunc,
			paginateRequest: &pagination.PaginateRequest[*User]{
				After: lo.ToPtr(mustEncodeKeysetCursor(
					&User{ID: 0 + 1, Name: "name0", Age: 100}, defaultOrderByKeys,
				)),
				First: lo.ToPtr(2),
			},
			expectedEdgesLen: 2,
			expectedPageInfo: &pagination.PageInfo{
				TotalCount:      100,
				HasNextPage:     true,
				HasPreviousPage: true,
				StartCursor: lo.ToPtr(mustEncodeKeysetCursor(
					&User{ID: 1 + 1, Name: "name1", Age: 99}, defaultOrderByKeys,
				)),
				EndCursor: lo.ToPtr(mustEncodeKeysetCursor(
					&User{ID: 2 + 1, Name: "name2", Age: 98}, defaultOrderByKeys,
				)),
			},
		},
		{
			name:             "First 2 without after cursor",
			limitIfNotSet:    10,
			maxLimit:         20,
			applyCursorsFunc: applyCursorsFunc,
			paginateRequest: &pagination.PaginateRequest[*User]{
				First: lo.ToPtr(2),
			},
			expectedEdgesLen: 2,
			expectedPageInfo: &pagination.PageInfo{
				TotalCount:      100,
				HasNextPage:     true,
				HasPreviousPage: false,
				StartCursor: lo.ToPtr(mustEncodeKeysetCursor(
					&User{ID: 0 + 1, Name: "name0", Age: 100}, defaultOrderByKeys,
				)),
				EndCursor: lo.ToPtr(mustEncodeKeysetCursor(
					&User{ID: 1 + 1, Name: "name1", Age: 99}, defaultOrderByKeys,
				)),
			},
		},
		{
			name:             "Last 2 before cursor 8",
			limitIfNotSet:    10,
			maxLimit:         20,
			applyCursorsFunc: applyCursorsFunc,
			paginateRequest: &pagination.PaginateRequest[*User]{
				Before: lo.ToPtr(mustEncodeKeysetCursor(
					&User{ID: 8 + 1, Name: "name8", Age: 92}, defaultOrderByKeys,
				)),
				Last: lo.ToPtr(2),
			},
			expectedEdgesLen: 2,
			expectedPageInfo: &pagination.PageInfo{
				TotalCount:      100,
				HasNextPage:     true,
				HasPreviousPage: true,
				StartCursor: lo.ToPtr(mustEncodeKeysetCursor(
					&User{ID: 6 + 1, Name: "name6", Age: 94}, defaultOrderByKeys,
				)),
				EndCursor: lo.ToPtr(mustEncodeKeysetCursor(
					&User{ID: 7 + 1, Name: "name7", Age: 93}, defaultOrderByKeys,
				)),
			},
		},
		{
			name:             "After cursor 0, Before cursor 8, First 5",
			limitIfNotSet:    10,
			maxLimit:         20,
			applyCursorsFunc: applyCursorsFunc,
			paginateRequest: &pagination.PaginateRequest[*User]{
				After: lo.ToPtr(mustEncodeKeysetCursor(
					&User{ID: 0 + 1, Name: "name0", Age: 100}, defaultOrderByKeys,
				)),
				Before: lo.ToPtr(mustEncodeKeysetCursor(
					&User{ID: 8 + 1, Name: "name8", Age: 92}, defaultOrderByKeys,
				)),
				First: lo.ToPtr(5),
			},
			expectedEdgesLen: 5,
			expectedPageInfo: &pagination.PageInfo{
				TotalCount:      100,
				HasNextPage:     true,
				HasPreviousPage: true,
				StartCursor: lo.ToPtr(mustEncodeKeysetCursor(
					&User{ID: 1 + 1, Name: "name1", Age: 99}, defaultOrderByKeys,
				)),
				EndCursor: lo.ToPtr(mustEncodeKeysetCursor(
					&User{ID: 5 + 1, Name: "name5", Age: 95}, defaultOrderByKeys,
				)),
			},
		},
		{
			name:             "After cursor 0, Before cursor 4, First 8",
			limitIfNotSet:    10,
			maxLimit:         20,
			applyCursorsFunc: applyCursorsFunc,
			paginateRequest: &pagination.PaginateRequest[*User]{
				After: lo.ToPtr(mustEncodeKeysetCursor(
					&User{ID: 0 + 1, Name: "name0", Age: 100}, defaultOrderByKeys,
				)),
				Before: lo.ToPtr(mustEncodeKeysetCursor(
					&User{ID: 4 + 1, Name: "name4", Age: 96}, defaultOrderByKeys,
				)),
				First: lo.ToPtr(8),
			},
			expectedEdgesLen: 3,
			expectedPageInfo: &pagination.PageInfo{
				TotalCount:      100,
				HasNextPage:     true,
				HasPreviousPage: true,
				StartCursor: lo.ToPtr(mustEncodeKeysetCursor(
					&User{ID: 1 + 1, Name: "name1", Age: 99}, defaultOrderByKeys,
				)),
				EndCursor: lo.ToPtr(mustEncodeKeysetCursor(
					&User{ID: 3 + 1, Name: "name3", Age: 97}, defaultOrderByKeys,
				)),
			},
		},
		{
			name:             "After cursor 99",
			limitIfNotSet:    10,
			maxLimit:         20,
			applyCursorsFunc: applyCursorsFunc,
			paginateRequest: &pagination.PaginateRequest[*User]{
				After: lo.ToPtr(mustEncodeKeysetCursor(
					&User{ID: 99 + 1, Name: "name99", Age: 1}, defaultOrderByKeys,
				)),
			},
			expectedEdgesLen: 0,
			expectedPageInfo: &pagination.PageInfo{
				TotalCount:      100,
				HasNextPage:     false,
				HasPreviousPage: true,
				StartCursor:     nil,
				EndCursor:       nil,
			},
		},
		{
			name:             "Before cursor 0",
			limitIfNotSet:    10,
			maxLimit:         20,
			applyCursorsFunc: applyCursorsFunc,
			paginateRequest: &pagination.PaginateRequest[*User]{
				Before: lo.ToPtr(mustEncodeKeysetCursor(
					&User{ID: 0 + 1, Name: "name0", Age: 100}, defaultOrderByKeys,
				)),
			},
			expectedEdgesLen: 0,
			expectedPageInfo: &pagination.PageInfo{
				TotalCount:      100,
				HasNextPage:     true,
				HasPreviousPage: false,
				StartCursor:     nil,
				EndCursor:       nil,
			},
		},
		{
			name:             "First 200",
			limitIfNotSet:    10,
			maxLimit:         300,
			applyCursorsFunc: applyCursorsFunc,
			paginateRequest: &pagination.PaginateRequest[*User]{
				First: lo.ToPtr(200),
			},
			expectedEdgesLen: 100,
			expectedPageInfo: &pagination.PageInfo{
				TotalCount:      100,
				HasNextPage:     false,
				HasPreviousPage: false,
				StartCursor: lo.ToPtr(mustEncodeKeysetCursor(
					&User{ID: 0 + 1, Name: "name0", Age: 100}, defaultOrderByKeys,
				)),
				EndCursor: lo.ToPtr(mustEncodeKeysetCursor(
					&User{ID: 99 + 1, Name: "name99", Age: 1}, defaultOrderByKeys,
				)),
			},
		},
		{
			name:             "First 0",
			limitIfNotSet:    10,
			maxLimit:         20,
			applyCursorsFunc: applyCursorsFunc,
			paginateRequest: &pagination.PaginateRequest[*User]{
				First: lo.ToPtr(0),
			},
			expectedEdgesLen: 0,
			expectedPageInfo: &pagination.PageInfo{
				TotalCount:      100,
				HasNextPage:     true,
				HasPreviousPage: false,
				StartCursor:     nil,
				EndCursor:       nil,
			},
		},
		{
			name:             "Last 0",
			limitIfNotSet:    10,
			maxLimit:         20,
			applyCursorsFunc: applyCursorsFunc,
			paginateRequest: &pagination.PaginateRequest[*User]{
				Last: lo.ToPtr(0),
			},
			expectedEdgesLen: 0,
			expectedPageInfo: &pagination.PageInfo{
				TotalCount:      100,
				HasNextPage:     false,
				HasPreviousPage: true,
				StartCursor:     nil,
				EndCursor:       nil,
			},
		},
		{
			name:             "After cursor 95, First 10",
			limitIfNotSet:    10,
			maxLimit:         20,
			applyCursorsFunc: applyCursorsFunc,
			paginateRequest: &pagination.PaginateRequest[*User]{
				After: lo.ToPtr(mustEncodeKeysetCursor(
					&User{ID: 95 + 1, Name: "name95", Age: 5}, defaultOrderByKeys,
				)),
				First: lo.ToPtr(10),
			},
			expectedEdgesLen: 4,
			expectedPageInfo: &pagination.PageInfo{
				TotalCount:      100,
				HasNextPage:     false,
				HasPreviousPage: true,
				StartCursor: lo.ToPtr(mustEncodeKeysetCursor(
					&User{ID: 96 + 1, Name: "name96", Age: 4}, defaultOrderByKeys,
				)),
				EndCursor: lo.ToPtr(mustEncodeKeysetCursor(
					&User{ID: 99 + 1, Name: "name99", Age: 1}, defaultOrderByKeys,
				)),
			},
		},
		{
			name:             "Before cursor 4, Last 10",
			limitIfNotSet:    10,
			maxLimit:         20,
			applyCursorsFunc: applyCursorsFunc,
			paginateRequest: &pagination.PaginateRequest[*User]{
				Before: lo.ToPtr(mustEncodeKeysetCursor(
					&User{ID: 4 + 1, Name: "name4", Age: 96}, defaultOrderByKeys,
				)),
				Last: lo.ToPtr(10),
			},
			expectedEdgesLen: 4,
			expectedPageInfo: &pagination.PageInfo{
				TotalCount:      100,
				HasNextPage:     true,
				HasPreviousPage: false,
				StartCursor: lo.ToPtr(mustEncodeKeysetCursor(
					&User{ID: 0 + 1, Name: "name0", Age: 100}, defaultOrderByKeys,
				)),
				EndCursor: lo.ToPtr(mustEncodeKeysetCursor(
					&User{ID: 3 + 1, Name: "name3", Age: 97}, defaultOrderByKeys,
				)),
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if tc.expectedPanic != "" {
				require.PanicsWithValue(t, tc.expectedPanic, func() {
					pagination.New(tc.maxLimit, tc.limitIfNotSet, defaultOrderBys, tc.applyCursorsFunc)
				})
				return
			}

			p := pagination.New(tc.maxLimit, tc.limitIfNotSet, defaultOrderBys, tc.applyCursorsFunc)
			resp, err := p.Paginate(context.Background(), tc.paginateRequest)

			if tc.expectedError != "" {
				require.Error(t, err)
				require.ErrorContains(t, err, tc.expectedError)
				return
			}

			require.NoError(t, err)
			require.Len(t, resp.Edges, tc.expectedEdgesLen)
			require.Equal(t, tc.expectedPageInfo, resp.PageInfo)
		})
	}
}

func TestUnexpectOrderBys(t *testing.T) {
	require.PanicsWithValue(t, "orderBysIfNotSet must be set", func() {
		pagination.New(10, 10, nil, func(ctx context.Context, req *pagination.ApplyCursorsRequest) (*pagination.ApplyCursorsResponse[*User], error) {
			return nil, nil
		})
	})
	require.PanicsWithValue(t, "orderBysIfNotSet must be set", func() {
		pagination.New(10, 10, []pagination.OrderBy{}, func(ctx context.Context, req *pagination.ApplyCursorsRequest) (*pagination.ApplyCursorsResponse[*User], error) {
			return nil, nil
		})
	})

	p := pagination.New(10, 10,
		[]pagination.OrderBy{
			{Field: "ID", Desc: false},
		}, func(ctx context.Context, req *pagination.ApplyCursorsRequest) (*pagination.ApplyCursorsResponse[*User], error) {
			return nil, nil
		},
	)
	resp, err := p.Paginate(context.Background(), &pagination.PaginateRequest[*User]{
		First: lo.ToPtr(10),
		OrderBys: []pagination.OrderBy{
			{Field: "ID", Desc: false},
			{Field: "ID", Desc: true},
		},
	})
	require.ErrorContains(t, err, "duplicated order by fields [ID]")
	require.Nil(t, resp)
}

func TestContext(t *testing.T) {
	resetDB(t)

	testCase := func(t *testing.T, f func(db *gorm.DB) pagination.ApplyCursorsFunc[*User]) {
		{
			p := pagination.New(
				10, 10,
				[]pagination.OrderBy{
					{Field: "ID", Desc: false},
				}, func(ctx context.Context, req *pagination.ApplyCursorsRequest) (*pagination.ApplyCursorsResponse[*User], error) {
					return f(db)(ctx, req)
				},
			)
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			resp, err := p.Paginate(ctx, &pagination.PaginateRequest[*User]{
				First: lo.ToPtr(10),
			})
			require.ErrorContains(t, err, "context canceled")
			require.Nil(t, resp)
		}

		{
			p := pagination.New(
				10, 10,
				[]pagination.OrderBy{
					{Field: "ID", Desc: false},
				}, func(ctx context.Context, req *pagination.ApplyCursorsRequest) (*pagination.ApplyCursorsResponse[*User], error) {
					// Set WithContext here
					return f(db.WithContext(ctx))(ctx, req)
				},
			)
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			resp, err := p.Paginate(ctx, &pagination.PaginateRequest[*User]{
				First: lo.ToPtr(10),
			})
			require.ErrorContains(t, err, "context canceled")
			require.Nil(t, resp)
		}
	}
	t.Run("keyset", func(t *testing.T) { testCase(t, NewKeysetAdapter) })
	t.Run("offset", func(t *testing.T) { testCase(t, NewOffsetAdapter) })
}

// func TestAny(t *testing.T) {
// 	resetDB(t)

// 	p := pagination.New(
// 		10, 10,
// 		[]pagination.OrderBy{
// 			{Field: "ID", Desc: false},
// 		}, func(ctx context.Context, req *pagination.ApplyCursorsRequest) (*pagination.ApplyCursorsResponse[any], error) {
// 			return NewKeysetAdapter[any](db.Model(&User{}))(ctx, req)
// 		},
// 	)
// 	resp, err := p.Paginate(context.Background(), &pagination.PaginateRequest[any]{
// 		First: lo.ToPtr(10),
// 	})
// 	require.NoError(t, err)
// 	require.Len(t, resp.Edges, 10)
// }
