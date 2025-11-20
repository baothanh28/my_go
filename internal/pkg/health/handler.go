package health

import (
	"encoding/json"
	"net/http"
)

// HTTPHandler returns an HTTP handler for health checks
func HTTPHandler(service *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		response := service.GetHealthResponse(r.Context())

		// Set appropriate status code
		statusCode := http.StatusOK
		if response.Status == StatusDown {
			statusCode = http.StatusServiceUnavailable
		} else if response.Status == StatusDegraded {
			// You can choose to return 200 or 503 for degraded
			// Some teams prefer 200 with degraded info in body
			statusCode = http.StatusOK
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		json.NewEncoder(w).Encode(response)
	}
}

// ReadinessHandler returns a readiness probe handler
// This is useful for Kubernetes readiness probes
func ReadinessHandler(service *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		response := service.GetHealthResponse(r.Context())

		// For readiness, we're strict: only UP is ready
		if response.Status == StatusUp {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("NOT READY"))
		}
	}
}

// LivenessHandler returns a liveness probe handler
// This should only check if the application is alive, not dependencies
func LivenessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Simple alive check - if we can respond, we're alive
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}
}

// DetailedHealthHandler returns a handler with detailed health information
func DetailedHealthHandler(service *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Get health response
		response := service.GetHealthResponse(ctx)

		// Add additional runtime information
		if response.Details == nil {
			response.Details = make(map[string]interface{})
		}

		// You can add more details here
		// For example: version, uptime, etc.

		// Set status code
		statusCode := http.StatusOK
		if response.Status == StatusDown {
			statusCode = http.StatusServiceUnavailable
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		json.NewEncoder(w).Encode(response)
	}
}
