package cursor

import (
	"testing"

	jsoniter "github.com/json-iterator/go"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestJsoniterWithStructFieldNameAndEmitEmpty(t *testing.T) {
	type User struct {
		gorm.Model
		Name        string `json:"name,omitempty"`
		Description string `json:"description,omitempty"`
		age         int
	}
	jsonCustomize := jsoniter.Config{
		EscapeHTML:             true,
		SortMapKeys:            true,
		ValidateJsonRawMessage: true,
		TagKey:                 "__dummy__",
	}.Froze()
	str, err := jsonCustomize.MarshalToString(User{
		Model: gorm.Model{ID: 1},
		Name:  "molon",
	})
	require.NoError(t, err)
	require.Equal(t, `{"ID":1,"CreatedAt":"0001-01-01T00:00:00Z","UpdatedAt":"0001-01-01T00:00:00Z","DeletedAt":null,"Name":"molon","Description":""}`, str)
}

func TestEncodeKeysetCursor(t *testing.T) {
	type User struct {
		gorm.Model
		Name          string `json:"name,omitempty"`
		Description   string `json:"description,omitempty"`
		IgnoredByJson string `json:"-"`
		age           int
	}
	user := User{
		Model: gorm.Model{ID: 1},
		Name:  "molon",
	}
	{
		cursor, err := EncodeKeysetCursor(user, []string{"ID"})
		require.NoError(t, err)
		require.Equal(t, `{"ID":1}`, cursor)
	}
	{
		cursor, err := EncodeKeysetCursor(user, []string{"ID", "Description"})
		require.NoError(t, err)
		require.Equal(t, `{"Description":"","ID":1}`, cursor)
	}
	{
		cursor, err := EncodeKeysetCursor(user, []string{"ID", "Name", "Description"})
		require.NoError(t, err)
		require.Equal(t, `{"Description":"","ID":1,"Name":"molon"}`, cursor)
	}
	{
		cursor, err := EncodeKeysetCursor(user, []string{"Name", "Description", "IgnoredByJson"})
		require.NoError(t, err)
		require.Equal(t, `{"Description":"","IgnoredByJson":"","Name":"molon"}`, cursor)
	}
}
