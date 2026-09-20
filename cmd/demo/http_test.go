package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/require"
)

func TestAPIWorkflow(t *testing.T) {
	service := testService(t)
	handler := NewHandler(service, fstest.MapFS{"index.html": {Data: []byte("demo")}})

	list := httptest.NewRecorder()
	handler.ServeHTTP(list, httptest.NewRequest(http.MethodGet, "/api/fixtures?q=service.image&class=atomic&page_size=2", nil))
	require.Equal(t, http.StatusOK, list.Code)
	var page FixturePage
	require.NoError(t, json.Unmarshal(list.Body.Bytes(), &page))
	require.Len(t, page.Items, 2)

	fixtureResponse := httptest.NewRecorder()
	fixturePath := "/api/fixtures/" + url.PathEscape(testFixtureID)
	handler.ServeHTTP(fixtureResponse, httptest.NewRequest(http.MethodGet, fixturePath, nil))
	require.Equal(t, http.StatusOK, fixtureResponse.Code)
	var fixture FixtureDetail
	require.NoError(t, json.Unmarshal(fixtureResponse.Body.Bytes(), &fixture))

	payload, err := json.Marshal(map[string]any{
		"algorithm": "intent", "previous_base": fixture.PreviousBase,
		"user_override": fixture.UserOverride, "target_base": fixture.TargetBase,
		"expected": fixture.Expected,
	})
	require.NoError(t, err)
	mergeResponse := httptest.NewRecorder()
	handler.ServeHTTP(mergeResponse, httptest.NewRequest(http.MethodPost, "/api/merge", bytes.NewReader(payload)))
	require.Equal(t, http.StatusOK, mergeResponse.Code, mergeResponse.Body.String())
	var result MergeResponse
	require.NoError(t, json.Unmarshal(mergeResponse.Body.Bytes(), &result))
	require.NotEmpty(t, result.NewEffective)
	require.NotNil(t, result.Diagnostics)
}

func TestAPIErrorsAndStaticFallback(t *testing.T) {
	service := testService(t)
	handler := NewHandler(service, fstest.MapFS{"index.html": {Data: []byte("demo")}})

	for _, test := range []struct {
		name   string
		method string
		path   string
		body   io.Reader
		status int
	}{
		{name: "bad page", method: http.MethodGet, path: "/api/fixtures?page=0", status: http.StatusBadRequest},
		{name: "missing fixture", method: http.MethodGet, path: "/api/fixtures/not-a-case", status: http.StatusNotFound},
		{name: "bad json", method: http.MethodPost, path: "/api/merge", body: strings.NewReader("{"), status: http.StatusBadRequest},
		{name: "too large", method: http.MethodPost, path: "/api/merge", body: strings.NewReader(`{"algorithm":"intent","previous_base":"` + strings.Repeat("x", maxRequestBytes) + `"}`), status: http.StatusRequestEntityTooLarge},
		{name: "spa fallback", method: http.MethodGet, path: "/anything", status: http.StatusOK},
	} {
		t.Run(test.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(test.method, test.path, test.body))
			require.Equal(t, test.status, response.Code, response.Body.String())
		})
	}
}
