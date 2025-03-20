package server

import (
	"encoding/json"
	"gitlab.ozon.dev/qwestard/homework/internal/audit"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"gitlab.ozon.dev/qwestard/homework/internal/config"
	"gitlab.ozon.dev/qwestard/homework/internal/middleware"
	"gitlab.ozon.dev/qwestard/homework/internal/models"
	"gitlab.ozon.dev/qwestard/homework/internal/repository"
)

type Server struct {
	repo      repository.Repository
	user      string
	password  string
	addr      string
	auditPool *audit.AuditWorkerPool
}

func NewServer(repo repository.Repository, cfg *config.Config, auditPool *audit.AuditWorkerPool) *Server {
	return &Server{
		repo:      repo,
		user:      cfg.Username,
		password:  cfg.Password,
		addr:      cfg.Addr(),
		auditPool: auditPool,
	}
}

func (s *Server) RegisterRoutes(mux *http.ServeMux) {

	s.handleWith(mux, "/orders", s.handleOrders,
		[]string{"POST"},
	)

	s.handleWith(mux, "/orders/", s.handleOrderOne,
		[]string{"POST", "PUT", "DELETE"},
	)

	s.handleWith(mux, "/orders-deliver/", s.handleDeliver,
		[]string{"PUT"},
	)

	s.handleWith(mux, "/orders-return/", s.handleClientReturn,
		[]string{"PUT"},
	)

	mux.HandleFunc("/returns", s.handleGetReturns)
}

func (s *Server) Run() error {
	s.auditPool.Start(2)
	defer s.auditPool.Shutdown()

	mux := http.NewServeMux()

	s.RegisterRoutes(mux)

	log.Printf("Server listen on %s...", s.addr)
	return http.ListenAndServe(s.addr, mux)
}

func (s *Server) handleWith(mux *http.ServeMux, path string,
	handlerFunc http.HandlerFunc,
	methods []string,
) {
	finalHandler := middleware.LogMiddleware(s.auditPool, methods...)(
		middleware.BasicAuthMiddleware(s.user, s.password, methods...)(
			handlerFunc,
		),
	)
	mux.Handle(path, finalHandler)
}

