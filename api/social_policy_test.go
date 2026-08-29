package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/larslarsen/bb-go/core"
)

func TestSocialOnlyAllowedEndpoint(t *testing.T) {
	tests := []struct {
		name    string
		method  string
		path    string
		allowed bool
	}{
		{"profile read", http.MethodGet, "/ob/profile/QmPeer", true},
		{"post publish", http.MethodPost, "/ob/post", true},
		{"follow", http.MethodPost, "/ob/follow", true},
		{"chat", http.MethodPost, "/ob/chat", true},
		{"wallet tip", http.MethodPost, "/wallet/spend", true},
		{"lookalike route", http.MethodGet, "/ob/profile-marketplace", false},
		{"listing read", http.MethodGet, "/ob/listings", false},
		{"listing write", http.MethodPost, "/ob/listing", false},
		{"purchase", http.MethodPost, "/ob/purchase", false},
		{"dispute", http.MethodPost, "/ob/opendispute", false},
		{"ratings", http.MethodGet, "/ob/ratings", false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := socialOnlyAllowedEndpoint(test.path, test.method); got != test.allowed {
				t.Fatalf("socialOnlyAllowedEndpoint(%q, %q) = %t, want %t", test.path, test.method, got, test.allowed)
			}
		})
	}
}

func TestSocialOnlyHandlerRejectsMarketplaceEndpoint(t *testing.T) {
	handler := &jsonAPIHandler{
		config: JSONAPIConfig{Enabled: true},
		node: &core.OpenBazaarNode{
			RuntimeMode: core.RuntimeModeSocial,
		},
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/ob/listing", nil)

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
}
