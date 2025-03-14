package server

import (
	"encoding/json"
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
	repo     repository.Repository
	user     string
	password string
	addr     string
}

func NewServer(repo repository.Repository, cfg *config.Config) *Server {
	return &Server{
		repo:     repo,
		user:     cfg.Username,
		password: cfg.Password,
		addr:     cfg.Addr(),
	}
}

func (s *Server) RegisterRoutes(mux *http.ServeMux) {

	s.handleWith(mux, "/orders", s.handleOrders,
		[]string{"POST"}, []string{"POST"},
	)

	s.handleWith(mux, "/orders/", s.handleOrderOne,
		[]string{"POST", "PUT", "DELETE"},
		[]string{"POST", "PUT", "DELETE"},
	)

	s.handleWith(mux, "/orders-deliver/", s.handleDeliver,
		[]string{"PUT"}, []string{"PUT"},
	)

	s.handleWith(mux, "/orders-return/", s.handleClientReturn,
		[]string{"PUT"}, []string{"PUT"},
	)

	mux.HandleFunc("/returns", s.handleGetReturns)
}

func (s *Server) Run() error {
	mux := http.NewServeMux()

	s.RegisterRoutes(mux)

	log.Printf("Server listen on %s...", s.addr)
	return http.ListenAndServe(s.addr, mux)
}

func (s *Server) handleWith(mux *http.ServeMux, path string,
	handlerFunc http.HandlerFunc,
	logMethods []string, authMethods []string,
) {
	finalHandler := middleware.LogMiddleware(logMethods...)(
		middleware.BasicAuthMiddleware(s.user, s.password, authMethods...)(
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
	}
	o.LastStateChange = time.Now().UTC()
	if err := s.repo.Create(&o); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	writeJSON(w, http.StatusCreated, o)
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
		return
	}
	writeJSON(w, http.StatusOK, orders)
}

func (s *Server) handleGetOrder(w http.ResponseWriter, _ *http.Request, id string) {
	o, err := s.repo.GetByID(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if o == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, o)
}

func (s *Server) handleUpdateOrder(w http.ResponseWriter, r *http.Request, id string) {
	var updated models.Order
	if err := json.NewDecoder(r.Body).Decode(&updated); err != nil {
		http.Error(w, "bad JSON", http.StatusBadRequest)
		return
	}
	if updated.ID != id {
		http.Error(w, "ID mismatch", http.StatusBadRequest)
		return
	}
	updated.LastStateChange = time.Now().UTC()
	if err := s.repo.Update(&updated); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) handleDeleteOrder(w http.ResponseWriter, _ *http.Request, id string) {
	if err := s.repo.Delete(id); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleDeliver(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/orders-deliver/")
	if err := s.repo.Deliver(id); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleClientReturn(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/orders-return/")
	if err := s.repo.ClientReturn(id); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusOK)
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
		return
	}
	writeJSON(w, http.StatusOK, orders)
}

func writeJSON(w http.ResponseWriter, code int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(data)
}
