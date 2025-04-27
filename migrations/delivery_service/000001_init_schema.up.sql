CREATE TABLE delivery_zones (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    code VARCHAR(50) NOT NULL UNIQUE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP
);

-- Создаем базовую зону доставки
INSERT INTO delivery_zones (name, code) VALUES ('Зона 1', 'ZONE_1');

CREATE TABLE delivery_slots (
    id SERIAL PRIMARY KEY,
    start_time TIMESTAMP NOT NULL,
    end_time TIMESTAMP NOT NULL,
    zone_id INTEGER NOT NULL DEFAULT 1,
    capacity INTEGER NOT NULL DEFAULT 10,
    available INTEGER NOT NULL DEFAULT 10,
    max_deliveries INTEGER NOT NULL DEFAULT 10,
    current_load INTEGER NOT NULL DEFAULT 0,
    is_disabled BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP,
    CONSTRAINT fk_delivery_slot_zone FOREIGN KEY (zone_id) REFERENCES delivery_zones(id) ON DELETE CASCADE
);

CREATE TABLE couriers (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    phone VARCHAR(255) NOT NULL,
    email VARCHAR(255) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'available',
    current_zone_id INTEGER,
    vehicle_type VARCHAR(50),
    vehicle_number VARCHAR(50),
    capacity INTEGER,
    rating DECIMAL(3,2),
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP,
    CONSTRAINT fk_courier_zone FOREIGN KEY (current_zone_id) REFERENCES delivery_zones(id) ON DELETE SET NULL
);

CREATE TABLE courier_schedules (
    id SERIAL PRIMARY KEY,
    courier_id INTEGER NOT NULL,
    slot_id INTEGER NOT NULL,
    order_id INTEGER,
    delivery_id INTEGER,
    start_time TIMESTAMP NOT NULL,
    end_time TIMESTAMP NOT NULL,
    is_available BOOLEAN NOT NULL DEFAULT true,
    is_reserved BOOLEAN NOT NULL DEFAULT false,
    is_completed BOOLEAN NOT NULL DEFAULT false,
    notes TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP,
    CONSTRAINT fk_schedule_courier FOREIGN KEY (courier_id) REFERENCES couriers(id) ON DELETE CASCADE,
    CONSTRAINT fk_schedule_slot FOREIGN KEY (slot_id) REFERENCES delivery_slots(id) ON DELETE CASCADE
);

CREATE TABLE delivery (
    id SERIAL PRIMARY KEY,
    order_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,
    courier_id INTEGER,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    scheduled_start_time TIMESTAMP,
    scheduled_end_time TIMESTAMP,
    actual_start_time TIMESTAMP,
    actual_end_time TIMESTAMP,
    delivery_address TEXT NOT NULL,
    recipient_name VARCHAR(255) NOT NULL,
    recipient_phone VARCHAR(255) NOT NULL,
    notes TEXT,
    tracking_code VARCHAR(100),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP,
    CONSTRAINT fk_delivery_courier FOREIGN KEY (courier_id) REFERENCES couriers(id) ON DELETE SET NULL
);

CREATE INDEX idx_delivery_slots_time ON delivery_slots(start_time, end_time);
CREATE INDEX idx_couriers_active ON couriers(is_active);
CREATE INDEX idx_courier_schedules_courier ON courier_schedules(courier_id);
CREATE INDEX idx_courier_schedules_slot ON courier_schedules(slot_id);
CREATE INDEX idx_courier_schedules_available ON courier_schedules(is_available);
CREATE INDEX idx_delivery_order_id ON delivery(order_id);
CREATE INDEX idx_delivery_user_id ON delivery(user_id);
CREATE INDEX idx_delivery_courier_id ON delivery(courier_id);
CREATE INDEX idx_delivery_status ON delivery(status); 