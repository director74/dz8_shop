-- Отменяем тестовые данные для service_order

-- Удаляем тестовые заказы
DELETE FROM orders WHERE user_id IN (1, 2, 3, 4, 5);

-- Удаляем тестовых пользователей
DELETE FROM users WHERE id IN (1, 2, 3, 4, 5); 