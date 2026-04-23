package aggregatorserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHealthReturnsOKWithCORS(t *testing.T) {
	agg := NewAggregatorServer(9000, nil, "http://127.0.0.1:6500", "http://127.0.0.1:8080")
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	agg.Health(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if rr.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatalf("expected wildcard CORS, got %q", rr.Header().Get("Access-Control-Allow-Origin"))
	}
	if !strings.Contains(rr.Body.String(), `"status":"ok"`) {
		t.Fatalf("expected health JSON body, got %s", rr.Body.String())
	}
}

func TestParseNodesDropsEmptyValues(t *testing.T) {
	nodes := parseNodes("127.0.0.1:6500, ,127.0.0.1:6501")
	if len(nodes) != 2 {
		t.Fatalf("expected 2 nodes after trimming empties, got %d", len(nodes))
	}
	if nodes[0] != "127.0.0.1:6500" || nodes[1] != "127.0.0.1:6501" {
		t.Fatalf("unexpected parsed nodes: %+v", nodes)
	}
}
