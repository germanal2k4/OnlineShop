-- +goose Up
CREATE TABLE orders
(
    id                TEXT PRIMARY KEY,
    recipient_id      TEXT             NOT NULL,
    storage_deadline  TIMESTAMPTZ      NOT NULL,

    accepted_at       TIMESTAMPTZ,
    delivered_at      TIMESTAMPTZ,
    returned_at       TIMESTAMPTZ,
    client_return_at  TIMESTAMPTZ,
    last_state_change TIMESTAMPTZ      NOT NULL,

    weight            DOUBLE PRECISION NOT NULL DEFAULT 0,
    cost              DOUBLE PRECISION NOT NULL DEFAULT 0
);

CREATE TABLE order_packaging
(
    order_id  TEXT NOT NULL,
    pkg_value TEXT NOT NULL,
    CONSTRAINT order_packaging_pk PRIMARY KEY (order_id, pkg_value),
    CONSTRAINT order_packaging_fk FOREIGN KEY (order_id)
        REFERENCES orders (id)
        ON DELETE CASCADE
);

-- +goose Down
DROP TABLE order_packaging;
DROP TABLE orders;
