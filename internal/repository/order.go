package repository

import (
	"database/sql"
	"fmt"
	"strings"

	"gitlab.ozon.dev/qwestard/homework/internal/models"
)

type OrderRepository struct {
	db *sql.DB
}

func NewOrderRepository(db *sql.DB) *OrderRepository {
	return &OrderRepository{db: db}
}

func (r *OrderRepository) Create(o *models.Order) error {
	query := `INSERT INTO orders (
			id, recipient_id, storage_deadline, accepted_at, delivered_at, 
			returned_at, client_return_at, last_state_change, weight, cost, packaging
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`

	_, err := r.db.Exec(query,
		o.ID, o.RecipientID, o.StorageDeadline, o.AcceptedAt, o.DeliveredAt,
		o.ReturnedAt, o.ClientReturnAt, o.LastStateChange, o.Weight, o.Cost, pqStringArray(o.Packaging),
	)
	if err != nil {
		return fmt.Errorf("create order: %w", err)
	}
	return nil
}

// GetByID — возвращает заказ по ID.
func (r *OrderRepository) GetByID(id string) (*models.Order, error) {
	query := `SELECT 
			id, recipient_id, storage_deadline, 
			accepted_at, delivered_at, returned_at, client_return_at, last_state_change,
			weight, cost, packaging
		FROM orders WHERE id=$1`

	row := r.db.QueryRow(query, id)
	o := &models.Order{}
	var packaging []string

	err := row.Scan(
		&o.ID, &o.RecipientID, &o.StorageDeadline,
		&o.AcceptedAt, &o.DeliveredAt, &o.ReturnedAt, &o.ClientReturnAt, &o.LastStateChange,
		&o.Weight, &o.Cost, &packaging,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get order by id: %w", err)
	}
	o.Packaging = packaging
	return o, nil
}

func (r *OrderRepository) Update(o *models.Order) error {
	query := `UPDATE orders SET 
			recipient_id=$1, storage_deadline=$2,
			accepted_at=$3, delivered_at=$4, returned_at=$5, client_return_at=$6, last_state_change=$7,
			weight=$8, cost=$9, packaging=$10
		WHERE id=$11`
	res, err := r.db.Exec(query,
		o.RecipientID, o.StorageDeadline,
		o.AcceptedAt, o.DeliveredAt, o.ReturnedAt, o.ClientReturnAt, o.LastStateChange,
		o.Weight, o.Cost, pqStringArray(o.Packaging),
		o.ID,
	)
	if err != nil {
		return fmt.Errorf("update order: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("order %s not found", o.ID)
	}
	return nil
}

func (r *OrderRepository) Delete(id string) error {
	query := `DELETE FROM orders WHERE id=$1`
	res, err := r.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("delete order: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("order %s not found", id)
	}
	return nil
}

func (r *OrderRepository) List(cursor string, limit int64) ([]*models.Order, error) {
	if limit <= 0 {
		limit = 10
	}
	var filters []string
	var args []interface{}
	idx := 1

	query := `SELECT 
			id, recipient_id, storage_deadline,
			accepted_at, delivered_at, returned_at, client_return_at, last_state_change,
			weight, cost, packaging
		FROM orders`
	if cursor != "" {
		filters = append(filters, fmt.Sprintf("id>$%d", idx))
		args = append(args, cursor)
		idx++
	}
	if len(filters) > 0 {
		query += " WHERE " + strings.Join(filters, " AND ")
	}
	st := ` ORDER BY id ASC`
	query += st
	query += fmt.Sprintf(" LIMIT $%d", idx)
	args = append(args, limit)

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list orders: %w", err)
	}
	defer rows.Close()

	var res []*models.Order
	for rows.Next() {
		o := &models.Order{}
		var packaging []string
		err := rows.Scan(
			&o.ID, &o.RecipientID, &o.StorageDeadline,
			&o.AcceptedAt, &o.DeliveredAt, &o.ReturnedAt, &o.ClientReturnAt, &o.LastStateChange,
			&o.Weight, &o.Cost, &packaging,
		)
		if err != nil {
			return nil, fmt.Errorf("scan order: %w", err)
		}
		o.Packaging = packaging
		res = append(res, o)
	}
	return res, nil
}

func pqStringArray(a []string) interface{} {
	return a
}
