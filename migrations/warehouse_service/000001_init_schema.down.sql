DROP INDEX IF EXISTS idx_warehouse_reservations_warehouse_item_id;
DROP INDEX IF EXISTS idx_warehouse_reservations_status;
DROP INDEX IF EXISTS idx_warehouse_reservations_product_id;
DROP INDEX IF EXISTS idx_warehouse_reservations_order_id;
DROP INDEX IF EXISTS idx_warehouse_items_sku;
DROP INDEX IF EXISTS idx_warehouse_items_product_id;
DROP INDEX IF EXISTS idx_warehouse_items_available;
DROP INDEX IF EXISTS idx_products_sku;

DROP TABLE IF EXISTS warehouse_reservations;
DROP TABLE IF EXISTS warehouse_items;
DROP TABLE IF EXISTS products; 