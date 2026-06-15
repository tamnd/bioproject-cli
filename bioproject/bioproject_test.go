package bioproject

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func testServer(t *testing.T, mux *http.ServeMux) (*httptest.Server, *Client) {
	t.Helper()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	cfg := DefaultConfig()
	cfg.BaseURL = srv.URL
	cfg.Rate = 0
	cfg.Retries = 0
	return srv, NewClient(cfg)
}

func TestSearch(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/esearch.fcgi", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("db") != "bioproject" {
			http.Error(w, "wrong db", 400)
			return
		}
		if r.URL.Query().Get("email") == "" {
			http.Error(w, "missing email", 400)
			return
		}
		json.NewEncoder(w).Encode(wireSearch{
			ESearchResult: struct {
				Count  string   `json:"count"`
				IDList []string `json:"idlist"`
			}{
				Count:  "1056474",
				IDList: []string{"638671", "722463"},
			},
		})
	})
	_, client := testServer(t, mux)
	ids, total, err := client.Search(context.Background(), "RNA-seq", 2, 0)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1056474 {
		t.Errorf("total = %d, want 1056474", total)
	}
	if len(ids) != 2 {
		t.Errorf("len = %d, want 2", len(ids))
	}
	if ids[0] != "638671" {
		t.Errorf("ids[0] = %q, want 638671", ids[0])
	}
}

func TestFetchProjects(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/esummary.fcgi", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("db") != "bioproject" {
			http.Error(w, "wrong db", 400)
			return
		}
		if r.URL.Query().Get("email") == "" {
			http.Error(w, "missing email", 400)
			return
		}
		wp := wireProject{
			UID:         "638671",
			ProjectID:   638671,
			Title:       "RNA-Seq study on root samples from Olea europaea cultivars",
			Description: "Transcriptome analysis of olive root samples.",
			Organism:    "Olea europaea",
			Type:        "Transcriptome or Gene expression",
			Supergroup:  "eukaryotes",
		}
		wpBytes, _ := json.Marshal(wp)
		result := map[string]json.RawMessage{
			"uids":   json.RawMessage(`["638671"]`),
			"638671": wpBytes,
		}
		json.NewEncoder(w).Encode(map[string]any{"result": result})
	})
	_, client := testServer(t, mux)
	projects, err := client.FetchProjects(context.Background(), []string{"638671"})
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 1 {
		t.Fatalf("len = %d, want 1", len(projects))
	}
	p := projects[0]
	if p.ID != "638671" {
		t.Errorf("ID = %q, want 638671", p.ID)
	}
	if p.Accession != "PRJNA638671" {
		t.Errorf("Accession = %q, want PRJNA638671", p.Accession)
	}
	if p.Organism != "Olea europaea" {
		t.Errorf("Organism = %q, want Olea europaea", p.Organism)
	}
	if p.Type != "Transcriptome or Gene expression" {
		t.Errorf("Type = %q, want Transcriptome or Gene expression", p.Type)
	}
	if p.Supergroup != "eukaryotes" {
		t.Errorf("Supergroup = %q, want eukaryotes", p.Supergroup)
	}
}

func TestGetProject(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/esummary.fcgi", func(w http.ResponseWriter, r *http.Request) {
		wp := wireProject{
			UID:       "722463",
			ProjectID: 722463,
			Title:     "Single-cell RNA-seq of human brain",
		}
		wpBytes, _ := json.Marshal(wp)
		result := map[string]json.RawMessage{
			"uids":   json.RawMessage(`["722463"]`),
			"722463": wpBytes,
		}
		json.NewEncoder(w).Encode(map[string]any{"result": result})
	})
	_, client := testServer(t, mux)
	p, err := client.GetProject(context.Background(), "722463")
	if err != nil {
		t.Fatal(err)
	}
	if p.ID != "722463" {
		t.Errorf("ID = %q, want 722463", p.ID)
	}
	if p.Accession != "PRJNA722463" {
		t.Errorf("Accession = %q, want PRJNA722463", p.Accession)
	}
}

func TestSearchAndFetch(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/esearch.fcgi", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(wireSearch{
			ESearchResult: struct {
				Count  string   `json:"count"`
				IDList []string `json:"idlist"`
			}{
				Count:  "42",
				IDList: []string{"638671"},
			},
		})
	})
	mux.HandleFunc("/esummary.fcgi", func(w http.ResponseWriter, r *http.Request) {
		wp := wireProject{
			UID:       "638671",
			ProjectID: 638671,
			Title:     "Olive root RNA-Seq",
			Organism:  "Olea europaea",
		}
		wpBytes, _ := json.Marshal(wp)
		result := map[string]json.RawMessage{
			"uids":   json.RawMessage(`["638671"]`),
			"638671": wpBytes,
		}
		json.NewEncoder(w).Encode(map[string]any{"result": result})
	})
	_, client := testServer(t, mux)
	projects, total, err := client.SearchAndFetch(context.Background(), "olive", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if total != 42 {
		t.Errorf("total = %d, want 42", total)
	}
	if len(projects) != 1 {
		t.Fatalf("len = %d, want 1", len(projects))
	}
	if projects[0].ID != "638671" {
		t.Errorf("ID = %q, want 638671", projects[0].ID)
	}
}

func TestFetchProjectsEmpty(t *testing.T) {
	_, client := testServer(t, http.NewServeMux())
	projects, err := client.FetchProjects(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if projects != nil {
		t.Error("expected nil projects for empty input")
	}
}
