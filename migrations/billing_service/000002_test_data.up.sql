-- Тестовые данные для биллинга

-- Добавление тестовых аккаунтов
INSERT INTO accounts (user_id, balance) VALUES
(1, 5001.00);

-- Добавление тестовых транзакций
INSERT INTO transactions (account_id, amount, type, status) VALUES
-- Пользователь 1
(1, 10000.00, 'deposit', 'completed'),
(1, -4999.00, 'payment', 'completed');