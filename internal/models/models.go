package models

import "time"

type OrderState string

const (
	OrderStateAccepted  OrderState = "accepted"
	OrderStateDelivered OrderState = "delivered"
	OrderStateReturned  OrderState = "returned"
	OrderStateClientRtn OrderState = "client_rtn"
)

type Order struct {
	ID              string    `json:"id"`
	RecipientID     string    `json:"recipient_id"`
	StorageDeadline time.Time `json:"storage_deadline"`
	AcceptedAt      time.Time `json:"accepted_at,omitempty"`
	DeliveredAt     time.Time `json:"delivered_at,omitempty"`
	ReturnedAt      time.Time `json:"returned_at,omitempty"`
	ClientReturnAt  time.Time `json:"client_return_at,omitempty"`
	LastStateChange time.Time `json:"last_state_change"`
	Weight          float64   `json:"weight"`
	Cost            float64   `json:"cost"`
	Packaging       []string  `json:"packaging"`
}

func (o *Order) UpdateState(newState OrderState) {
	now := time.Now().UTC()
	o.LastStateChange = now
	switch newState {
	case OrderStateAccepted:
		if o.AcceptedAt.IsZero() {
			o.AcceptedAt = now
		}
	case OrderStateDelivered:
		o.DeliveredAt = now
	case OrderStateClientRtn:
		o.ClientReturnAt = now
	case OrderStateReturned:
		o.ReturnedAt = now
	}
}

func (o *Order) CurrentState() OrderState {
	if !o.ReturnedAt.IsZero() {
		return OrderStateReturned
	}
	if !o.ClientReturnAt.IsZero() {
		return OrderStateClientRtn
	}
	if !o.DeliveredAt.IsZero() {
		return OrderStateDelivered
	}
	if !o.AcceptedAt.IsZero() {
		return OrderStateAccepted
	}
	return ""
}
