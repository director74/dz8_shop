package webapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// BillingClient представляет HTTP клиент для работы с сервисом биллинга
type BillingClient struct {
	baseURL    string
	httpClient *http.Client
}

func NewBillingClient(baseURL string) *BillingClient {
	return &BillingClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *BillingClient) CreateAccount(ctx context.Context, userID uint) error {
	url := fmt.Sprintf("%s/api/v1/accounts", c.baseURL)

	reqBody := map[string]interface{}{
		"user_id": userID,
	}

	reqBodyJSON, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("ошибка при маршалинге запроса: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(reqBodyJSON))
	if err != nil {
		return fmt.Errorf("ошибка при создании запроса: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("ошибка при выполнении запроса: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("неуспешный ответ от сервиса биллинга: %s", resp.Status)
	}

	return nil
}

// WithdrawMoney снимает деньги с аккаунта в сервисе биллинга
func (c *BillingClient) WithdrawMoney(ctx context.Context, userID uint, amount float64, email string, token string) (bool, error) {
	url := fmt.Sprintf("%s/api/v1/billing/withdraw", c.baseURL)

	reqBody := map[string]interface{}{
		"user_id": userID,
		"amount":  amount,
		"email":   email,
	}

	reqBodyJSON, err := json.Marshal(reqBody)
	if err != nil {
		return false, fmt.Errorf("ошибка при маршалинге запроса: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(reqBodyJSON))
	if err != nil {
		return false, fmt.Errorf("ошибка при создании запроса: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Добавляем JWT токен в заголовок авторизации
	if token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("ошибка при выполнении запроса: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusBadRequest {
		// Недостаточно средств
		return false, nil
	}

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("неуспешный ответ от сервиса биллинга: %s", resp.Status)
	}

	var response struct {
		Success bool `json:"success"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return false, fmt.Errorf("ошибка при декодировании ответа: %w", err)
	}

	return response.Success, nil
}
