package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const maxRequestBytes = 3 << 20

type API struct {
	service *Service
	static  fs.FS
}

func NewHandler(service *Service, static fs.FS) http.Handler {
	api := &API{service: service, static: static}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/algorithms", api.handleAlgorithms)
	mux.HandleFunc("GET /api/fixtures", api.handleFixtureList)
	mux.HandleFunc("GET /api/fixtures/", api.handleFixture)
	mux.HandleFunc("POST /api/merge", api.handleMerge)
	mux.HandleFunc("/", api.handleStatic)
	return mux
}

func (a *API) handleAlgorithms(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"items": Algorithms()})
}

func (a *API) handleFixtureList(w http.ResponseWriter, request *http.Request) {
	page, err := positiveInt(request.URL.Query().Get("page"), 1, 1_000_000)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "request", "invalid page")
		return
	}
	pageSize, err := positiveInt(request.URL.Query().Get("page_size"), 50, 100)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "request", "invalid page_size")
		return
	}
	writeJSON(w, http.StatusOK, a.service.ListFixtures(request.URL.Query().Get("q"), request.URL.Query().Get("class"), page, pageSize))
}

func (a *API) handleFixture(w http.ResponseWriter, request *http.Request) {
	rawID := strings.TrimPrefix(request.URL.EscapedPath(), "/api/fixtures/")
	id, err := url.PathUnescape(rawID)
	if err != nil || id == "" {
		writeAPIError(w, http.StatusBadRequest, "request", "invalid fixture id")
		return
	}
	fixture, ok := a.service.Fixture(id)
	if !ok {
		writeAPIError(w, http.StatusNotFound, "request", "fixture not found")
		return
	}
	writeJSON(w, http.StatusOK, fixture)
}

func (a *API) handleMerge(w http.ResponseWriter, request *http.Request) {
	request.Body = http.MaxBytesReader(w, request.Body, maxRequestBytes)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	var payload struct {
		Algorithm    string  `json:"algorithm"`
		PreviousBase string  `json:"previous_base"`
		UserOverride string  `json:"user_override"`
		TargetBase   string  `json:"target_base"`
		Expected     *string `json:"expected"`
	}
	if err := decoder.Decode(&payload); err != nil {
		status := http.StatusBadRequest
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			status = http.StatusRequestEntityTooLarge
		}
		writeAPIError(w, status, "request", fmt.Sprintf("invalid JSON body: %v", err))
		return
	}
	if err := ensureJSONEOF(decoder); err != nil {
		writeAPIError(w, http.StatusBadRequest, "request", err.Error())
		return
	}
	mergeRequest := MergeRequest{
		Algorithm: payload.Algorithm, PreviousBase: payload.PreviousBase,
		UserOverride: payload.UserOverride, TargetBase: payload.TargetBase,
	}
	if payload.Expected != nil {
		mergeRequest.Expected = *payload.Expected
		mergeRequest.HasExpected = true
	}
	result, err := a.service.Merge(mergeRequest)
	if err != nil {
		var requestErr *RequestError
		if errors.As(err, &requestErr) {
			writeAPIError(w, http.StatusBadRequest, requestErr.Stage, requestErr.Err.Error())
			return
		}
		writeAPIError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *API) handleStatic(w http.ResponseWriter, request *http.Request) {
	if a.static == nil {
		http.NotFound(w, request)
		return
	}
	path := strings.TrimPrefix(request.URL.Path, "/")
	if path == "" {
		path = "index.html"
	}
	content, err := fs.ReadFile(a.static, path)
	if err != nil {
		content, err = fs.ReadFile(a.static, "index.html")
	}
	if err != nil {
		http.NotFound(w, request)
		return
	}
	if strings.HasSuffix(path, ".js") {
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	} else if strings.HasSuffix(path, ".css") {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
	} else if path == "index.html" || !strings.Contains(path, ".") {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(content)
}

func positiveInt(raw string, fallback, maximum int) (int, error) {
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 || value > maximum {
		return 0, errors.New("out of range")
	}
	return value, nil
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return errors.New("request body must contain one JSON object")
	}
	return nil
}

func writeAPIError(w http.ResponseWriter, status int, stage, message string) {
	writeJSONStatus(w, status, map[string]any{"error": map[string]string{"stage": stage, "message": message}})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	writeJSONStatus(w, status, value)
}

func writeJSONStatus(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
