-- Тестовые данные для доставки

-- Добавление временных слотов доставки (утро, день, вечер на ближайшие 3 дня)
INSERT INTO delivery_slots (start_time, end_time, zone_id, capacity, available, max_deliveries, current_load, is_disabled) VALUES
-- Сегодня
(CURRENT_DATE + '09:00:00'::time, CURRENT_DATE + '12:00:00'::time, 1, 5, 5, 5, 0, false),
(CURRENT_DATE + '13:00:00'::time, CURRENT_DATE + '16:00:00'::time, 1, 5, 5, 5, 0, false),
(CURRENT_DATE + '17:00:00'::time, CURRENT_DATE + '20:00:00'::time, 1, 5, 5, 5, 0, false),
-- Завтра
(CURRENT_DATE + INTERVAL '1 day' + '09:00:00'::time, CURRENT_DATE + INTERVAL '1 day' + '12:00:00'::time, 1, 5, 5, 5, 0, false),
(CURRENT_DATE + INTERVAL '1 day' + '13:00:00'::time, CURRENT_DATE + INTERVAL '1 day' + '16:00:00'::time, 1, 5, 5, 5, 0, false),
(CURRENT_DATE + INTERVAL '1 day' + '17:00:00'::time, CURRENT_DATE + INTERVAL '1 day' + '20:00:00'::time, 1, 5, 5, 5, 0, false),
-- Послезавтра
(CURRENT_DATE + INTERVAL '2 days' + '09:00:00'::time, CURRENT_DATE + INTERVAL '2 days' + '12:00:00'::time, 1, 5, 5, 5, 0, false),
(CURRENT_DATE + INTERVAL '2 days' + '13:00:00'::time, CURRENT_DATE + INTERVAL '2 days' + '16:00:00'::time, 1, 5, 5, 5, 0, false),
(CURRENT_DATE + INTERVAL '2 days' + '17:00:00'::time, CURRENT_DATE + INTERVAL '2 days' + '20:00:00'::time, 1, 5, 5, 5, 0, false);

-- Добавление курьеров
INSERT INTO couriers (name, phone, email, status, current_zone_id, is_active) VALUES
('Иван Петров', '+7 (901) 123-45-67', 'ivan@example.com', 'available', 1, true),
('Мария Сидорова', '+7 (902) 234-56-78', 'maria@example.com', 'available', 1, true),
('Алексей Иванов', '+7 (903) 345-67-89', 'alexey@example.com', 'available', 1, true),
('Елена Смирнова', '+7 (904) 456-78-90', 'elena@example.com', 'available', 1, true),
('Дмитрий Козлов', '+7 (905) 567-89-01', 'dmitry@example.com', 'available', 1, true);

-- Добавление расписания курьеров (все курьеры доступны на все слоты)
INSERT INTO courier_schedules (courier_id, slot_id, start_time, end_time, is_available, is_reserved, is_completed)
SELECT c.id, s.id, s.start_time, s.end_time, true, false, false
FROM couriers c
CROSS JOIN delivery_slots s;

-- Пример заказов доставки (несколько для тестирования)
INSERT INTO delivery (order_id, user_id, courier_id, status, scheduled_start_time, scheduled_end_time, delivery_address, recipient_name, recipient_phone) 
SELECT 
    d.order_id, 
    d.user_id, 
    d.courier_id, 
    d.status, 
    s.start_time as scheduled_start_time, 
    s.end_time as scheduled_end_time, 
    d.delivery_address, 
    d.recipient_name, 
    d.recipient_phone
FROM (
    VALUES
    (1, 1, 1, 'scheduled', 'ул. Ленина, 10, кв. 15', 'Иван Иванов', '+7 (900) 123-45-67', 1)
) as d(order_id, user_id, courier_id, status, delivery_address, recipient_name, recipient_phone, slot_id)
JOIN delivery_slots s ON s.id = d.slot_id; 