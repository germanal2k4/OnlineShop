package server

import (
	"context"
	"encoding/json"
	"fmt"
	"homework/internal/audit"
	"homework/internal/config"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"homework/internal/middleware"
	"homework/internal/models"
	"homework/internal/wrapper"
)

type Server struct {
	wrap      *wrapper.OrderWrapper
	user      string
	password  string
	addr      string
	auditPool *audit.AuditWorkerPool
}

func NewServer(wrap *wrapper.OrderWrapper, cfg *config.Config, auditPool *audit.AuditWorkerPool) *Server {
	return &Server{
		wrap:      wrap,
		user:      cfg.Username,
		password:  cfg.Password,
		addr:      cfg.Addr(),
		auditPool: auditPool,
	}
}

func (s *Server) logStatusTransition(orderID, oldState, newState, endpoint string) {
	s.auditPool.Log(audit.AuditLog{
		Timestamp: time.Now().UTC(),
		OrderID:   orderID,
		OldState:  oldState,
		NewState:  newState,
		Endpoint:  endpoint,
		Request:   "status transition",
		Response:  fmt.Sprintf("%s -> %s", oldState, newState),
		Message:   "status transition succeeded",
	})
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

	s.handleWith(mux, "/orders-accept/", s.handleAccept, []string{"PUT"})
	s.handleWith(mux, "/orders-courier-return/", s.handleCourierReturn, []string{"PUT"})

	mux.Handle("/returns", middleware.AuditResponseMiddleware(s.auditPool)(http.HandlerFunc(s.handleGetReturns)))

	mux.Handle("/history", middleware.AuditResponseMiddleware(s.auditPool)(http.HandlerFunc(s.handleOrderHistory)))
}

func (s *Server) Run() error {
	ctx, cancel := context.WithCancel(context.Background())

	s.auditPool.Start(ctx)

	defer func() {
		cancel()
		s.auditPool.Shutdown()
	}()

	mux := http.NewServeMux()

	s.RegisterRoutes(mux)

	log.Printf("Server listen on %s...", s.addr)
	return http.ListenAndServe(s.addr, mux)
}

func (s *Server) handleWith(mux *http.ServeMux, path string,
	handlerFunc http.HandlerFunc,
	methods []string,
) {
	finalHandler := middleware.AuditResponseMiddleware(s.auditPool)(
		middleware.LogMiddleware(s.auditPool, methods...)(
			middleware.BasicAuthMiddleware(s.user, s.password, methods...)(
				handlerFunc,
			),
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
		return
	}
	o.LastStateChange = time.Now().UTC()

	if err := s.wrap.CreateOrder(&o); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}

	writeJSON(w, http.StatusCreated, o)
	s.logStatusTransition(o.ID, "", string(models.OrderStateAccepted), r.URL.Path)
}

func (s *Server) handleListOrders(w http.ResponseWriter, _ *http.Request) {
	orders, err := s.wrap.ListActiveOrders()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, orders)
}

func (s *Server) handleGetOrder(w http.ResponseWriter, _ *http.Request, id string) {
	o, err := s.wrap.GetOrderByID(id)
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
	oldOrder, err := s.wrap.GetOrderByID(id)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	oldState := ""
	if oldOrder != nil {
		oldState = string(oldOrder.CurrentState())
	}

	updated.LastStateChange = time.Now().UTC()
	if err := s.wrap.UpdateOrder(&updated); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, updated)
	newState := string(updated.CurrentState())
	if oldState != newState {
		s.logStatusTransition(id, oldState, newState, r.URL.Path)
	}
}

func (s *Server) handleDeleteOrder(w http.ResponseWriter, _ *http.Request, id string) {
	if err := s.wrap.DeleteOrder(id); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
	s.logStatusTransition(id, "existing", "deleted", "/orders/"+id)
}

func (s *Server) handleDeliver(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/orders-deliver/")
	if err := s.wrap.DeliverOrder(id); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusOK)
	s.logStatusTransition(id, "", string(models.OrderStateDelivered), r.URL.Path)
}

func (s *Server) handleClientReturn(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/orders-return/")
	if err := s.wrap.ClientReturnOrder(id); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusOK)
	s.logStatusTransition(id, "", string(models.OrderStateClientRtn), r.URL.Path)
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

	orders, err := s.wrap.GetReturns(offset, limit, recipientID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, orders)
}

func (s *Server) handleAccept(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/orders-accept/")
	if err := s.wrap.AcceptOrder(id); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusOK)
	s.logStatusTransition(id, "", string(models.OrderStateAccepted), r.URL.Path)

}

func (s *Server) handleCourierReturn(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/orders-courier-return/")
	if err := s.wrap.CourierReturnOrder(id); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusOK)
	s.logStatusTransition(id, "", string(models.OrderStateReturned), r.URL.Path)
}

func (s *Server) handleOrderHistory(w http.ResponseWriter, _ *http.Request) {
	orders, err := s.wrap.ListHistoryOrders()
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
