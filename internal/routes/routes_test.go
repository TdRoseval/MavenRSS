package routes

import (
	"net/http"
	"testing"

	"MavenRSS/internal/api/core"
)

func TestRegisterAPIRoutes_DoesNotPanic(t *testing.T) {
	mux := http.NewServeMux()

	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("RegisterAPIRoutes panicked: %v", recovered)
		}
	}()

	RegisterAPIRoutes(mux, &core.Handler{})
}
