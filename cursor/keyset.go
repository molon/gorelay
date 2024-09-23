package cursor

import (
	"context"
	"encoding/json"

	"github.com/molon/gorelay/pagination"
	"github.com/pkg/errors"
	"github.com/samber/lo"
)

type KeysetFinder[T any] interface {
	Find(ctx context.Context, after, before *map[string]any, orderBys []pagination.OrderBy, limit int, fromLast bool) ([]T, error)
}

type KeysetFinderFunc[T any] func(ctx context.Context, after, before *map[string]any, orderBys []pagination.OrderBy, limit int, fromLast bool) ([]T, error)

func (f KeysetFinderFunc[T]) Find(ctx context.Context, after, before *map[string]any, orderBys []pagination.OrderBy, limit int, fromLast bool) ([]T, error) {
	return f(ctx, after, before, orderBys, limit, fromLast)
}

func NewKeysetAdapter[T any](finder KeysetFinder[T]) pagination.ApplyCursorsFunc[T] {
	return func(ctx context.Context, req *pagination.ApplyCursorsRequest) (*pagination.ApplyCursorsResponse[T], error) {
		keys := lo.Map(req.OrderBys, func(item pagination.OrderBy, _ int) string {
			return item.Field
		})

		after, before, err := decodeKeysetCursors[T](req.After, req.Before, keys)
		if err != nil {
			return nil, err
		}

		var totalCount int
		counter, ok := finder.(Counter[T])
		if ok {
			var err error
			totalCount, err = counter.Count(ctx)
			if err != nil {
				return nil, err
			}
		}

		var edges []pagination.Edge[T]
		if req.Limit <= 0 || (counter != nil && totalCount <= 0) {
			edges = make([]pagination.Edge[T], 0)
		} else {
			nodes, err := finder.Find(ctx, after, before, req.OrderBys, req.Limit, req.FromLast)
			if err != nil {
				return nil, err
			}
			edges = make([]pagination.Edge[T], len(nodes))
			for i, node := range nodes {
				cursor, err := EncodeKeysetCursor(node, keys)
				if err != nil {
					return nil, err
				}
				edges[i] = pagination.Edge[T]{
					Node:   node,
					Cursor: cursor,
				}
			}
		}

		resp := &pagination.ApplyCursorsResponse[T]{
			Edges:      edges,
			TotalCount: totalCount,
			// If we don't have a counter, it would be very costly to check whether after and before really exist,
			// So it is usually not worth it. Normally, checking that it is not nil is sufficient.
			HasAfterOrPrevious: after != nil,
			HasBeforeOrNext:    before != nil,
		}
		return resp, nil
	}
}

func EncodeKeysetCursor[T any](node T, keys []string) (string, error) {
	b, err := json.Marshal(node)
	if err != nil {
		return "", errors.Wrap(err, "marshal cursor")
	}
	m := make(map[string]any)
	if err := json.Unmarshal(b, &m); err != nil {
		return "", errors.Wrap(err, "unmarshal cursor")
	}
	b, err = json.Marshal(lo.PickByKeys(m, keys))
	if err != nil {
		return "", errors.Wrap(err, "marshal filtered cursor")
	}
	return string(b), nil
}

func DecodeKeysetCursor[T any](cursor string, keys []string) (map[string]any, error) {
	var m map[string]any
	if err := json.Unmarshal([]byte(cursor), &m); err != nil {
		return nil, errors.Wrap(err, "unmarshal cursor")
	}
	if len(m) != len(keys) {
		return nil, errors.New("cursor length != keys length")
	}
	// TODO: omitempty not supported now
	for _, key := range keys {
		if _, ok := m[key]; !ok {
			return nil, errors.Errorf("key %q not found in cursor", key)
		}
	}
	return m, nil
}

func decodeKeysetCursors[T any](after, before *string, keys []string) (afterKeyset, beforeKeyset *map[string]any, err error) {
	// TODO: should more strict ?
	if after != nil && before != nil && *after == *before {
		return nil, nil, errors.New("after == before")
	}
	if after != nil {
		m, err := DecodeKeysetCursor[T](*after, keys)
		if err != nil {
			return nil, nil, err
		}
		afterKeyset = &m
	}
	if before != nil {
		m, err := DecodeKeysetCursor[T](*before, keys)
		if err != nil {
			return nil, nil, err
		}
		beforeKeyset = &m
	}
	return afterKeyset, beforeKeyset, nil
}
