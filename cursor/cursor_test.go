package cursor_test

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

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
			ID:        fmt.Sprintf("id%d", i),
			Name:      fmt.Sprintf("name%d", i),
			Age:       100 - i,
			CreatedAt: time.Now(),
		})
	}
	err := db.Session(&gorm.Session{Logger: logger.Discard}).Create(vs).Error
	require.NoError(t, err)
}

type User struct {
	ID        string    `json:"id" gorm:"primaryKey;not null;"`
	Name      string    `json:"name" gorm:"not null;"`
	Age       int       `json:"age" gorm:"index;not null;"`
	CreatedAt time.Time `json:"createdAt" gorm:"index;not null;"`
}

func MustJsonString(v any) string {
	s, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(s)
}
