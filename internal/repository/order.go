package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"gitlab.ozon.dev/qwestard/homework/internal/models"
)

type Repository interface {
	Create(o *models.Order) error
	List(cursor string, limit int64, recipientID string) ([]*models.Order, error)
	GetByID(tx *sql.Tx, id string) (*models.Order, error)
	GetID(id string) (*models.Order, error)
	Update(tx *sql.Tx, o *models.Order) error
	Delete(id string) error
	Deliver(id string) error
	ClientReturn(id string) error
	GetReturns(offset, limit int64, recipientID string) ([]*models.Order, error)
	ReturnOrder(id string) error
	AcceptOrder(id string) error
	fetchPackaging(orderID string) ([]string, error)
	UpdateTx(o *models.Order) error
}

type OrderRepository struct {
	db *sql.DB
}

func NewOrderRepository(db *sql.DB) *OrderRepository {
	return &OrderRepository{db: db}
}

func (r *OrderRepository) Create(o *models.Order) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	query := `INSERT INTO orders (
		id, recipient_id, storage_deadline, accepted_at, delivered_at,
		returned_at, client_return_at, last_state_change, weight, cost
	) VALUES ($1,
	          $2,
	          $3,
	          $4,
	          $5,
	          $6,
	          $7,
	          $8,
	          $9,
	          $10)`

	_, err = tx.Exec(query,
		o.ID,
		o.RecipientID,
		o.StorageDeadline,
		o.AcceptedAt,
		o.DeliveredAt,
		o.ReturnedAt,
		o.ClientReturnAt,
		o.LastStateChange,
		o.Weight,
		o.Cost,
	)
	if err != nil {
		return fmt.Errorf("create orders: %w", err)
	}

	if err := insertPackaging(tx, o.ID, o.Packaging); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func insertPackaging(tx *sql.Tx, orderID string, packaging []string) error {
	for _, pkg := range packaging {
		q := `INSERT INTO order_packaging(order_id, pkg_value) VALUES($1,$2)`
		if _, err := tx.Exec(q, orderID, pkg); err != nil {
			return fmt.Errorf("insertPackaging: %w", err)
		}
	}
	return nil
}

func (r *OrderRepository) GetByID(tx *sql.Tx, id string) (*models.Order, error) {
	o := &models.Order{}
	query := `SELECT
		id, recipient_id, storage_deadline,
		accepted_at, delivered_at, returned_at, client_return_at,
		last_state_change, weight, cost
	FROM orders WHERE id=$1 FOR UPDATE`
	row := tx.QueryRow(query, id)
	err := row.Scan(
		&o.ID, &o.RecipientID, &o.StorageDeadline,
		&o.AcceptedAt, &o.DeliveredAt, &o.ReturnedAt, &o.ClientReturnAt,
		&o.LastStateChange, &o.Weight, &o.Cost,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil || row.Err() != nil {
		return nil, fmt.Errorf("GetByID: %w", err)
	}
	rows, err := tx.Query(`SELECT pkg_value FROM order_packaging WHERE order_id=$1`, id)
	if err != nil {
		return nil, fmt.Errorf("GetByID: %w", err)
	}
	defer rows.Close()
	var pkgs []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		if rows.Err() != nil {
			return nil, rows.Err()
		}
		pkgs = append(pkgs, s)
	}
	o.Packaging = pkgs
	return o, nil
}

