package server

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"gitlab.ozon.dev/qwestard/homework/internal/models"
	"gitlab.ozon.dev/qwestard/homework/internal/repository"
)

type Server struct {
	repo     *repository.OrderRepository
	user     string
	password string
	addr     string
}

func NewServer(repo *repository.OrderRepository, user, pass, addr string) *Server {
	return &Server{
		repo:     repo,
		user:     user,
		password: pass,
		addr:     addr,
	}
}

func (s *Server) Run() error {
	mux := http.NewServeMux()

	mux.Handle("/orders", s.logMiddleware(s.basicAuthMiddleware(http.HandlerFunc(s.handleOrders), "POST"), "POST"))

	mux.HandleFunc("/orders", s.handleOrders)

	mux.Handle("/orders/", s.logMiddleware(s.basicAuthMiddleware(http.HandlerFunc(s.handleOrderOne), "POST,PUT,DELETE"), "POST,PUT,DELETE"))

	log.Printf("Сервер слушает на %s ...", s.addr)
	return http.ListenAndServe(s.addr, mux)
}

func (s *Server) handleOrders(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		s.handleCreateOrder(w, r)
	case http.MethodGet:
		s.handleListOrders(w, r)
	default:
		http.Error(w, "метод не поддерживается", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleOrderOne(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/orders/")
	if id == "" {
		http.Error(w, "не указан {id}", http.StatusBadRequest)
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
		http.Error(w, "метод не поддерживается", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleCreateOrder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "допустим только POST", http.StatusMethodNotAllowed)
		return
	}
	var order models.Order
	if err := json.NewDecoder(r.Body).Decode(&order); err != nil {
		http.Error(w, "ошибка парсинга JSON", http.StatusBadRequest)
		return
	}
	if order.ID == "" {
		http.Error(w, "пустое поле ID", http.StatusBadRequest)
		return
	}
	order.LastStateChange = time.Now().UTC()

	if err := s.repo.Create(&order); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, order)
}

func (s *Server) handleListOrders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "только GET", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	cursor := q.Get("cursor")
	limitStr := q.Get("limit")
	var limit int64 = 10
	if limitStr != "" {
		if l, err := strconv.ParseInt(limitStr, 10, 64); err == nil {
			limit = l
		}
	}
	orders, err := s.repo.List(cursor, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, orders)
}

func (s *Server) handleGetOrder(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodGet {
		http.Error(w, "только GET", http.StatusMethodNotAllowed)
		return
	}
	o, err := s.repo.GetByID(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if o == nil {
		http.Error(w, "не найден", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, o)
}

func (s *Server) handleUpdateOrder(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPut {
		http.Error(w, "допустим только PUT", http.StatusMethodNotAllowed)
		return
	}
	var updated models.Order
	if err := json.NewDecoder(r.Body).Decode(&updated); err != nil {
		http.Error(w, "ошибка парсинга JSON", http.StatusBadRequest)
		return
	}
	if updated.ID != id {
		http.Error(w, "ID не совпадает", http.StatusBadRequest)
		return
	}
	updated.LastStateChange = time.Now().UTC()
	if err := s.repo.Update(&updated); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) handleDeleteOrder(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodDelete {
		http.Error(w, "допустим только DELETE", http.StatusMethodNotAllowed)
		return
	}
	if err := s.repo.Delete(id); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeJSON(w http.ResponseWriter, code int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(data)
}

func (s *Server) basicAuthMiddleware(next http.Handler, methods string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isMethodInList(r.Method, methods) {
			next.ServeHTTP(w, r)
			return
		}
		u, p, ok := r.BasicAuth()
		if !ok || u != s.user || p != s.password {
			w.Header().Set("WWW-Authenticate", `Basic realm="orders"`)
			http.Error(w, "не авторизован", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) logMiddleware(next http.Handler, methods string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isMethodInList(r.Method, methods) {
			log.Printf("[%s] %s", r.Method, r.URL.Path)
		}
		next.ServeHTTP(w, r)
	})
}

func isMethodInList(method, list string) bool {
	for _, m := range strings.Split(list, ",") {
		if strings.TrimSpace(m) == method {
			return true
		}
	}
	return false
}
