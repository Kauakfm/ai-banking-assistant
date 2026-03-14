package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/kauakfm/ai-banking-assistant/internal/client"
	"github.com/kauakfm/ai-banking-assistant/internal/domain"
	"github.com/kauakfm/ai-banking-assistant/internal/handler"
	"github.com/kauakfm/ai-banking-assistant/pkg/resilience"
)

func getCustomerEndpoint(h http.Handler, pattern, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	w := httptest.NewRecorder()

	mux := http.NewServeMux()
	mux.Handle("GET "+pattern, h)
	mux.ServeHTTP(w, req)
	return w
}

func TestProfile_MockFallback(t *testing.T) {
	// Sem URL base → retorna dados mock
	profileClient := client.NewProfileClient("", 5*time.Second, newTestCB(),
		resilience.NewRetrier(resilience.RetryConfig{MaxAttempts: 1, BaseDelay: time.Millisecond, MaxDelay: time.Millisecond}), nil)
	h := handler.NewProfileHandler(profileClient, nil)

	w := getCustomerEndpoint(h, "/v1/customers/{customerId}/profile", "/v1/customers/c1/profile")

	if w.Code != http.StatusOK {
		t.Fatalf("esperado 200, obteve %d: %s", w.Code, w.Body.String())
	}

	var profile domain.Profile
	json.NewDecoder(w.Body).Decode(&profile)
	if profile.ID != "c1" {
		t.Errorf("esperado ID 'c1', obteve '%s'", profile.ID)
	}
	if profile.Segment != "premium" {
		t.Errorf("esperado segmento 'premium', obteve '%s'", profile.Segment)
	}
}

func TestProfile_RealAPI_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(domain.Profile{
			ID:      "c1",
			Name:    "John Doe",
			Segment: "private",
		})
	}))
	defer srv.Close()

	profileClient := client.NewProfileClient(srv.URL, 5*time.Second, newTestCB(),
		resilience.NewRetrier(resilience.RetryConfig{MaxAttempts: 1, BaseDelay: time.Millisecond, MaxDelay: time.Millisecond}), nil)
	h := handler.NewProfileHandler(profileClient, nil)

	w := getCustomerEndpoint(h, "/v1/customers/{customerId}/profile", "/v1/customers/c1/profile")

	var profile domain.Profile
	json.NewDecoder(w.Body).Decode(&profile)
	if profile.Name != "John Doe" {
		t.Errorf("esperado 'John Doe', obteve '%s'", profile.Name)
	}
}

func TestTransaction_MockFallback(t *testing.T) {
	txnClient := client.NewTransactionClient("", 5*time.Second, newTestCB(),
		resilience.NewRetrier(resilience.RetryConfig{MaxAttempts: 1, BaseDelay: time.Millisecond, MaxDelay: time.Millisecond}), nil)
	h := handler.NewTransactionHandler(txnClient, nil)

	w := getCustomerEndpoint(h, "/v1/customers/{customerId}/transactions", "/v1/customers/c1/transactions")

	if w.Code != http.StatusOK {
		t.Fatalf("esperado 200, obteve %d", w.Code)
	}

	var txns []domain.Transaction
	json.NewDecoder(w.Body).Decode(&txns)
	if len(txns) != 5 {
		t.Errorf("esperado 5 transações mock, obteve %d", len(txns))
	}
}

func TestTransaction_RealAPI_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]domain.Transaction{
			{ID: "t1", Amount: -100, Type: "debit"},
		})
	}))
	defer srv.Close()

	txnClient := client.NewTransactionClient(srv.URL, 5*time.Second, newTestCB(),
		resilience.NewRetrier(resilience.RetryConfig{MaxAttempts: 1, BaseDelay: time.Millisecond, MaxDelay: time.Millisecond}), nil)
	h := handler.NewTransactionHandler(txnClient, nil)

	w := getCustomerEndpoint(h, "/v1/customers/{customerId}/transactions", "/v1/customers/c1/transactions")

	var txns []domain.Transaction
	json.NewDecoder(w.Body).Decode(&txns)
	if len(txns) != 1 || txns[0].ID != "t1" {
		t.Errorf("transações inesperadas: %v", txns)
	}
}
