package integrations

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"github.com/stretchr/testify/require"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	_ "github.com/lib/pq"

	"github.com/pressly/goose/v3"

	"github.com/stretchr/testify/assert"

	"gitlab.ozon.dev/qwestard/homework/internal/config"
	"gitlab.ozon.dev/qwestard/homework/internal/models"
	"gitlab.ozon.dev/qwestard/homework/internal/repository"
	"gitlab.ozon.dev/qwestard/homework/internal/server"
)

var (
	db           *sql.DB
	testServer   *httptest.Server
	testUsername string
	testPassword string
)

func TestMain(m *testing.M) {
	cfg := config.LoadConfig()

	testUsername = cfg.Username
	testPassword = cfg.Password

	var err error
	db, err = sql.Open("postgres", cfg.DSN)
	if err != nil {
		log.Fatalf("sql.Open error: %v", err)
	}
	if err = db.Ping(); err != nil {
		log.Fatalf("db.Ping error: %v", err)
	}

	if err := goose.Up(db, "../migrations"); err != nil {
		log.Fatalf("goose.Up error: %v", err)
	}

	repo := repository.NewOrderRepository(db)
	srv := server.NewServer(repo, cfg)

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	testServer = httptest.NewServer(mux)

	if _, err := db.Exec("TRUNCATE orders CASCADE"); err != nil {
		log.Printf("truncate error: %v", err)
	}

	m.Run()

	testServer.Close()
	_ = db.Close()
}

func doRequest(t *testing.T, method, path string, body any) (*http.Response, []byte) {
	t.Helper()

	var reqBody []byte
	var err error
	if body != nil {
		reqBody, err = json.Marshal(body)
		if err != nil {
			t.Fatalf("json.Marshal error: %v", err)
		}
	}

	req, err := http.NewRequest(method, testServer.URL+path, bytes.NewReader(reqBody))
	if err != nil {
		t.Fatalf("http.NewRequest: %v", err)
	}

	req.SetBasicAuth(testUsername, testPassword)

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("client.Do: %v", err)
	}
	respBody, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	return resp, respBody
}

func TestCreateOrderIntegration(t *testing.T) {
	order := models.Order{
		ID:              "test-integration-create-1",
		RecipientID:     "user001",
		StorageDeadline: time.Now().Add(2 * time.Hour),
		Weight:          5,
		Cost:            99,
	}

	resp, body := doRequest(t, http.MethodPost, "/orders", order)

	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var got models.Order
	err := json.Unmarshal(body, &got)
	require.NoError(t, err)
	assert.Equal(t, order.ID, got.ID)
}

func TestCreateOrderWrongDeadline(t *testing.T) {
	order := models.Order{
		ID:              "test-integration-wrong-deadline",
		RecipientID:     "user002",
		StorageDeadline: time.Now().Add(-time.Hour),
	}

	resp, _ := doRequest(t, http.MethodPost, "/orders", order)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestListOrdersIntegration(t *testing.T) {
	for i := 1; i <= 2; i++ {
		order := models.Order{
			ID:              "test-list-" + strconv.Itoa(i),
			RecipientID:     "userL" + strconv.Itoa(i),
			StorageDeadline: time.Now().Add(2 * time.Hour),
		}
		doRequest(t, http.MethodPost, "/orders", order)
	}

	resp, body := doRequest(t, http.MethodGet, "/orders?cursor=&limit=10", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var orders []models.Order
	err := json.Unmarshal(body, &orders)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(orders), 2)
}

func TestUpdateOrderIntegration(t *testing.T) {
	order := models.Order{
		ID:              "test-update-1",
		RecipientID:     "userUp",
		StorageDeadline: time.Now().Add(2 * time.Hour),
	}
	doRequest(t, http.MethodPost, "/orders", order)

	order.Weight = 123
	resp, body := doRequest(t, http.MethodPut, "/orders/test-update-1", order)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var updated models.Order
	err := json.Unmarshal(body, &updated)
	require.NoError(t, err)
	assert.Equal(t, float64(123), updated.Weight)
}

func TestDeleteOrderIntegration(t *testing.T) {
	order := models.Order{
		ID:              "test-delete-1",
		RecipientID:     "userDel",
		StorageDeadline: time.Now().Add(2 * time.Hour),
	}
	doRequest(t, http.MethodPost, "/orders", order)

	resp, _ := doRequest(t, http.MethodDelete, "/orders/test-delete-1", nil)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)

	resp2, _ := doRequest(t, http.MethodGet, "/orders/test-delete-1", nil)
	assert.Equal(t, http.StatusNotFound, resp2.StatusCode)
}
