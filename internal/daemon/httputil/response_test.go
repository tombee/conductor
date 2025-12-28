package httputil

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteJSON(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		data       any
		wantStatus int
		wantJSON   string
	}{
		{
			name:       "success with map",
			status:     http.StatusOK,
			data:       map[string]string{"message": "success"},
			wantStatus: http.StatusOK,
			wantJSON:   `{"message":"success"}`,
		},
		{
			name:       "success with struct",
			status:     http.StatusCreated,
			data:       struct{ ID int }{ID: 42},
			wantStatus: http.StatusCreated,
			wantJSON:   `{"ID":42}`,
		},
		{
			name:       "error status code",
			status:     http.StatusInternalServerError,
			data:       map[string]string{"error": "something went wrong"},
			wantStatus: http.StatusInternalServerError,
			wantJSON:   `{"error":"something went wrong"}`,
		},
		{
			name:       "empty object",
			status:     http.StatusNoContent,
			data:       map[string]string{},
			wantStatus: http.StatusNoContent,
			wantJSON:   `{}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			WriteJSON(w, tt.status, tt.data)

			if w.Code != tt.wantStatus {
				t.Errorf("WriteJSON() status = %v, want %v", w.Code, tt.wantStatus)
			}

			contentType := w.Header().Get("Content-Type")
			if contentType != "application/json" {
				t.Errorf("WriteJSON() Content-Type = %v, want application/json", contentType)
			}

			var got, want map[string]any
			if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
				t.Fatalf("Failed to unmarshal response: %v", err)
			}
			if err := json.Unmarshal([]byte(tt.wantJSON), &want); err != nil {
				t.Fatalf("Failed to unmarshal expected JSON: %v", err)
			}

			if len(got) != len(want) {
				t.Errorf("WriteJSON() response length = %d, want %d", len(got), len(want))
			}

			for k, v := range want {
				if got[k] != v {
					t.Errorf("WriteJSON() response[%s] = %v, want %v", k, got[k], v)
				}
			}
		})
	}
}

func TestWriteError(t *testing.T) {
	tests := []struct {
		name        string
		status      int
		message     string
		wantStatus  int
		wantMessage string
	}{
		{
			name:        "not found error",
			status:      http.StatusNotFound,
			message:     "resource not found",
			wantStatus:  http.StatusNotFound,
			wantMessage: "resource not found",
		},
		{
			name:        "bad request error",
			status:      http.StatusBadRequest,
			message:     "invalid input",
			wantStatus:  http.StatusBadRequest,
			wantMessage: "invalid input",
		},
		{
			name:        "internal server error",
			status:      http.StatusInternalServerError,
			message:     "internal error",
			wantStatus:  http.StatusInternalServerError,
			wantMessage: "internal error",
		},
		{
			name:        "empty message",
			status:      http.StatusBadRequest,
			message:     "",
			wantStatus:  http.StatusBadRequest,
			wantMessage: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			WriteError(w, tt.status, tt.message)

			if w.Code != tt.wantStatus {
				t.Errorf("WriteError() status = %v, want %v", w.Code, tt.wantStatus)
			}

			contentType := w.Header().Get("Content-Type")
			if contentType != "application/json" {
				t.Errorf("WriteError() Content-Type = %v, want application/json", contentType)
			}

			var response map[string]string
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Fatalf("Failed to unmarshal response: %v", err)
			}

			if response["error"] != tt.wantMessage {
				t.Errorf("WriteError() error message = %v, want %v", response["error"], tt.wantMessage)
			}
		})
	}
}
