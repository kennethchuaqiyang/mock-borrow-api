package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestHappyPathGet covers: valid username/location/userid query params,
// valid headers -> 200 OK, correct response headers, and correct metadata/cache body.
func TestHappyPathGet(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/user?username=john&location=Singapore&userid=1", nil)
	req.Header.Set("X-Browser-Type", "Chrome")
	req.Header.Set("X-Admin-Flag", "true")

	rec := httptest.NewRecorder()
	handleGet(rec, req)

	res := rec.Result()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", res.StatusCode)
	}

	if got := res.Header.Get("X-Browser"); got != "Chrome" {
		t.Errorf("expected X-Browser header 'Chrome', got %q", got)
	}
	if got := res.Header.Get("X-Secret-Key"); got == "" {
		t.Errorf("expected non-empty X-Secret-Key header")
	} else {
		want := generateSecretKey(1, "john")
		if got != want {
			t.Errorf("secret key mismatch: got %q, want %q", got, want)
		}
	}

	var body getResponse
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}

	if body.Metadata.Username != "john" {
		t.Errorf("expected metadata username 'john', got %q", body.Metadata.Username)
	}
	if body.Metadata.Location != "Singapore" {
		t.Errorf("expected metadata location 'Singapore', got %q", body.Metadata.Location)
	}
	if body.Metadata.Salary != 5000 {
		t.Errorf("expected metadata salary 5000 (seeded user), got %v", body.Metadata.Salary)
	}

	if body.Cache.Username != "john" {
		t.Errorf("expected cache username 'john', got %q", body.Cache.Username)
	}
	if body.Cache.UserIdentity != 1 {
		t.Errorf("expected cache user_identity 1, got %d", body.Cache.UserIdentity)
	}
	if body.Cache.Salary != 5000 {
		t.Errorf("expected cache salary 5000, got %v", body.Cache.Salary)
	}
}

// TestHappyPathPost covers: a borrow request that keeps the new total <= 200
// -> 200 OK, allowed_to_borrow = true, amount_owed updated correctly.
func TestHappyPathPost(t *testing.T) {
	// Use a fresh user id so this test doesn't depend on state from other tests.
	userID := 101
	reqBody := postRequest{
		Username:       "alice",
		Location:       "Singapore",
		UserID:         userID,
		AmountToBorrow: 50,
	}
	payload, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/borrow", bytes.NewReader(payload))
	req.Header.Set("X-Browser-Type", "Firefox")
	req.Header.Set("X-Admin-Flag", "false")
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	handlePost(rec, req)

	res := rec.Result()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", res.StatusCode)
	}

	if got := res.Header.Get("X-Browser"); got != "Firefox" {
		t.Errorf("expected X-Browser header 'Firefox', got %q", got)
	}
	if got := res.Header.Get("X-Secret-Key"); got == "" {
		t.Errorf("expected non-empty X-Secret-Key header")
	} else {
		want := generateSecretKey(userID, "alice")
		if got != want {
			t.Errorf("secret key mismatch: got %q, want %q", got, want)
		}
	}

	var body postResponse
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}

	if body.Metadata.Username != "alice" {
		t.Errorf("expected username 'alice', got %q", body.Metadata.Username)
	}
	if body.Metadata.Location != "Singapore" {
		t.Errorf("expected location 'Singapore', got %q", body.Metadata.Location)
	}
	if !body.Metadata.AllowedToBorrow {
		t.Errorf("expected allowed_to_borrow=true for a request within limit")
	}
	if body.Metadata.AmountAlteredBy != 50 {
		t.Errorf("expected amount_altered_to_borrow=50, got %v", body.Metadata.AmountAlteredBy)
	}
	if body.Metadata.AmountOwed != 50 {
		t.Errorf("expected amount_owed=50 (new user, 0+50), got %v", body.Metadata.AmountOwed)
	}

	// Confirm the store was actually updated (persisted side effect of a happy-path borrow).
	stored := store.Get(userID, "alice", "Singapore")
	if stored.AmountOwed != 50 {
		t.Errorf("expected store to persist amount_owed=50, got %v", stored.AmountOwed)
	}
}

// TestWrongMethodOnUserEndpoint covers: hitting GET /api/user with any method
// other than GET should be rejected with 405, not silently processed.
func TestWrongMethodOnUserEndpoint(t *testing.T) {
	methods := []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/user?username=john&location=Singapore&userid=1", nil)
			rec := httptest.NewRecorder()
			handleGet(rec, req)

			if rec.Code != http.StatusMethodNotAllowed {
				t.Errorf("expected 405 for %s on /api/user, got %d", method, rec.Code)
			}
		})
	}
}

// TestWrongMethodOnBorrowEndpoint covers: hitting POST /api/borrow with any method
// other than POST should be rejected with 405, not silently processed.
func TestWrongMethodOnBorrowEndpoint(t *testing.T) {
	methods := []string{http.MethodGet, http.MethodPut, http.MethodDelete, http.MethodPatch}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/borrow", nil)
			rec := httptest.NewRecorder()
			handlePost(rec, req)

			if rec.Code != http.StatusMethodNotAllowed {
				t.Errorf("expected 405 for %s on /api/borrow, got %d", method, rec.Code)
			}
		})
	}
}