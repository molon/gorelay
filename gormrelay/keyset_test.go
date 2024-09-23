package gormrelay_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/molon/gorelay/cursor"
	"github.com/molon/gorelay/gormrelay"
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

	if err = db.AutoMigrate(&User{}); err != nil {
		panic(err)
	}

	m.Run()
}

func resetDB(t *testing.T) {
	db.Exec("DELETE FROM users")

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
	ID   uint   `gorm:"primarykey;not null;"`
	Name string `gorm:"not null;"`
	Age  int    `gorm:"index;not null;"`
}

func MustJsonString(v any) string {
	s, err := json.Marshal(v)
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

func TestKeysetCursor(t *testing.T) {
	resetDB(t)

	defaultOrderBys := []pagination.OrderBy{
		{Field: "ID", Desc: false},
	}
	applyCursorsFunc := func(ctx context.Context, req *pagination.ApplyCursorsRequest) (*pagination.ApplyCursorsResponse[*User], error) {
		return gormrelay.NewKeysetAdapter[*User](db.Model(&User{}))(ctx, req) // TODO: should Model be called here?
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
		// {
		// 	name:             "Invalid: Both First and Last",
		// 	limitIfNotSet:    10,
		// 	maxLimit:         20,
		// 	applyCursorsFunc: applyCursorsFunc,
		// 	paginateRequest: &pagination.PaginateRequest[*User]{
		// 		First: lo.ToPtr(5),
		// 		Last:  lo.ToPtr(5),
		// 	},
		// 	expectedError: "first and last cannot be used together",
		// },
		// {
		// 	name:             "Invalid: Negative First",
		// 	limitIfNotSet:    10,
		// 	maxLimit:         20,
		// 	applyCursorsFunc: applyCursorsFunc,
		// 	paginateRequest: &pagination.PaginateRequest[*User]{
		// 		First: lo.ToPtr(-5),
		// 	},
		// 	expectedError: "first must be a non-negative integer",
		// },
		// {
		// 	name:             "Invalid: Negative Last",
		// 	limitIfNotSet:    10,
		// 	maxLimit:         20,
		// 	applyCursorsFunc: applyCursorsFunc,
		// 	paginateRequest: &pagination.PaginateRequest[*User]{
		// 		Last: lo.ToPtr(-5),
		// 	},
		// 	expectedError: "last must be a non-negative integer",
		// },
		// {
		// 	name:             "Invalid: No limitIfNotSet",
		// 	limitIfNotSet:    0, // Assuming 0 indicates not set
		// 	maxLimit:         20,
		// 	applyCursorsFunc: applyCursorsFunc,
		// 	paginateRequest:  &pagination.PaginateRequest[*User]{},
		// 	expectedPanic:    "limitIfNotSet must be greater than 0",
		// },
		// {
		// 	name:             "Invalid: maxLimit < limitIfNotSet",
		// 	limitIfNotSet:    10,
		// 	maxLimit:         8,
		// 	applyCursorsFunc: applyCursorsFunc,
		// 	paginateRequest:  &pagination.PaginateRequest[*User]{},
		// 	expectedPanic:    "maxLimit must be greater than or equal to limitIfNotSet",
		// },
		// {
		// 	name:             "Invalid: No applyCursorsFunc",
		// 	limitIfNotSet:    10,
		// 	maxLimit:         20,
		// 	applyCursorsFunc: nil, // No ApplyCursorsFunc provided
		// 	paginateRequest:  &pagination.PaginateRequest[*User]{},
		// 	expectedPanic:    "applyCursorsFunc must be set",
		// },
		// {
		// 	name:             "Invalid: first > maxLimit",
		// 	limitIfNotSet:    10,
		// 	maxLimit:         20,
		// 	applyCursorsFunc: applyCursorsFunc,
		// 	paginateRequest: &pagination.PaginateRequest[*User]{
		// 		First: lo.ToPtr(21),
		// 	},
		// 	expectedError: "first must be less than or equal to max limit",
		// },
		// {
		// 	name:             "Invalid: last > maxLimit",
		// 	limitIfNotSet:    10,
		// 	maxLimit:         20,
		// 	applyCursorsFunc: applyCursorsFunc,
		// 	paginateRequest: &pagination.PaginateRequest[*User]{
		// 		Last: lo.ToPtr(21),
		// 	},
		// 	expectedError: "last must be less than or equal to max limit",
		// },
		// {
		// 	name:             "Invalid: after == before",
		// 	limitIfNotSet:    10,
		// 	maxLimit:         20,
		// 	applyCursorsFunc: applyCursorsFunc,
		// 	paginateRequest: &pagination.PaginateRequest[*User]{
		// 		After:  lo.ToPtr(mustEncodeKeysetCursor(&User{ID: "id1"}, []string{"id"})),
		// 		Before: lo.ToPtr(mustEncodeKeysetCursor(&User{ID: "id1"}, []string{"id"})),
		// 	},
		// 	expectedError: "after == before",
		// },
		// {
		// 	name:             "Limit if not set",
		// 	limitIfNotSet:    10,
		// 	maxLimit:         20,
		// 	applyCursorsFunc: applyCursorsFunc,
		// 	paginateRequest:  &pagination.PaginateRequest[*User]{},
		// 	expectedEdgesLen: 10,
		// 	expectedPageInfo: &pagination.PageInfo{
		// 		TotalCount:      100,
		// 		HasNextPage:     true,
		// 		HasPreviousPage: false,
		// 		StartCursor: lo.ToPtr(mustEncodeKeysetCursor(
		// 			&User{ID: 0 + 1, Name: "name0", Age: 100}, []string{"ID"},
		// 		)),
		// 		EndCursor: lo.ToPtr(mustEncodeKeysetCursor(
		// 			&User{ID: 9 + 1, Name: "name9", Age: 81}, []string{"ID"},
		// 		)),
		// 	},
		// },
		// {
		// 	name:             "First 2 after cursor 0",
		// 	limitIfNotSet:    10,
		// 	maxLimit:         20,
		// 	applyCursorsFunc: applyCursorsFunc,
		// 	paginateRequest: &pagination.PaginateRequest[*User]{
		// 		After: lo.ToPtr(mustEncodeKeysetCursor(
		// 			&User{ID: 0 + 1, Name: "name0", Age: 100}, []string{"ID"},
		// 		)),
		// 		First: lo.ToPtr(2),
		// 	},
		// 	expectedEdgesLen: 2,
		// 	expectedPageInfo: &pagination.PageInfo{
		// 		TotalCount:      100,
		// 		HasNextPage:     true,
		// 		HasPreviousPage: true,
		// 		StartCursor: lo.ToPtr(mustEncodeKeysetCursor(
		// 			&User{ID: 1 + 1, Name: "name1", Age: 99}, []string{"ID"},
		// 		)),
		// 		EndCursor: lo.ToPtr(mustEncodeKeysetCursor(
		// 			&User{ID: 2 + 1, Name: "name2", Age: 98}, []string{"ID"},
		// 		)),
		// 	},
		// },
		// {
		// 	name:             "First 2 without after cursor",
		// 	limitIfNotSet:    10,
		// 	maxLimit:         20,
		// 	applyCursorsFunc: applyCursorsFunc,
		// 	paginateRequest: &pagination.PaginateRequest[*User]{
		// 		First: lo.ToPtr(2),
		// 	},
		// 	expectedEdgesLen: 2,
		// 	expectedPageInfo: &pagination.PageInfo{
		// 		TotalCount:      100,
		// 		HasNextPage:     true,
		// 		HasPreviousPage: false,
		// 		StartCursor: lo.ToPtr(mustEncodeKeysetCursor(
		// 			&User{ID: 0 + 1, Name: "name0", Age: 100}, []string{"ID"},
		// 		)),
		// 		EndCursor: lo.ToPtr(mustEncodeKeysetCursor(
		// 			&User{ID: 1 + 1, Name: "name1", Age: 99}, []string{"ID"},
		// 		)),
		// 	},
		// },
		// {
		// 	name:             "Last 2 before cursor 8",
		// 	limitIfNotSet:    10,
		// 	maxLimit:         20,
		// 	applyCursorsFunc: applyCursorsFunc,
		// 	paginateRequest: &pagination.PaginateRequest[*User]{
		// 		Before: lo.ToPtr(mustEncodeKeysetCursor(
		// 			&User{ID: 8 + 1, Name: "name8", Age: 92}, []string{"ID"},
		// 		)),
		// 		Last: lo.ToPtr(2),
		// 	},
		// 	expectedEdgesLen: 2,
		// 	expectedPageInfo: &pagination.PageInfo{
		// 		TotalCount:      100,
		// 		HasNextPage:     true,
		// 		HasPreviousPage: true,
		// 		StartCursor: lo.ToPtr(mustEncodeKeysetCursor(
		// 			&User{ID: 6 + 1, Name: "name6", Age: 94}, []string{"ID"},
		// 		)),
		// 		EndCursor: lo.ToPtr(mustEncodeKeysetCursor(
		// 			&User{ID: 7 + 1, Name: "name7", Age: 93}, []string{"ID"},
		// 		)),
		// 	},
		// },
		{
			name:             "After cursor 0, Before cursor 8, First 5",
			limitIfNotSet:    10,
			maxLimit:         20,
			applyCursorsFunc: applyCursorsFunc,
			paginateRequest: &pagination.PaginateRequest[*User]{
				After: lo.ToPtr(mustEncodeKeysetCursor(
					&User{ID: 0 + 1, Name: "name0", Age: 100}, []string{"ID"},
				)),
				Before: lo.ToPtr(mustEncodeKeysetCursor(
					&User{ID: 8 + 1, Name: "name8", Age: 92}, []string{"ID"},
				)),
				First: lo.ToPtr(5),
			},
			expectedEdgesLen: 5,
			expectedPageInfo: &pagination.PageInfo{
				TotalCount:      100,
				HasNextPage:     true,
				HasPreviousPage: true,
				StartCursor: lo.ToPtr(mustEncodeKeysetCursor(
					&User{ID: 1 + 1, Name: "name1", Age: 99}, []string{"ID"},
				)),
				EndCursor: lo.ToPtr(mustEncodeKeysetCursor(
					&User{ID: 5 + 1, Name: "name5", Age: 95}, []string{"ID"},
				)),
			},
		},
		// {
		// 	name:             "After cursor 0, Before cursor 4, First 8",
		// 	limitIfNotSet:    10,
		// 	maxLimit:         20,
		// 	applyCursorsFunc: applyCursorsFunc,
		// 	paginateRequest: &pagination.PaginateRequest[*User]{
		// 		After:  lo.ToPtr(toOffsetCursor(0)),
		// 		Before: lo.ToPtr(toOffsetCursor(4)),
		// 		First:  lo.ToPtr(8),
		// 	},
		// 	expectedEdgesLen: 3,
		// 	expectedFirstKey: "id1",
		// 	expectedLastKey:  "id3",
		// 	expectedPageInfo: &pagination.PageInfo{
		// 		TotalCount:      100,
		// 		HasNextPage:     true,
		// 		HasPreviousPage: true,
		// 		StartCursor:     lo.ToPtr(toOffsetCursor(1)),
		// 		EndCursor:       lo.ToPtr(toOffsetCursor(3)),
		// 	},
		// },
		// {
		// 	name:             "After cursor 99",
		// 	limitIfNotSet:    10,
		// 	maxLimit:         20,
		// 	applyCursorsFunc: applyCursorsFunc,
		// 	paginateRequest: &pagination.PaginateRequest[*User]{
		// 		After: lo.ToPtr(toOffsetCursor(99)),
		// 	},
		// 	expectedEdgesLen: 0,
		// 	expectedPageInfo: &pagination.PageInfo{
		// 		TotalCount:      100,
		// 		HasNextPage:     false,
		// 		HasPreviousPage: true,
		// 		StartCursor:     nil,
		// 		EndCursor:       nil,
		// 	},
		// },
		// {
		// 	name:             "Before cursor 0",
		// 	limitIfNotSet:    10,
		// 	maxLimit:         20,
		// 	applyCursorsFunc: applyCursorsFunc,
		// 	paginateRequest: &pagination.PaginateRequest[*User]{
		// 		Before: lo.ToPtr(toOffsetCursor(0)),
		// 	},
		// 	expectedEdgesLen: 0,
		// 	expectedPageInfo: &pagination.PageInfo{
		// 		TotalCount:      100,
		// 		HasNextPage:     true,
		// 		HasPreviousPage: false,
		// 		StartCursor:     nil,
		// 		EndCursor:       nil,
		// 	},
		// },
		// {
		// 	name:             "First 200",
		// 	limitIfNotSet:    10,
		// 	maxLimit:         300,
		// 	applyCursorsFunc: applyCursorsFunc,
		// 	paginateRequest: &pagination.PaginateRequest[*User]{
		// 		First: lo.ToPtr(200),
		// 	},
		// 	expectedEdgesLen: 100,
		// 	expectedFirstKey: "id0",
		// 	expectedLastKey:  "id99",
		// 	expectedPageInfo: &pagination.PageInfo{
		// 		TotalCount:      100,
		// 		HasNextPage:     false,
		// 		HasPreviousPage: false,
		// 		StartCursor:     lo.ToPtr(toOffsetCursor(0)),
		// 		EndCursor:       lo.ToPtr(toOffsetCursor(99)),
		// 	},
		// },
		// {
		// 	name:             "First 0",
		// 	limitIfNotSet:    10,
		// 	maxLimit:         20,
		// 	applyCursorsFunc: applyCursorsFunc,
		// 	paginateRequest: &pagination.PaginateRequest[*User]{
		// 		First: lo.ToPtr(0),
		// 	},
		// 	expectedEdgesLen: 0,
		// 	expectedPageInfo: &pagination.PageInfo{
		// 		TotalCount:      100,
		// 		HasNextPage:     true,
		// 		HasPreviousPage: false,
		// 		StartCursor:     nil,
		// 		EndCursor:       nil,
		// 	},
		// },
		// {
		// 	name:             "Last 0",
		// 	limitIfNotSet:    10,
		// 	maxLimit:         20,
		// 	applyCursorsFunc: applyCursorsFunc,
		// 	paginateRequest: &pagination.PaginateRequest[*User]{
		// 		Last: lo.ToPtr(0),
		// 	},
		// 	expectedEdgesLen: 0,
		// 	expectedPageInfo: &pagination.PageInfo{
		// 		TotalCount:      100,
		// 		HasNextPage:     false,
		// 		HasPreviousPage: true,
		// 		StartCursor:     nil,
		// 		EndCursor:       nil,
		// 	},
		// },
		// {
		// 	name:             "After cursor 95, First 10",
		// 	limitIfNotSet:    10,
		// 	maxLimit:         20,
		// 	applyCursorsFunc: applyCursorsFunc,
		// 	paginateRequest: &pagination.PaginateRequest[*User]{
		// 		After: lo.ToPtr(toOffsetCursor(95)),
		// 		First: lo.ToPtr(10),
		// 	},
		// 	expectedEdgesLen: 4,
		// 	expectedFirstKey: "id96",
		// 	expectedLastKey:  "id99",
		// 	expectedPageInfo: &pagination.PageInfo{
		// 		TotalCount:      100,
		// 		HasNextPage:     false,
		// 		HasPreviousPage: true,
		// 		StartCursor:     lo.ToPtr(toOffsetCursor(96)),
		// 		EndCursor:       lo.ToPtr(toOffsetCursor(99)),
		// 	},
		// },
		// {
		// 	name:             "Before cursor 4, Last 10",
		// 	limitIfNotSet:    10,
		// 	maxLimit:         20,
		// 	applyCursorsFunc: applyCursorsFunc,
		// 	paginateRequest: &pagination.PaginateRequest[*User]{
		// 		Before: lo.ToPtr(toOffsetCursor(4)),
		// 		Last:   lo.ToPtr(10),
		// 	},
		// 	expectedEdgesLen: 4,
		// 	expectedFirstKey: "id0",
		// 	expectedLastKey:  "id3",
		// 	expectedPageInfo: &pagination.PageInfo{
		// 		TotalCount:      100,
		// 		HasNextPage:     true,
		// 		HasPreviousPage: false,
		// 		StartCursor:     lo.ToPtr(toOffsetCursor(0)),
		// 		EndCursor:       lo.ToPtr(toOffsetCursor(3)),
		// 	},
		// },
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
