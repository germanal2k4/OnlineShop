package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gitlab.ozon.dev/qwestard/homework/internal/config"
	"gitlab.ozon.dev/qwestard/homework/internal/models"
	"gitlab.ozon.dev/qwestard/homework/internal/repository"
)

type fakeRepo struct {
	orders          map[string]models.Order
	createErr       error
	listErr         error
	getErr          error
	updateErr       error
	deleteErr       error
	deliverErr      error
	clientReturnErr error
	getReturnsErr   error
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{orders: make(map[string]models.Order)}
}

func (r *fakeRepo) Create(o *models.Order) error {
	if r.createErr != nil {
		return r.createErr
	}
	if _, exists := r.orders[o.ID]; exists {
		return errors.New("order already exists")
	}
	r.orders[o.ID] = *o
	return nil
}

func (r *fakeRepo) List(_ string, _ int64, _ string) ([]*models.Order, error) {
	if r.listErr != nil {
		return nil, r.listErr
	}
	var result []*models.Order
	for _, o := range r.orders {
		result = append(result, &o)
	}
	return result, nil
}

func (r *fakeRepo) GetByID(id string) (*models.Order, error) {
	if r.getErr != nil {
		return nil, r.getErr
	}
	o, exists := r.orders[id]
	if !exists {
		return nil, nil
	}
	copyOrder := o
	return &copyOrder, nil
}

func (r *fakeRepo) Update(o *models.Order) error {
	if r.updateErr != nil {
		return r.updateErr
	}
	if _, exists := r.orders[o.ID]; !exists {
		return errors.New("order not found")
	}
	r.orders[o.ID] = *o
	return nil
}

func (r *fakeRepo) Delete(id string) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	if _, exists := r.orders[id]; !exists {
		return errors.New("order not found")
	}
	delete(r.orders, id)
	return nil
}

func (r *fakeRepo) Deliver(id string) error {
	if r.deliverErr != nil {
		return r.deliverErr
	}
	o, exists := r.orders[id]
	if !exists {
		return errors.New("order not found")
	}
	o.DeliveredAt = time.Now()
	r.orders[id] = o
	return nil
}

func (r *fakeRepo) ClientReturn(id string) error {
	if r.clientReturnErr != nil {
		return r.clientReturnErr
	}
	o, exists := r.orders[id]
	if !exists {
		return errors.New("order not found")
	}
	o.ClientReturnAt = time.Now()
	r.orders[id] = o
	return nil
}

func (r *fakeRepo) GetReturns(_, _ int64, _ string) ([]*models.Order, error) {
	if r.getReturnsErr != nil {
		return nil, r.getReturnsErr
	}
	var result []*models.Order
	for _, o := range r.orders {
		if !o.ClientReturnAt.IsZero() {
			copyOrder := o
			result = append(result, &copyOrder)
		}
	}
	return result, nil
}

var _ repository.Repository = (*fakeRepo)(nil)

func newTestConfig() *config.Config {
	return &config.Config{
		HTTPPort: "8080",
		Username: "testuser",
		Password: "testpass",
	}
}

func createTestMux(s *Server) *http.ServeMux {
	mux := http.NewServeMux()
	s.handleWith(mux, "/orders", s.handleOrders, []string{"POST"}, []string{"POST"})
	s.handleWith(mux, "/orders/", s.handleOrderOne, []string{"POST", "PUT", "DELETE"}, []string{"POST", "PUT", "DELETE"})
	s.handleWith(mux, "/orders-deliver/", s.handleDeliver, []string{"PUT"}, []string{"PUT"})
	s.handleWith(mux, "/orders-return/", s.handleClientReturn, []string{"PUT"}, []string{"PUT"})
	mux.HandleFunc("/returns", s.handleGetReturns)
	return mux
}

func TestHandleCreateOrder(t *testing.T) {
	repo := newFakeRepo()
	cfg := newTestConfig()
	s := NewServer(repo, cfg)
	mux := createTestMux(s)

	t.Run("valid order", func(t *testing.T) {
		order := models.Order{
			ID:              "order-1",
			StorageDeadline: time.Now().Add(time.Hour),
		}
		data, _ := json.Marshal(order)
		req := httptest.NewRequest("POST", "/orders", bytes.NewReader(data))
		req.SetBasicAuth(cfg.Username, cfg.Password)
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Errorf("expected %d, got %d", http.StatusCreated, rec.Code)
		}
		var got models.Order
		_ = json.NewDecoder(rec.Body).Decode(&got)
		if got.ID != "order-1" {
			t.Errorf("expected ID 'order-1', got '%s'", got.ID)
		}
	})

	t.Run("bad JSON", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/orders", strings.NewReader("badjson"))
		req.SetBasicAuth(cfg.Username, cfg.Password)
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected %d, got %d", http.StatusBadRequest, rec.Code)
		}
	})

	t.Run("wrong deadline", func(t *testing.T) {
		order := models.Order{
			ID:              "order-2",
			StorageDeadline: time.Now().Add(-time.Hour),
		}
		data, _ := json.Marshal(order)
		req := httptest.NewRequest("POST", "/orders", bytes.NewReader(data))
		req.SetBasicAuth(cfg.Username, cfg.Password)
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected %d, got %d", http.StatusBadRequest, rec.Code)
		}
	})

	t.Run("repo conflict", func(t *testing.T) {
		repo.createErr = errors.New("some conflict")
		order := models.Order{
			ID:              "order-3",
			StorageDeadline: time.Now().Add(time.Hour),
		}
		data, _ := json.Marshal(order)
		req := httptest.NewRequest("POST", "/orders", bytes.NewReader(data))
		req.SetBasicAuth(cfg.Username, cfg.Password)
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusConflict {
			t.Errorf("expected %d, got %d", http.StatusConflict, rec.Code)
		}
		repo.createErr = nil
	})
}

