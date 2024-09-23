package cursor

import (
	"context"
	"encoding/base64"
	"strconv"

	"github.com/molon/gorelay/pagination"
	"github.com/pkg/errors"
)

type OffsetFinder[T any] interface {
	Find(ctx context.Context, skip, limit int) ([]T, error)
}

type OffsetFinderFunc[T any] func(ctx context.Context, skip, limit int) ([]T, error)

func (f OffsetFinderFunc[T]) Find(ctx context.Context, skip, limit int) ([]T, error) {
	return f(ctx, skip, limit)
}

type OffsetParser interface {
	Encode(ctx context.Context, offset int) (string, error)
	Decode(ctx context.Context, cursor string) (int, error)
}

func NewOffsetAdapter[T any](finder OffsetFinder[T]) pagination.ApplyCursorsFunc[T] {
	return func(ctx context.Context, req *pagination.ApplyCursorsRequest) (*pagination.ApplyCursorsResponse[T], error) {
		parser, ok := finder.(OffsetParser)
		if !ok {
			parser = defaultOffsetParser
		}
		after, before, err := parseOffsetCursors[T](ctx, req, parser)
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

		limit, skip := req.Limit, 0
		if after != nil {
			skip = *after + 1
		} else if before != nil {
			skip = *before - limit
		}
		if skip < 0 {
			skip = 0
		}
		if before != nil {
			rangeLen := *before - skip
			if rangeLen < 0 {
				rangeLen = 0
			}
			if limit > rangeLen {
				limit = rangeLen
			}
		}

		var edges []pagination.Edge[T]
		if limit <= 0 || (counter != nil && skip >= totalCount) {
			edges = make([]pagination.Edge[T], 0)
		} else {
			nodes, err := finder.Find(ctx, skip, limit)
			if err != nil {
				return nil, err
			}
			edges = make([]pagination.Edge[T], len(nodes))
			for i, node := range nodes {
				cursor, err := parser.Encode(ctx, skip+i)
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
		}

		if counter != nil {
			resp.HasAfterOrPrevious = after != nil && *after < totalCount
			resp.HasBeforeOrNext = before != nil && *before < totalCount
		} else {
			// If we don't have a counter, it would be very costly to check whether after and before really exist,
			// So it is usually not worth it. Normally, checking that it is not nil is sufficient.
			resp.HasAfterOrPrevious = after != nil
			resp.HasBeforeOrNext = before != nil
		}

		return resp, nil
	}
}

type offsetParserImpl struct{}

var defaultOffsetParser = &offsetParserImpl{}

// TODO: 感觉应该这里返回 plain text ，然后 pagination 那边需要加密再加密？ 因为偶尔是返回 nodes 就好，并非所有都需要 cursor
func (p *offsetParserImpl) Decode(_ context.Context, cursor string) (int, error) {
	decoded, err := base64.StdEncoding.DecodeString(cursor)
	if err != nil {
		// TODO:
		return 0, errors.Wrapf(err, "decode offset cursor %q", cursor)
	}
	offset, err := strconv.Atoi(string(decoded))
	if err != nil {
		return 0, errors.Wrapf(err, "parse offset cursor %q", cursor)
	}
	return offset, nil
}

func (p *offsetParserImpl) Encode(_ context.Context, offset int) (string, error) {
	return base64.StdEncoding.EncodeToString([]byte(strconv.Itoa(offset))), nil
}

func parseOffsetCursors[T any](ctx context.Context, req *pagination.ApplyCursorsRequest, parser OffsetParser) (after, before *int, err error) {
	if req.After != nil {
		offset, err := parser.Decode(ctx, *req.After)
		if err != nil {
			return nil, nil, err
		}
		after = &offset
	}
	if req.Before != nil {
		offset, err := parser.Decode(ctx, *req.Before)
		if err != nil {
			return nil, nil, err
		}
		before = &offset
	}
	if after != nil && *after < 0 {
		return nil, nil, errors.New("after < 0")
	}
	if before != nil && *before < 0 {
		return nil, nil, errors.New("before < 0")
	}
	if after != nil && before != nil && *after >= *before {
		return nil, nil, errors.New("after >= before")
	}
	return after, before, nil
}
