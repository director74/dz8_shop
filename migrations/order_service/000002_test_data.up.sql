-- Тестовые данные для service_order

-- Добавление тестовых пользователей, соответствующих аккаунтам в billing_service
INSERT INTO users (id, username, email, password, created_at) VALUES
(1, 'user1', 'user1@example.com', '$2a$10$hIqGDp2M4MzVvz1YfpzbS.9xUUiVRGm8YxjvJarVdCd6ATk3wbh7m', NOW());

-- Добавление тестовых заказов
INSERT INTO orders (user_id, amount, status, created_at) VALUES
(1, 4999.00, 'pending', NOW() - INTERVAL '30 day');

-- Сбрасываем последовательность id, чтобы новые записи создавались после существующих
SELECT setval('users_id_seq', (SELECT MAX(id) FROM users));
SELECT setval('orders_id_seq', (SELECT MAX(id) FROM orders)); 