func TestHandleListOrders(t *testing.T) {
	repo := newFakeRepo()
	cfg := newTestConfig()
	s := NewServer(repo, cfg)
	mux := createTestMux(s)

	repo.orders["1"] = models.Order{ID: "1", StorageDeadline: time.Now().Add(time.Hour)}
	repo.orders["2"] = models.Order{ID: "2", StorageDeadline: time.Now().Add(time.Hour)}

	req := httptest.NewRequest("GET", "/orders?cursor=&limit=10", nil)
	req.SetBasicAuth(cfg.Username, cfg.Password)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected %d, got %d", http.StatusOK, rec.Code)
	}
	var orders []*models.Order
	_ = json.NewDecoder(rec.Body).Decode(&orders)
	if len(orders) != 2 {
		t.Errorf("expected 2 orders, got %d", len(orders))
	}
}

func TestHandleGetOrder(t *testing.T) {
	repo := newFakeRepo()
	cfg := newTestConfig()
	s := NewServer(repo, cfg)
	mux := createTestMux(s)

	repo.orders["abc"] = models.Order{ID: "abc", StorageDeadline: time.Now().Add(time.Hour)}

	t.Run("order exists", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/orders/abc", nil)
		req.SetBasicAuth(cfg.Username, cfg.Password)
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("expected %d, got %d", http.StatusOK, rec.Code)
		}
		var o models.Order
		_ = json.NewDecoder(rec.Body).Decode(&o)
		if o.ID != "abc" {
			t.Errorf("expected 'abc', got '%s'", o.ID)
		}
	})

	t.Run("order not found", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/orders/zzz", nil)
		req.SetBasicAuth(cfg.Username, cfg.Password)
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Errorf("expected %d, got %d", http.StatusNotFound, rec.Code)
		}
	})

	t.Run("repo error", func(t *testing.T) {
		repo.getErr = errors.New("some error")
		req := httptest.NewRequest("GET", "/orders/abc", nil)
		req.SetBasicAuth(cfg.Username, cfg.Password)
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusInternalServerError {
			t.Errorf("expected %d, got %d", http.StatusInternalServerError, rec.Code)
		}
		repo.getErr = nil
	})
}

func TestHandleUpdateOrder(t *testing.T) {
	repo := newFakeRepo()
	cfg := newTestConfig()
	s := NewServer(repo, cfg)
	mux := createTestMux(s)

	repo.orders["abc"] = models.Order{ID: "abc", StorageDeadline: time.Now().Add(time.Hour)}

	t.Run("successful update", func(t *testing.T) {
		updateOrder := models.Order{
			ID:              "abc",
			StorageDeadline: time.Now().Add(2 * time.Hour),
		}
		data, _ := json.Marshal(updateOrder)
		req := httptest.NewRequest("PUT", "/orders/abc", bytes.NewReader(data))
		req.SetBasicAuth(cfg.Username, cfg.Password)
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("expected %d, got %d", http.StatusOK, rec.Code)
		}
		var got models.Order
		_ = json.NewDecoder(rec.Body).Decode(&got)
		if got.ID != "abc" {
			t.Errorf("expected 'abc', got '%s'", got.ID)
		}
	})

	t.Run("ID mismatch", func(t *testing.T) {
		updateOrder := models.Order{
			ID:              "another",
			StorageDeadline: time.Now().Add(2 * time.Hour),
		}
		data, _ := json.Marshal(updateOrder)
		req := httptest.NewRequest("PUT", "/orders/abc", bytes.NewReader(data))
		req.SetBasicAuth(cfg.Username, cfg.Password)
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected %d, got %d", http.StatusBadRequest, rec.Code)
		}
	})

	t.Run("bad JSON", func(t *testing.T) {
		req := httptest.NewRequest("PUT", "/orders/abc", strings.NewReader("{not valid json"))
		req.SetBasicAuth(cfg.Username, cfg.Password)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected %d, got %d", http.StatusBadRequest, rec.Code)
		}
	})

	t.Run("repo error", func(t *testing.T) {
		repo.updateErr = errors.New("some update error")
		updateOrder := models.Order{ID: "abc"}
		data, _ := json.Marshal(updateOrder)
		req := httptest.NewRequest("PUT", "/orders/abc", bytes.NewReader(data))
		req.SetBasicAuth(cfg.Username, cfg.Password)
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusInternalServerError {
			t.Errorf("expected %d, got %d", http.StatusInternalServerError, rec.Code)
		}
		repo.updateErr = nil
	})
}

