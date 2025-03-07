package repository_test

import (
	_ "context"
	"database/sql"
	"log"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"

	"gitlab.ozon.dev/qwestard/homework/internal/models"
	"gitlab.ozon.dev/qwestard/homework/internal/repository"
)

var testDB *sql.DB
var repo *repository.OrderRepository

func TestMain(m *testing.M) {
	dsn := os.Getenv("TEST_DSN")
	if dsn == "" {
		dsn = "host=localhost user=postgres password=postgres dbname=orders_test sslmode=disable"
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("Ошибка открытия DB: %v", err)
	}
	if err := db.Ping(); err != nil {
		log.Fatalf("Нет соединения с DB: %v", err)
	}

	testDB = db
	repo = repository.NewOrderRepository(testDB)

	code := m.Run()

	_ = testDB.Close()

	os.Exit(code)
}

func TestCreateAndGetByID(t *testing.T) {
	order := &models.Order{
		ID:              "test-order-123",
		RecipientID:     "user42",
		StorageDeadline: time.Now().Add(24 * time.Hour),
		LastStateChange: time.Now().UTC(),
		Weight:          2.5,
		Cost:            100.0,
		Packaging:       []string{"box"},
	}
	// 1. Create
	err := repo.Create(order)
	assert.NoError(t, err, "Создание заказа не должно падать")

	// 2. GetByID
	o2, err := repo.GetByID("test-order-123")
	assert.NoError(t, err)
	assert.NotNil(t, o2)
	assert.Equal(t, order.ID, o2.ID)
	assert.Equal(t, "user42", o2.RecipientID)
}

func TestUpdate(t *testing.T) {
	// Создаём новый заказ
	o := &models.Order{
		ID:              "test-order-999",
		RecipientID:     "initialUser",
		StorageDeadline: time.Now().Add(24 * time.Hour),
		LastStateChange: time.Now().UTC(),
	}
	err := repo.Create(o)
	assert.NoError(t, err)

	// Меняем поля
	o.RecipientID = "updatedUser"
	o.Weight = 5.0
	err = repo.Update(o)
	assert.NoError(t, err)

	// Считываем снова
	o2, err := repo.GetByID("test-order-999")
	assert.NoError(t, err)
	assert.NotNil(t, o2)
	assert.Equal(t, "updatedUser", o2.RecipientID)
	assert.Equal(t, 5.0, o2.Weight)
}

func TestDelete(t *testing.T) {
	o := &models.Order{
		ID:          "test-delete-1",
		RecipientID: "deleteUser",
	}
	err := repo.Create(o)
	assert.NoError(t, err)

	// Удаляем
	err = repo.Delete("test-delete-1")
	assert.NoError(t, err)

	// Проверяем, что теперь нет
	o2, err := repo.GetByID("test-delete-1")
	assert.NoError(t, err)
	assert.Nil(t, o2)
}

func TestList(t *testing.T) {

	o1 := &models.Order{ID: "list-001", RecipientID: "A", LastStateChange: time.Now().UTC()}
	o2 := &models.Order{ID: "list-002", RecipientID: "B", LastStateChange: time.Now().UTC()}
	err := repo.Create(o1)
	assert.NoError(t, err)
	err = repo.Create(o2)
	assert.NoError(t, err)

	ords, err := repo.List("", 10)
	assert.NoError(t, err)
	assert.True(t, len(ords) >= 2)

	ords2, err := repo.List("list-001", 10)
	assert.NoError(t, err)
	var found001 bool
	var found002 bool
	for _, od := range ords2 {
		if od.ID == "list-001" {
			found001 = true
		}
		if od.ID == "list-002" {
			found002 = true
		}
	}
	assert.False(t, found001)
	assert.True(t, found002)
}
