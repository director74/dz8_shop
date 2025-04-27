-- Удаление тестовых данных

-- Удаление заказов доставки
DELETE FROM delivery 
WHERE order_id IN (1001, 1002, 1003, 1004, 1005);

-- Удаление расписания курьеров
DELETE FROM courier_schedules;

-- Удаление курьеров
DELETE FROM couriers;

-- Удаление временных слотов
DELETE FROM delivery_slots; 