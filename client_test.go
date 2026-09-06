package prometheus

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNew_requiresHTTPClient(t *testing.T) {
	t.Parallel()

	client, err := New("http://localhost:9090", nil)
	if err == nil {
		t.Fatal("expected error for nil http client")
	}
	if client != nil {
		t.Fatal("expected nil client when http client is missing")
	}
	if !strings.Contains(err.Error(), "http client is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestQueryAndTestConnection_againstFixtureHTTP(t *testing.T) {
	t.Parallel()

	var sawQueryRange, sawHealthy bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/api/v1/query_range"):
			sawQueryRange = true
			if got := r.URL.Query().Get("query"); got != "up" && r.Method == http.MethodGet {
				body, _ := io.ReadAll(r.Body)
				if !strings.Contains(string(body), "query=up") && got != "up" {
					t.Errorf("query_range missing query=up, path=%s qs=%s body=%s", r.URL.Path, r.URL.RawQuery, body)
				}
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"matrix","result":[{"metric":{"__name__":"up","job":"prometheus"},"values":[[1600000000,"1"]]}]}}`))
		case r.URL.Path == "/-/healthy":
			sawHealthy = true
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("OK"))
		case strings.HasSuffix(r.URL.Path, "/api/v1/query"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[]}}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)

	client, err := New(srv.URL, srv.Client())
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	start := time.Unix(1600000000, 0).Add(-time.Hour)
	end := time.Unix(1600000000, 0)
	result, err := client.Query(ctx, "up", start, end, time.Minute, 0)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if result.Status != "success" {
		t.Fatalf("Query status=%q error=%q", result.Status, result.Error)
	}
	if result.ResultType != "metrics" {
		t.Fatalf("ResultType=%q, want metrics", result.ResultType)
	}
	if result.Data == nil || len(result.Data.Result) != 1 {
		t.Fatalf("expected 1 series, got %+v", result.Data)
	}
	if result.Data.Result[0].Metric["job"] != "prometheus" {
		t.Fatalf("metric labels=%v", result.Data.Result[0].Metric)
	}
	if !sawQueryRange {
		t.Fatal("expected fixture to receive /api/v1/query_range")
	}

	if err := client.TestConnection(ctx); err != nil {
		t.Fatalf("TestConnection: %v", err)
	}
	if !sawHealthy {
		t.Fatal("expected TestConnection to hit /-/healthy")
	}
}

func TestTestConnection_usesHealthyThenQueryFallback(t *testing.T) {
	t.Parallel()

	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		if r.URL.Path == "/-/healthy" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if strings.Contains(r.URL.Path, "/api/v1/query") {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"scalar","result":[1,"1"]}}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)

	client, err := New(srv.URL, srv.Client())
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.TestConnection(ctx); err != nil {
		t.Fatalf("TestConnection: %v", err)
	}
	if len(paths) < 2 || paths[0] != "/-/healthy" {
		t.Fatalf("paths=%v, want /-/healthy then query", paths)
	}
}

func TestQuery_fixturePOSTBody(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "query_range") {
			http.NotFound(w, r)
			return
		}
		query := r.URL.Query().Get("query")
		if query == "" && r.Method == http.MethodPost {
			_ = r.ParseForm()
			query = r.Form.Get("query")
		}
		if query != `sum(rate(http_requests_total[5m]))` {
			t.Errorf("query=%q", query)
		}
		w.Header().Set("Content-Type", "application/json")
		enc := json.NewEncoder(w)
		_ = enc.Encode(map[string]any{
			"status": "success",
			"data": map[string]any{
				"resultType": "matrix",
				"result":     []any{},
			},
		})
	}))
	t.Cleanup(srv.Close)

	client, err := New(srv.URL, srv.Client())
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := client.Query(ctx, `sum(rate(http_requests_total[5m]))`, time.Now().Add(-time.Hour), time.Now(), 15*time.Second, 0)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if result.Status != "success" {
		t.Fatalf("status=%q error=%q", result.Status, result.Error)
	}
}