func TestHandleDeleteOrder(t *testing.T) {
	repo := newFakeRepo()
	cfg := newTestConfig()
	s := NewServer(repo, cfg)
	mux := createTestMux(s)

	repo.orders["abc"] = models.Order{ID: "abc"}

	t.Run("successful delete", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/orders/abc", nil)
		req.SetBasicAuth(cfg.Username, cfg.Password)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent {
			t.Errorf("expected %d, got %d", http.StatusNoContent, rec.Code)
		}
		if _, exists := repo.orders["abc"]; exists {
			t.Errorf("order 'abc' not deleted")
		}
	})

	t.Run("order not found", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/orders/notexist", nil)
		req.SetBasicAuth(cfg.Username, cfg.Password)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Errorf("expected %d, got %d", http.StatusNotFound, rec.Code)
		}
	})
}

func TestHandleDeliver(t *testing.T) {
	repo := newFakeRepo()
	cfg := newTestConfig()
	s := NewServer(repo, cfg)
	mux := createTestMux(s)

	repo.orders["abc"] = models.Order{ID: "abc"}

	t.Run("successful deliver", func(t *testing.T) {
		req := httptest.NewRequest("PUT", "/orders-deliver/abc", nil)
		req.SetBasicAuth(cfg.Username, cfg.Password)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("expected %d, got %d", http.StatusOK, rec.Code)
		}
	})

	t.Run("deliver error", func(t *testing.T) {
		repo.deliverErr = errors.New("some deliver err")
		req := httptest.NewRequest("PUT", "/orders-deliver/abc", nil)
		req.SetBasicAuth(cfg.Username, cfg.Password)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Errorf("expected %d, got %d", http.StatusNotFound, rec.Code)
		}
		repo.deliverErr = nil
	})

	t.Run("wrong method", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/orders-deliver/abc", nil)
		req.SetBasicAuth(cfg.Username, cfg.Password)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected %d, got %d", http.StatusMethodNotAllowed, rec.Code)
		}
	})
}

func TestHandleClientReturn(t *testing.T) {
	repo := newFakeRepo()
	cfg := newTestConfig()
	s := NewServer(repo, cfg)
	mux := createTestMux(s)

	repo.orders["xyz"] = models.Order{ID: "xyz"}

	t.Run("successful client return", func(t *testing.T) {
		req := httptest.NewRequest("PUT", "/orders-return/xyz", nil)
		req.SetBasicAuth(cfg.Username, cfg.Password)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("expected %d, got %d", http.StatusOK, rec.Code)
		}
	})

	t.Run("return error", func(t *testing.T) {
		repo.clientReturnErr = errors.New("return err")
		req := httptest.NewRequest("PUT", "/orders-return/xyz", nil)
		req.SetBasicAuth(cfg.Username, cfg.Password)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Errorf("expected %d, got %d", http.StatusNotFound, rec.Code)
		}
		repo.clientReturnErr = nil
	})

	t.Run("wrong method", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/orders-return/xyz", nil)
		req.SetBasicAuth(cfg.Username, cfg.Password)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected %d, got %d", http.StatusMethodNotAllowed, rec.Code)
		}
	})
}

func TestHandleGetReturns(t *testing.T) {
	repo := newFakeRepo()
	cfg := newTestConfig()
	s := NewServer(repo, cfg)
	mux := createTestMux(s)

	repo.orders["r1"] = models.Order{ID: "r1", ClientReturnAt: time.Now()}
	repo.orders["r2"] = models.Order{ID: "r2"}

	t.Run("successful get returns", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/returns?offset=0&limit=10", nil)
		req.SetBasicAuth(cfg.Username, cfg.Password)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("expected %d, got %d", http.StatusOK, rec.Code)
		}
		var orders []*models.Order
		_ = json.NewDecoder(rec.Body).Decode(&orders)
		if len(orders) != 1 {
			t.Errorf("expected 1 returned order, got %d", len(orders))
		}
	})

	t.Run("wrong method", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/returns", nil)
		req.SetBasicAuth(cfg.Username, cfg.Password)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected %d, got %d", http.StatusMethodNotAllowed, rec.Code)
		}
	})

	t.Run("repo error", func(t *testing.T) {
		repo.getReturnsErr = errors.New("some error in returns")
		req := httptest.NewRequest("GET", "/returns", nil)
		req.SetBasicAuth(cfg.Username, cfg.Password)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusInternalServerError {
			t.Errorf("expected %d, got %d", http.StatusInternalServerError, rec.Code)
		}
	})
}
