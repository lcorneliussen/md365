package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lcorneliussen/md365/internal/config"
)

func TestEndpointsFor(t *testing.T) {
	cfg := &config.Config{Accounts: map[string]*config.Account{
		"default": {},
		"tenant":  {Tenant: "contoso.onmicrosoft.com"},
		"bad":     {Tenant: "../other"},
	}}

	defaults, err := endpointsFor(cfg, "default")
	if err != nil {
		t.Fatal(err)
	}
	if defaults.authorize != "https://login.microsoftonline.com/common/oauth2/v2.0/authorize" {
		t.Fatalf("default authorize endpoint = %q", defaults.authorize)
	}

	tenant, err := endpointsFor(cfg, "tenant")
	if err != nil {
		t.Fatal(err)
	}
	if tenant.token != "https://login.microsoftonline.com/contoso.onmicrosoft.com/oauth2/v2.0/token" {
		t.Fatalf("tenant token endpoint = %q", tenant.token)
	}

	if _, err := endpointsFor(cfg, "bad"); err == nil {
		t.Fatal("expected invalid tenant error")
	}
}

func TestGenerateState(t *testing.T) {
	first, err := generateState()
	if err != nil {
		t.Fatal(err)
	}
	second, err := generateState()
	if err != nil {
		t.Fatal(err)
	}
	if len(first) < 43 {
		t.Fatalf("state length = %d, want at least 43", len(first))
	}
	if first == second {
		t.Fatal("consecutive OAuth state values matched")
	}
}

func TestAuthCodeCallbackHandler(t *testing.T) {
	tests := []struct {
		name       string
		target     string
		wantStatus int
		wantCode   string
		wantError  string
	}{
		{name: "valid", target: "/?state=expected&code=auth-code", wantStatus: http.StatusOK, wantCode: "auth-code"},
		{name: "missing state", target: "/?code=auth-code", wantStatus: http.StatusBadRequest},
		{name: "mismatched state", target: "/?state=attacker&code=auth-code", wantStatus: http.StatusBadRequest},
		{name: "provider error", target: "/?state=expected&error=access_denied&error_description=cancelled", wantStatus: http.StatusBadRequest, wantError: "authorization error: access_denied - cancelled"},
		{name: "missing code", target: "/?state=expected", wantStatus: http.StatusBadRequest, wantError: "no authorization code received"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resultCh := make(chan string, 1)
			errorCh := make(chan error, 1)
			recorder := httptest.NewRecorder()
			authCodeCallbackHandler("expected", resultCh, errorCh).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, tt.target, nil))

			if recorder.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", recorder.Code, tt.wantStatus)
			}
			select {
			case code := <-resultCh:
				if code != tt.wantCode {
					t.Fatalf("code = %q, want %q", code, tt.wantCode)
				}
			default:
				if tt.wantCode != "" {
					t.Fatalf("missing code %q", tt.wantCode)
				}
			}
			select {
			case err := <-errorCh:
				if tt.wantError == "" || !strings.Contains(err.Error(), tt.wantError) {
					t.Fatalf("error = %q, want %q", err, tt.wantError)
				}
			default:
				if tt.wantError != "" {
					t.Fatalf("missing error %q", tt.wantError)
				}
			}
		})
	}
}

func TestAuthCodeCallbackHandlerOnlyCompletesOnce(t *testing.T) {
	resultCh := make(chan string, 2)
	errorCh := make(chan error, 2)
	handler := authCodeCallbackHandler("expected", resultCh, errorCh)

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/?state=expected&code=first", nil))
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/?state=expected&code=second", nil))

	if code := <-resultCh; code != "first" {
		t.Fatalf("code = %q, want first", code)
	}
	select {
	case code := <-resultCh:
		t.Fatalf("unexpected second completion: %q", code)
	default:
	}
}
