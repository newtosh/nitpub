package telemetry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRegisterSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req registerRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.InstanceID == "" {
			t.Fatal("expected non-empty instance_id in request")
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(registerResponse{Token: "issued-token"})
	}))
	defer srv.Close()

	id, token, err := Register(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if id == "" {
		t.Fatal("expected non-empty instance id")
	}
	if token != "issued-token" {
		t.Fatalf("token = %q, want issued-token", token)
	}
}

func TestRegisterNon2xxReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	if _, _, err := Register(context.Background(), srv.URL); err == nil {
		t.Fatal("expected error on non-2xx response")
	}
}

func TestRegisterEmptyTokenReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(registerResponse{})
	}))
	defer srv.Close()

	if _, _, err := Register(context.Background(), srv.URL); err == nil {
		t.Fatal("expected error when response carries no token")
	}
}

func TestRegisterNetworkFailureReturnsError(t *testing.T) {
	if _, _, err := Register(context.Background(), "http://127.0.0.1:1"); err == nil {
		t.Fatal("expected error on connection failure")
	}
}
