package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"time"

	"github.com/kauakfm/ai-banking-assistant/internal/client"
	"github.com/kauakfm/ai-banking-assistant/internal/domain"
	"github.com/kauakfm/ai-banking-assistant/internal/handler"
	"github.com/kauakfm/ai-banking-assistant/pkg/cache"
	"github.com/kauakfm/ai-banking-assistant/pkg/middleware"
	"github.com/kauakfm/ai-banking-assistant/pkg/resilience"
	"github.com/sony/gobreaker"
)

var (
	sharedMetrics     *middleware.Metrics
	sharedMetricsOnce sync.Once
)

func testMetrics() *middleware.Metrics {
	sharedMetricsOnce.Do(func() {
		sharedMetrics = middleware.NewMetrics()
	})
	return sharedMetrics
}

func newTestCB() *gobreaker.CircuitBreaker {
	return resilience.NewCircuitBreaker(resilience.CBConfig{
		Name:             "test",
		MaxRequests:      5,
		FailureThreshold: 5,
		Timeout:          10 * time.Second,
	})
}

func newTestCache() *cache.Cache {
	return cache.New(5*time.Minute, 10*time.Minute)
}

func startMockAgent(status int, resp *domain.AgentResponse) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if resp != nil {
			json.NewEncoder(w).Encode(resp)
		}
	}))
}

func postAssistant(h http.Handler, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/v1/assistant", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	mux := http.NewServeMux()
	mux.Handle("POST /v1/assistant", h)
	mux.ServeHTTP(w, req)
	return w
}

func TestAssistant_Success(t *testing.T) {
	agentSrv := startMockAgent(http.StatusOK, &domain.AgentResponse{
		CustomerID: "c1",
		Query:      "balance",
		Response:   "Your balance is $100",
	})
	defer agentSrv.Close()

	agentClient := client.NewAgentClient(agentSrv.URL, 5*time.Second, newTestCB(), nil)
	h := handler.NewAssistantHandler(agentClient, newTestCache(), testMetrics(), nil)

	w := postAssistant(h, `{"customer_id":"c1","prompt":"balance"}`)

	if w.Code != http.StatusOK {
		t.Fatalf("esperado 200, obteve %d: %s", w.Code, w.Body.String())
	}

	var resp domain.AssistantResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Response != "Your balance is $100" {
		t.Errorf("esperado 'Your balance is $100', obteve '%s'", resp.Response)
	}
	if resp.CustomerID != "c1" {
		t.Errorf("esperado customer_id 'c1', obteve '%s'", resp.CustomerID)
	}
	if resp.Cached {
		t.Error("primeira requisição não deveria estar em cache")
	}
}

func TestAssistant_CacheHit(t *testing.T) {
	agentSrv := startMockAgent(http.StatusOK, &domain.AgentResponse{
		Response: "cached response",
	})
	defer agentSrv.Close()

	agentClient := client.NewAgentClient(agentSrv.URL, 5*time.Second, newTestCB(), nil)
	c := newTestCache()
	metrics := testMetrics()
	h := handler.NewAssistantHandler(agentClient, c, metrics, nil)

	postAssistant(h, `{"customer_id":"c1","prompt":"balance"}`)

	w := postAssistant(h, `{"customer_id":"c1","prompt":"balance"}`)

	var resp domain.AssistantResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if !resp.Cached {
		t.Error("segunda requisição deveria ser cache hit")
	}
}

func TestAssistant_EmptyPrompt(t *testing.T) {
	h := handler.NewAssistantHandler(nil, newTestCache(), testMetrics(), nil)
	w := postAssistant(h, `{"customer_id":"c1","prompt":""}`)

	if w.Code != http.StatusBadRequest {
		t.Errorf("esperado 400, obteve %d", w.Code)
	}
}

func TestAssistant_InvalidBody(t *testing.T) {
	h := handler.NewAssistantHandler(nil, newTestCache(), testMetrics(), nil)
	w := postAssistant(h, `not json`)

	if w.Code != http.StatusBadRequest {
		t.Errorf("esperado 400, obteve %d", w.Code)
	}
}

func TestAssistant_AgentError(t *testing.T) {
	agentSrv := startMockAgent(http.StatusInternalServerError, nil)
	defer agentSrv.Close()

	agentClient := client.NewAgentClient(agentSrv.URL, 5*time.Second, newTestCB(), nil)
	h := handler.NewAssistantHandler(agentClient, newTestCache(), testMetrics(), nil)

	w := postAssistant(h, `{"customer_id":"c1","prompt":"test"}`)

	if w.Code != http.StatusBadGateway {
		t.Errorf("esperado 502, obteve %d", w.Code)
	}
}
