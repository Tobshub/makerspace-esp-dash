package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/Tobshub/makerspace-esp-dash/backend/internal/config"
)

func TestReadyReportsDependencies(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := New(Deps{
		Config:       config.Config{AppURL: "http://localhost:5173"},
		DatabasePing: func(context.Context) error { return errors.New("down") },
		MQTTUp:       func() bool { return false },
	})
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	router.ServeHTTP(res, req)
	if res.Code != http.StatusServiceUnavailable {
		t.Fatalf("status %d", res.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["database"] != "down" || body["mqtt"] != "down" || body["status"] != "degraded" {
		t.Fatalf("body %s", res.Body.String())
	}

	res = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/health", nil)
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("health status %d", res.Code)
	}
}

func TestReadyOK(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := New(Deps{
		Config:       config.Config{AppURL: "http://localhost:5173"},
		DatabasePing: func(context.Context) error { return nil },
		MQTTUp:       func() bool { return true },
		BrokerHost:   "localhost",
		BrokerPort:   1883,
	})
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("status %d body %s", res.Code, res.Body.String())
	}
}
