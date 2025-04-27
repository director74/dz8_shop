DROP INDEX IF EXISTS idx_delivery_orders_status;
DROP INDEX IF EXISTS idx_delivery_orders_slot_id;
DROP INDEX IF EXISTS idx_delivery_orders_courier_id;
DROP INDEX IF EXISTS idx_delivery_orders_user_id;
DROP INDEX IF EXISTS idx_delivery_orders_order_id;
DROP INDEX IF EXISTS idx_courier_schedules_available;
DROP INDEX IF EXISTS idx_courier_schedules_slot;
DROP INDEX IF EXISTS idx_courier_schedules_courier;
DROP INDEX IF EXISTS idx_couriers_active;
DROP INDEX IF EXISTS idx_delivery_slots_time;

DROP TABLE IF EXISTS delivery_orders;
DROP TABLE IF EXISTS courier_schedules;
DROP TABLE IF EXISTS couriers;
DROP TABLE IF EXISTS delivery_slots; 