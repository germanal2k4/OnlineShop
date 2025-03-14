package repository_test

import (
	"database/sql"
	"gitlab.ozon.dev/qwestard/homework/internal/models"
	"log"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"

	"gitlab.ozon.dev/qwestard/homework/internal/repository"
)

var db *sql.DB
var repo *repository.OrderRepository

func TestMain(m *testing.M) {
	dsn := os.Getenv("TEST_DSN")
	if dsn == "" {
		dsn = "host=localhost user=postgres password=postgres dbname=orders_test sslmode=disable"
	}
	var err error
	db, err = sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	if err = db.Ping(); err != nil {
		log.Fatalf("ping db: %v", err)
	}
	repo = repository.NewOrderRepository(db)

	code := m.Run()

	db.Exec("DELETE FROM order_packaging")
	db.Exec("DELETE FROM orders")

	os.Exit(code)
}

func TestCreateGetDelete(t *testing.T) {
	o := &models.Order{
		ID:              "test-100",
		RecipientID:     "user42",
		StorageDeadline: time.Now().Add(24 * time.Hour),
		LastStateChange: time.Now().UTC(),
		Packaging:       []string{"box", "film"},
	}
	err := repo.Create(o)
	assert.NoError(t, err)

	o2, err := repo.GetByID("test-100")
	assert.NoError(t, err)
	assert.NotNil(t, o2)
	assert.Equal(t, "user42", o2.RecipientID)
	assert.ElementsMatch(t, []string{"box", "film"}, o2.Packaging)

	err = repo.Delete("test-100")
	assert.NoError(t, err)

	o3, err := repo.GetByID("test-100")
	assert.NoError(t, err)
	assert.Nil(t, o3)
}

func TestDeliverAndReturn(t *testing.T) {
	o := &models.Order{
		ID:              "test-deliver-1",
		RecipientID:     "userA",
		LastStateChange: time.Now().UTC(),
	}
	err := repo.Create(o)
	assert.NoError(t, err)

	err = repo.Deliver(o.ID)
	assert.NoError(t, err)

	o2, err := repo.GetByID(o.ID)
	assert.NoError(t, err)
	assert.Equal(t, models.OrderStateDelivered, o2.CurrentState())

	err = repo.ClientReturn(o.ID)
	assert.NoError(t, err)

	o3, _ := repo.GetByID(o.ID)
	assert.Equal(t, models.OrderStateClientRtn, o3.CurrentState())
}

func TestGetReturns(t *testing.T) {
	o1 := &models.Order{ID: "rtn-1", LastStateChange: time.Now().UTC()}
	o2 := &models.Order{ID: "rtn-2", LastStateChange: time.Now().UTC()}
	_ = repo.Create(o1)
	_ = repo.Create(o2)
	_ = repo.ClientReturn("rtn-2")

	list, err := repo.GetReturns(0, 10, "")
	assert.NoError(t, err)
	assert.Len(t, list, 1)
	assert.Equal(t, "rtn-2", list[0].ID)
}
