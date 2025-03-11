package integrations

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"gitlab.ozon.dev/qwestard/homework/internal/models"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/assert"

	"gitlab.ozon.dev/qwestard/homework/internal/config"
	"gitlab.ozon.dev/qwestard/homework/internal/repository"
	"gitlab.ozon.dev/qwestard/homework/internal/server"
)

var (
	db         *sql.DB
	testServer *httptest.Server
)

func TestMain(m *testing.M) {
	cfg := config.LoadConfig()

	var err error
	db, err = sql.Open("postgres", cfg.DSN)
	if err != nil {
		log.Fatalf("open db error: %v", err)
	}
	if err = db.Ping(); err != nil {
		log.Fatalf("ping db error: %v", err)
	}

	if err := goose.Up(db, "../migrations"); err != nil {
		log.Fatalf("goose up error: %v", err)
	}

	repo := repository.NewOrderRepository(db)
	srv := server.NewServer(repo, cfg)

	if err := srv.Run(); err != nil {
		log.Fatalf("Server stopped: %v", err)
	}

	_, _ = db.Exec("TRUNCATE orders CASCADE")

	testServer.Close()
	_ = db.Close()
}

func TestAcceptOrder(t *testing.T) {
	order := models.Order{
		ID:              "int-accept-1",
		RecipientID:     "userA",
		StorageDeadline: time.Now().Add(2 * time.Hour),
		Weight:          5,
		Cost:            100,
	}
	resp, body := doRequest(t, http.MethodPost, "/orders", order)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	assert.Contains(t, string(body), "int-accept-1")
}

func TestAcceptOrder_Duplicate(t *testing.T) {
	order := models.Order{
		ID:              "int-dup-1",
		RecipientID:     "userD",
		StorageDeadline: time.Now().Add(3 * time.Hour),
	}
	doRequest(t, http.MethodPost, "/orders", order)
	resp2, body2 := doRequest(t, http.MethodPost, "/orders", order)
	assert.True(t, resp2.StatusCode == http.StatusConflict || resp2.StatusCode == http.StatusBadRequest)
	t.Logf("Body: %s", string(body2))
}

func TestAcceptOrder_DeadlinePast(t *testing.T) {
	order := models.Order{
		ID:              "int-past-1",
		RecipientID:     "userP",
		StorageDeadline: time.Now().Add(-1 * time.Hour),
	}
	resp, _ := doRequest(t, http.MethodPost, "/orders", order)
	assert.True(t, resp.StatusCode == http.StatusConflict || resp.StatusCode == http.StatusBadRequest)
}

func TestDeliverOrder(t *testing.T) {
	order := models.Order{
		ID:              "int-deliv-1",
		RecipientID:     "userX",
		StorageDeadline: time.Now().Add(2 * time.Hour),
	}
	doRequest(t, http.MethodPost, "/orders", order)
	resp, _ := doRequest(t, http.MethodPut, "/orders-deliver/int-deliv-1", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestClientReturn(t *testing.T) {
	order := models.Order{
		ID:              "int-return-1",
		RecipientID:     "userR",
		StorageDeadline: time.Now().Add(2 * time.Hour),
	}
	doRequest(t, http.MethodPost, "/orders", order)
	doRequest(t, http.MethodPut, "/orders-deliver/int-return-1", nil)
	resp, _ := doRequest(t, http.MethodPut, "/orders-return/int-return-1", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestGetReturns(t *testing.T) {
	order := models.Order{
		ID:              "int-returns-1",
		RecipientID:     "userZ",
		StorageDeadline: time.Now().Add(2 * time.Hour),
	}
	doRequest(t, http.MethodPost, "/orders", order)
	doRequest(t, http.MethodPut, "/orders-deliver/int-returns-1", nil)
	doRequest(t, http.MethodPut, "/orders-return/int-returns-1", nil)

	resp, body := doRequest(t, http.MethodGet, "/returns?offset=0&limit=5", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var list []models.Order
	if err := json.Unmarshal(body, &list); err != nil {
		t.Fatalf("json decode error: %v", err)
	}
	assert.Len(t, list, 1)
	assert.Equal(t, "int-returns-1", list[0].ID)
}

func doRequest(t *testing.T, method, path string, body interface{}) (*http.Response, []byte) {
	var reqBody []byte
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("json marshal: %v", err)
		}
		reqBody = b
	}
	req, err := http.NewRequest(method, testServer.URL+path, bytes.NewReader(reqBody))
	if err != nil {
		t.Fatalf("http.NewRequest: %v", err)
	}
	if reqBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("client.Do: %v", err)
	}
	bodyBytes, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	return resp, bodyBytes
}
