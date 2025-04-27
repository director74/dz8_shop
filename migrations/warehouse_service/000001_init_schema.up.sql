CREATE TABLE products (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    price DECIMAL(12, 2) NOT NULL,
    sku VARCHAR(50) NOT NULL UNIQUE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP
);

CREATE TABLE warehouse_items (
    id SERIAL PRIMARY KEY,
    product_id INTEGER NOT NULL,
    quantity BIGINT NOT NULL DEFAULT 0,
    reserved_quantity BIGINT NOT NULL DEFAULT 0,
    available BIGINT GENERATED ALWAYS AS (quantity - reserved_quantity) STORED,
    location VARCHAR(100),
    name VARCHAR(255) NOT NULL,
    sku VARCHAR(50) NOT NULL,
    price DECIMAL(12, 2) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP,
    CONSTRAINT fk_warehouse_product FOREIGN KEY (product_id) REFERENCES products(id) ON DELETE CASCADE
);

CREATE TABLE warehouse_reservations (
    id SERIAL PRIMARY KEY,
    order_id INTEGER NOT NULL,
    product_id INTEGER NOT NULL,
    warehouse_item_id INTEGER NOT NULL,
    quantity INTEGER NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    expires_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP,
    CONSTRAINT fk_reservation_product FOREIGN KEY (product_id) REFERENCES products(id) ON DELETE CASCADE,
    CONSTRAINT fk_reservation_warehouse_item FOREIGN KEY (warehouse_item_id) REFERENCES warehouse_items(id) ON DELETE CASCADE
);

CREATE INDEX idx_products_sku ON products(sku);
CREATE INDEX idx_warehouse_items_product_id ON warehouse_items(product_id);
CREATE INDEX idx_warehouse_items_sku ON warehouse_items(sku);
CREATE INDEX idx_warehouse_items_available ON warehouse_items(available);
CREATE INDEX idx_warehouse_reservations_order_id ON warehouse_reservations(order_id);
CREATE INDEX idx_warehouse_reservations_product_id ON warehouse_reservations(product_id);
CREATE INDEX idx_warehouse_reservations_status ON warehouse_reservations(status);
CREATE INDEX idx_warehouse_reservations_warehouse_item_id ON warehouse_reservations(warehouse_item_id); 