func (s *Server) handleOrders(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		s.handleCreateOrder(w, r)
	case http.MethodGet:
		s.handleListOrders(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleOrderOne(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/orders/")
	if id == "" {
		http.Error(w, "missing ID", http.StatusBadRequest)
		return
	}
	switch r.Method {
	case http.MethodGet:
		s.handleGetOrder(w, r, id)
	case http.MethodPut:
		s.handleUpdateOrder(w, r, id)
	case http.MethodDelete:
		s.handleDeleteOrder(w, r, id)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleCreateOrder(w http.ResponseWriter, r *http.Request) {
	var o models.Order
	if err := json.NewDecoder(r.Body).Decode(&o); err != nil {
		http.Error(w, "bad JSON", http.StatusBadRequest)
		return
	}
	if time.Now().After(o.StorageDeadline) {
		http.Error(w, "wrong deadline", http.StatusBadRequest)
		s.auditPool.Log(audit.AuditLog{
			Timestamp: time.Now().UTC(),
			OrderID:   o.ID,
			Endpoint:  r.URL.Path,
			Request:   "create order " + o.ID,
			Response:  "deadline error",
			Message:   "storage_deadline is in the past",
		})
		return
	}
	o.LastStateChange = time.Now().UTC()
	if err := s.repo.Create(&o); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		s.auditPool.Log(audit.AuditLog{
			Timestamp: time.Now().UTC(),
			OrderID:   o.ID,
			Endpoint:  r.URL.Path,
			Request:   "create order " + o.ID,
			Response:  err.Error(),
			Message:   "order creation failed",
		})
		return
	}
	writeJSON(w, http.StatusCreated, o)
	s.auditPool.Log(audit.AuditLog{
		Timestamp: time.Now().UTC(),
		OrderID:   o.ID,
		Endpoint:  r.URL.Path,
		Request:   "create order " + o.ID,
		Response:  "order accepted",
		Message:   "order created successfully",
	})
}

func (s *Server) handleListOrders(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	cursor := q.Get("cursor")
	limitStr := q.Get("limit")
	limit, err := strconv.ParseInt(limitStr, 10, 64)
	if err != nil {
		limit = 10
	}
	recipientID := q.Get("recipient_id")

	orders, err := s.repo.List(cursor, limit, recipientID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		s.auditPool.Log(audit.AuditLog{
			Timestamp: time.Now().UTC(),
			Endpoint:  r.URL.Path,
			Request:   "list orders",
			Response:  err.Error(),
			Message:   "list orders failed",
		})
		return
	}
	writeJSON(w, http.StatusOK, orders)
	s.auditPool.Log(audit.AuditLog{
		Timestamp: time.Now().UTC(),
		Endpoint:  r.URL.Path,
		Request:   "list orders",
		Response:  "orders returned",
		Message:   "list orders succeeded",
	})
}

func (s *Server) handleGetOrder(w http.ResponseWriter, _ *http.Request, id string) {
	o, err := s.repo.GetByID(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		s.auditPool.Log(audit.AuditLog{
			Timestamp: time.Now().UTC(),
			OrderID:   id,
			Endpoint:  "/orders/" + id,
			Request:   "get order " + id,
			Response:  err.Error(),
			Message:   "get order failed",
		})
		return
	}
	if o == nil {
		http.Error(w, "not found", http.StatusNotFound)
		s.auditPool.Log(audit.AuditLog{
			Timestamp: time.Now().UTC(),
			OrderID:   id,
			Endpoint:  "/orders/" + id,
			Request:   "get order " + id,
			Response:  "not found",
			Message:   "order not found",
		})
		return
	}
	writeJSON(w, http.StatusOK, o)
	s.auditPool.Log(audit.AuditLog{
		Timestamp: time.Now().UTC(),
		OrderID:   id,
		Endpoint:  "/orders/" + id,
		Request:   "get order " + id,
		Response:  "order returned",
		Message:   "get order succeeded",
	})
}

func (s *Server) handleUpdateOrder(w http.ResponseWriter, r *http.Request, id string) {
	var updated models.Order
	if err := json.NewDecoder(r.Body).Decode(&updated); err != nil {
		http.Error(w, "bad JSON", http.StatusBadRequest)
		s.auditPool.Log(audit.AuditLog{
			Timestamp: time.Now().UTC(),
			OrderID:   id,
			Endpoint:  r.URL.Path,
			Request:   "update order " + id,
			Response:  "bad JSON",
			Message:   "JSON decode failed",
		})
		return
	}
	if updated.ID != id {
		http.Error(w, "ID mismatch", http.StatusBadRequest)
		s.auditPool.Log(audit.AuditLog{
			Timestamp: time.Now().UTC(),
			OrderID:   id,
			Endpoint:  r.URL.Path,
			Request:   "update order " + id,
			Response:  "ID mismatch",
			Message:   "order update failed: ID mismatch",
		})
		return
	}
	updated.LastStateChange = time.Now().UTC()
	if err := s.repo.Update(&updated); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		s.auditPool.Log(audit.AuditLog{
			Timestamp: time.Now().UTC(),
			OrderID:   id,
			Endpoint:  r.URL.Path,
			Request:   "update order " + id,
			Response:  err.Error(),
			Message:   "order update failed",
		})
		return
	}
	writeJSON(w, http.StatusOK, updated)
	s.auditPool.Log(audit.AuditLog{
		Timestamp: time.Now().UTC(),
		OrderID:   id,
		Endpoint:  r.URL.Path,
		Request:   "update order " + id,
		Response:  "order updated",
		Message:   "order updated successfully",
	})
}

func (s *Server) handleDeleteOrder(w http.ResponseWriter, _ *http.Request, id string) {
	if err := s.repo.Delete(id); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		s.auditPool.Log(audit.AuditLog{
			Timestamp: time.Now().UTC(),
			OrderID:   id,
			Endpoint:  "/orders/" + id,
			Request:   "delete order " + id,
			Response:  err.Error(),
			Message:   "order deletion failed",
		})
		return
	}
	w.WriteHeader(http.StatusNoContent)
	s.auditPool.Log(audit.AuditLog{
		Timestamp: time.Now().UTC(),
		OrderID:   id,
		Endpoint:  "/orders/" + id,
		Request:   "delete order " + id,
		Response:  "no content",
		Message:   "order deleted successfully",
	})
}

func (s *Server) handleDeliver(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/orders-deliver/")
	if err := s.repo.Deliver(id); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		s.auditPool.Log(audit.AuditLog{
			Timestamp: time.Now().UTC(),
			OrderID:   id,
			Endpoint:  r.URL.Path,
			Request:   "deliver order " + id,
			Response:  err.Error(),
			Message:   "deliver failed",
		})
		return
	}
	w.WriteHeader(http.StatusOK)
	s.auditPool.Log(audit.AuditLog{
		Timestamp: time.Now().UTC(),
		OrderID:   id,
		Endpoint:  r.URL.Path,
		Request:   "deliver order " + id,
		Response:  "order delivered",
		Message:   "deliver succeeded",
	})
}

func (s *Server) handleClientReturn(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/orders-return/")
	if err := s.repo.ClientReturn(id); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		s.auditPool.Log(audit.AuditLog{
			Timestamp: time.Now().UTC(),
			OrderID:   id,
			Endpoint:  r.URL.Path,
			Request:   "client return order " + id,
			Response:  err.Error(),
			Message:   "client return failed",
		})
		return
	}
	w.WriteHeader(http.StatusOK)
	s.auditPool.Log(audit.AuditLog{
		Timestamp: time.Now().UTC(),
		OrderID:   id,
		Endpoint:  r.URL.Path,
		Request:   "client return order " + id,
		Response:  "order returned by client",
		Message:   "client return succeeded",
	})
}

func (s *Server) handleGetReturns(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	offset, _ := strconv.ParseInt(q.Get("offset"), 10, 64)
	limit, _ := strconv.ParseInt(q.Get("limit"), 10, 64)
	recipientID := q.Get("recipient_id")

	orders, err := s.repo.GetReturns(offset, limit, recipientID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		s.auditPool.Log(audit.AuditLog{
			Timestamp: time.Now().UTC(),
			Endpoint:  r.URL.Path,
			Request:   "get returns",
			Response:  err.Error(),
			Message:   "get returns failed",
		})
		return
	}
	writeJSON(w, http.StatusOK, orders)
	s.auditPool.Log(audit.AuditLog{
		Timestamp: time.Now().UTC(),
		Endpoint:  r.URL.Path,
		Request:   "get returns",
		Response:  "orders returned",
		Message:   "get returns succeeded",
	})
}

func writeJSON(w http.ResponseWriter, code int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(data)
}
