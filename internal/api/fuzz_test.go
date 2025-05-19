package api

import (
	"bytes"
	"encoding/json"
	"github.com/grannnsacker/job-finder-back/pkg/utils"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"testing"
)

// FuzzValidateEmail tests email validation with random inputs
func FuzzValidateEmail(f *testing.F) {
	// Add seed corpus
	f.Add("test@example.com")
	f.Add("invalid-email")
	f.Add("")
	f.Add("very.long.email.address@very.long.domain.com")
	f.Add("special!chars@domain.com")
	f.Add("multiple@dots@domain.com")
	f.Add("no@domain")
	f.Add("no.at.sign")
	f.Add("just@")
	f.Add("@just")

	f.Fuzz(func(t *testing.T, email string) {
		// Create test server
		server := newTestServer(t, nil, nil)
		recorder := httptest.NewRecorder()

		// Create request body
		body := struct {
			Email string `json:"email"`
		}{
			Email: email,
		}

		jsonData, err := json.Marshal(body)
		require.NoError(t, err)

		// Create request
		request, err := http.NewRequest(http.MethodPost, BaseUrl+"/users/verify-email", bytes.NewBuffer(jsonData))
		require.NoError(t, err)
		request.Header.Set("Content-Type", "application/json")

		// Execute request
		server.router.ServeHTTP(recorder, request)

		// Verify response
		require.NotEqual(t, http.StatusInternalServerError, recorder.Code)
	})
}

// FuzzValidatePassword tests password validation with random inputs
func FuzzValidatePassword(f *testing.F) {
	// Add seed corpus
	f.Add("validPassword123!")
	f.Add("short")
	f.Add("")
	f.Add("no-numbers!")
	f.Add("no-special-chars123")
	f.Add("no-uppercase123!")
	f.Add("no-lowercase123!")
	f.Add(utils.RandomString(100))
	f.Add("!@#$%^&*()")
	f.Add("1234567890")

	f.Fuzz(func(t *testing.T, password string) {
		// Create test server
		server := newTestServer(t, nil, nil)
		recorder := httptest.NewRecorder()

		// Create request body
		body := struct {
			Password string `json:"password"`
		}{
			Password: password,
		}

		jsonData, err := json.Marshal(body)
		require.NoError(t, err)

		// Create request
		request, err := http.NewRequest(http.MethodPost, BaseUrl+"/users/change-password", bytes.NewBuffer(jsonData))
		require.NoError(t, err)
		request.Header.Set("Content-Type", "application/json")

		// Execute request
		server.router.ServeHTTP(recorder, request)

		// Verify response
		require.NotEqual(t, http.StatusInternalServerError, recorder.Code)
	})
}

// FuzzValidateJobTitle tests job title validation with random inputs
func FuzzValidateJobTitle(f *testing.F) {
	// Add seed corpus
	f.Add("Software Engineer")
	f.Add("")
	f.Add(utils.RandomString(200))
	f.Add("!@#$%^&*()")
	f.Add("1234567890")
	f.Add("Title with\nnewline")
	f.Add("Title with\ttab")
	f.Add("Title with spaces    ")
	f.Add("    Title with leading spaces")

	f.Fuzz(func(t *testing.T, title string) {
		// Create test server
		server := newTestServer(t, nil, nil)
		recorder := httptest.NewRecorder()

		// Create request body
		body := struct {
			Title string `json:"title"`
		}{
			Title: title,
		}

		jsonData, err := json.Marshal(body)
		require.NoError(t, err)

		// Create request
		request, err := http.NewRequest(http.MethodPost, BaseUrl+"/jobs/validate-title", bytes.NewBuffer(jsonData))
		require.NoError(t, err)
		request.Header.Set("Content-Type", "application/json")

		// Execute request
		server.router.ServeHTTP(recorder, request)

		// Verify response
		require.NotEqual(t, http.StatusInternalServerError, recorder.Code)
	})
}

// FuzzValidateSalary tests salary validation with random inputs
func FuzzValidateSalary(f *testing.F) {
	// Add seed corpus
	f.Add("50000")
	f.Add("100000")
	f.Add("")
	f.Add("not-a-number")
	f.Add("-1000")
	f.Add("0")
	f.Add("999999999")
	f.Add("50,000")
	f.Add("50.000")
	f.Add("50 000")

	f.Fuzz(func(t *testing.T, salary string) {
		// Create test server
		server := newTestServer(t, nil, nil)
		recorder := httptest.NewRecorder()

		// Create request body
		body := struct {
			Salary string `json:"salary"`
		}{
			Salary: salary,
		}

		jsonData, err := json.Marshal(body)
		require.NoError(t, err)

		// Create request
		request, err := http.NewRequest(http.MethodPost, BaseUrl+"/jobs/validate-salary", bytes.NewBuffer(jsonData))
		require.NoError(t, err)
		request.Header.Set("Content-Type", "application/json")

		// Execute request
		server.router.ServeHTTP(recorder, request)

		// Verify response
		require.NotEqual(t, http.StatusInternalServerError, recorder.Code)
	})
}

// FuzzValidateLocation tests location validation with random inputs
func FuzzValidateLocation(f *testing.F) {
	// Add seed corpus
	f.Add("New York")
	f.Add("")
	f.Add(utils.RandomString(100))
	f.Add("!@#$%^&*()")
	f.Add("1234567890")
	f.Add("Location with\nnewline")
	f.Add("Location with\ttab")
	f.Add("Location with spaces    ")
	f.Add("    Location with leading spaces")

	f.Fuzz(func(t *testing.T, location string) {
		// Create test server
		server := newTestServer(t, nil, nil)
		recorder := httptest.NewRecorder()

		// Create request body
		body := struct {
			Location string `json:"location"`
		}{
			Location: location,
		}

		jsonData, err := json.Marshal(body)
		require.NoError(t, err)

		// Create request
		request, err := http.NewRequest(http.MethodPost, BaseUrl+"/jobs/validate-location", bytes.NewBuffer(jsonData))
		require.NoError(t, err)
		request.Header.Set("Content-Type", "application/json")

		// Execute request
		server.router.ServeHTTP(recorder, request)

		// Verify response
		require.NotEqual(t, http.StatusInternalServerError, recorder.Code)
	})
}