func (r *OrderRepository) GetID(id string) (*models.Order, error) {

	o := &models.Order{}

	query := `SELECT
		id, recipient_id, storage_deadline,
		accepted_at, delivered_at, returned_at, client_return_at,
		last_state_change, weight, cost
	FROM orders WHERE id=$1`
	row := r.db.QueryRow(query, id)
	err := row.Scan(
		&o.ID, &o.RecipientID, &o.StorageDeadline,
		&o.AcceptedAt, &o.DeliveredAt, &o.ReturnedAt, &o.ClientReturnAt,
		&o.LastStateChange, &o.Weight, &o.Cost,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil || row.Err() != nil {
		return nil, fmt.Errorf("GetByID: %w", err)
	}
	pkgs, err := r.fetchPackaging(id)
	if err != nil {
		return nil, err
	}
	o.Packaging = pkgs
	return o, nil

}

func (r *OrderRepository) fetchPackaging(orderID string) ([]string, error) {
	var result []string
	rows, err := r.db.Query(`SELECT pkg_value FROM order_packaging WHERE order_id=$1`, orderID)
	if err != nil {
		return nil, fmt.Errorf("fetchPackaging: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		if rows.Err() != nil {
			return nil, rows.Err()
		}
		result = append(result, s)
	}
	return result, nil
}

func (r *OrderRepository) Update(tx *sql.Tx, o *models.Order) error {

	query := `UPDATE orders SET
		recipient_id=$1, storage_deadline=$2,
		accepted_at=$3, delivered_at=$4,
		returned_at=$5, client_return_at=$6,
		last_state_change=$7, weight=$8, cost=$9
	WHERE id=$10`
	res, err := tx.Exec(query,
		o.RecipientID, o.StorageDeadline,
		o.AcceptedAt, o.DeliveredAt,
		o.ReturnedAt, o.ClientReturnAt,
		o.LastStateChange, o.Weight, o.Cost,
		o.ID,
	)
	if err != nil {
		return fmt.Errorf("update order: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("order %s not found", o.ID)
	}

	if _, err := tx.Exec(`DELETE FROM order_packaging WHERE order_id=$1`, o.ID); err != nil {
		return fmt.Errorf("delete packaging: %w", err)
	}
	if err := insertPackaging(tx, o.ID, o.Packaging); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *OrderRepository) UpdateTx(o *models.Order) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	query := `UPDATE orders SET
		recipient_id=$1, storage_deadline=$2,
		accepted_at=$3, delivered_at=$4,
		returned_at=$5, client_return_at=$6,
		last_state_change=$7, weight=$8, cost=$9
	WHERE id=$10`
	res, err := tx.Exec(query,
		o.RecipientID, o.StorageDeadline,
		o.AcceptedAt, o.DeliveredAt,
		o.ReturnedAt, o.ClientReturnAt,
		o.LastStateChange, o.Weight, o.Cost,
		o.ID,
	)
	if err != nil {
		return fmt.Errorf("update order: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("order %s not found", o.ID)
	}

	if _, err := tx.Exec(`DELETE FROM order_packaging WHERE order_id=$1`, o.ID); err != nil {
		return fmt.Errorf("delete packaging: %w", err)
	}
	if err := insertPackaging(tx, o.ID, o.Packaging); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *OrderRepository) Delete(id string) error {
	res, err := r.db.Exec(`DELETE FROM orders WHERE id=$1`, id)
	if err != nil {
		return fmt.Errorf("delete order: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("order %s not found", id)
	}
	return nil
}

func (r *OrderRepository) Deliver(id string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	o, err := r.GetByID(tx, id)
	if err != nil {
		return err
	}
	if o == nil {
		return fmt.Errorf("order %s not found", id)
	}
	o.UpdateState(models.OrderStateDelivered)
	o.DeliveredAt = time.Now().UTC()
	o.LastStateChange = time.Now().UTC()

	if err := r.Update(tx, o); err != nil {
		return err
	}
	return tx.Commit()

}

func (r *OrderRepository) ClientReturn(id string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	o, err := r.GetByID(tx, id)
	if err != nil {
		return err
	}
	if o == nil {
		return fmt.Errorf("order %s not found", id)
	}

	o.UpdateState(models.OrderStateClientRtn)
	o.ClientReturnAt = time.Now().UTC()
	o.LastStateChange = time.Now().UTC()

	if err := r.Update(tx, o); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *OrderRepository) GetReturns(offset int64, limit int64, recipientID string) ([]*models.Order, error) {
	if limit <= 0 {
		limit = 10
	}

	var b strings.Builder
	b.WriteString(`SELECT id FROM orders WHERE client_return_at IS NOT NULL`)

	if recipientID != "" {
		b.WriteString(` AND recipient_id = $3`)
	}

	b.WriteString(` ORDER BY id ASC LIMIT $1 OFFSET $2`)

	var args []any
	args = append(args, limit, offset)

	if recipientID != "" {
		args = append(args, recipientID)
	}

	tx, err := r.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	rows, err := tx.Query(b.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("GetReturns: %w", err)
	}
	defer rows.Close()

	var result []*models.Order
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		if rows.Err() != nil {
			return nil, rows.Err()
		}
		o, err := r.GetByID(tx, id)
		if err != nil {
			return nil, err
		}
		result = append(result, o)
	}
	return result, tx.Commit()
}

func (r *OrderRepository) List(cursor string, limit int64, recipientID string) ([]*models.Order, error) {
	if limit <= 0 {
		limit = 10
	}

	var (
		sb         strings.Builder
		args       []interface{}
		conditions []string
		paramIndex = 1
	)

	sb.WriteString("SELECT id FROM orders")

	if cursor != "" {
		conditions = append(conditions, fmt.Sprintf("id > $%d", paramIndex))
		args = append(args, cursor)
		paramIndex++
	}

	if recipientID != "" {
		conditions = append(conditions, fmt.Sprintf("recipient_id = $%d", paramIndex))
		args = append(args, recipientID)
		paramIndex++
	}

	if len(conditions) > 0 {
		sb.WriteString(" WHERE ")
		sb.WriteString(strings.Join(conditions, " AND "))
	}

	sb.WriteString(fmt.Sprintf(" ORDER BY id ASC LIMIT $%d", paramIndex))
	args = append(args, limit)
	paramIndex++

	query := sb.String()

	tx, err := r.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	rows, err := tx.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list orders: %w", err)
	}
	defer rows.Close()

	var orders []*models.Order

	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		o, err := r.GetByID(tx, id)
		if err != nil {
			return nil, err
		}
		orders = append(orders, o)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return orders, tx.Commit()
}

func (r *OrderRepository) AcceptOrder(id string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	o, err := r.GetByID(tx, id)
	if err != nil {
		return err
	}
	if o == nil {
		return fmt.Errorf("order %s not found", id)
	}

	o.UpdateState(models.OrderStateAccepted)
	o.AcceptedAt = time.Now().UTC()
	o.LastStateChange = time.Now().UTC()

	if err := r.Update(tx, o); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *OrderRepository) ReturnOrder(id string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	o, err := r.GetByID(tx, id)
	if err != nil {
		return err
	}
	if o == nil {
		return fmt.Errorf("order %s not found", id)
	}
	o.UpdateState(models.OrderStateReturned)
	o.ReturnedAt = time.Now().UTC()
	o.LastStateChange = time.Now().UTC()

	if err := r.Update(tx, o); err != nil {
		return err
	}
	return tx.Commit()
}
