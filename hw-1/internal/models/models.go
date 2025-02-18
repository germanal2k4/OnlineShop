package models

import "time"

type OrderState string

// небольшой enum для удобства
const (
	OrderStateAccepted  OrderState = "accepted"
	OrderStateDelivered OrderState = "delivered"
	OrderStateReturned  OrderState = "returned"
	OrderStateClientRtn OrderState = "client_rtn"
)

// Order - структура заказа
type Order struct {
	ID              string     `json:"id"`
	RecipientID     string     `json:"recipient_id"`
	StorageDeadline time.Time  `json:"storage_deadline"`
	State           OrderState `json:"state"`
	AcceptedAt      *time.Time `json:"accepted_at,omitempty"`
	DeliveredAt     *time.Time `json:"delivered_at,omitempty"`
	ReturnedAt      *time.Time `json:"returned_at,omitempty"`
	ClientReturnAt  *time.Time `json:"client_return_at,omitempty"`
	LastStateChange time.Time  `json:"last_state_change"`
}
