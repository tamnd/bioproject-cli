// Package bioproject is the library behind the bioproject command line:
// the HTTP client, request shaping, and the typed data models for NCBI BioProject.
//
// The Client here is the spine every command shares. It sets a real
// User-Agent, paces requests so a busy session stays polite, and retries the
// transient failures (429 and 5xx) that any public API throws under load.
package bioproject

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DefaultUserAgent identifies the client to NCBI eUtils.
const DefaultUserAgent = "bioproject-cli/0.1.0 (github.com/tamnd/bioproject-cli)"

// Host is the eUtils hostname this client talks to, and the host the URI
// driver in domain.go claims.
const Host = "eutils.ncbi.nlm.nih.gov"

const baseURL = "https://eutils.ncbi.nlm.nih.gov/entrez/eutils"

const (
	ncbiEmail = "tamnd87@gmail.com"
	ncbiTool  = "bioproject-cli"
)

// Config holds the runtime settings for the BioProject client.
type Config struct {
	BaseURL   string
	Rate      time.Duration
	Retries   int
	Timeout   time.Duration
	UserAgent string
}

// DefaultConfig returns a Config with sensible defaults: 400ms rate limit,
// 3 retries, and a 30s timeout.
func DefaultConfig() Config {
	return Config{
		BaseURL:   baseURL,
		Rate:      400 * time.Millisecond,
		Retries:   3,
		Timeout:   30 * time.Second,
		UserAgent: DefaultUserAgent,
	}
}

// Client talks to NCBI eUtils for BioProject records.
type Client struct {
	cfg  Config
	http *http.Client
	last time.Time
}

// NewClient returns a Client using the given Config.
func NewClient(cfg Config) *Client {
	return &Client{
		cfg:  cfg,
		http: &http.Client{Timeout: cfg.Timeout},
	}
}

func (c *Client) wait() {
	if c.cfg.Rate > 0 {
		if since := time.Since(c.last); since < c.cfg.Rate {
			time.Sleep(c.cfg.Rate - since)
		}
	}
	c.last = time.Now()
}

func (c *Client) get(ctx context.Context, rawURL string, out any) error {
	for attempt := 0; attempt <= c.cfg.Retries; attempt++ {
		if attempt > 0 {
			d := time.Duration(attempt) * 500 * time.Millisecond
			if d > 5*time.Second {
				d = 5 * time.Second
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(d):
			}
		}
		c.wait()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		if err != nil {
			return err
		}
		req.Header.Set("User-Agent", c.cfg.UserAgent)
		resp, err := c.http.Do(req)
		if err != nil {
			if attempt < c.cfg.Retries {
				continue
			}
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			if attempt < c.cfg.Retries {
				continue
			}
			return fmt.Errorf("HTTP %d", resp.StatusCode)
		}
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("HTTP %d", resp.StatusCode)
		}
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return fmt.Errorf("all retries exhausted")
}

// --- wire types (unexported) ---

type wireSearch struct {
	ESearchResult struct {
		Count  string   `json:"count"`
		IDList []string `json:"idlist"`
	} `json:"esearchresult"`
}

type wireProject struct {
	UID         string `json:"uid"`
	ProjectID   int    `json:"project_id"`
	Title       string `json:"project_title"`
	Description string `json:"project_description"`
	Organism    string `json:"organism_name"`
	Type        string `json:"project_type"`
	Supergroup  string `json:"supergroup"`
}

type wireSummary struct {
	Result map[string]json.RawMessage `json:"result"`
}

// --- public types ---

// Project is a single NCBI BioProject record.
type Project struct {
	ID          string `json:"id"                   kit:"id"`
	Accession   string `json:"accession,omitempty"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Organism    string `json:"organism,omitempty"`
	Type        string `json:"type,omitempty"`
	Supergroup  string `json:"supergroup,omitempty"`
}

func toProject(w wireProject) *Project {
	acc := ""
	if w.ProjectID > 0 {
		acc = fmt.Sprintf("PRJNA%d", w.ProjectID)
	}
	return &Project{
		ID:          w.UID,
		Accession:   acc,
		Title:       w.Title,
		Description: w.Description,
		Organism:    w.Organism,
		Type:        w.Type,
		Supergroup:  w.Supergroup,
	}
}

// Search searches BioProject for records matching the query and returns IDs
// and the total hit count.
func (c *Client) Search(ctx context.Context, query string, limit, start int) ([]string, int, error) {
	u := fmt.Sprintf("%s/esearch.fcgi?db=bioproject&term=%s&retmax=%d&retstart=%d&retmode=json&email=%s&tool=%s",
		c.cfg.BaseURL, url.QueryEscape(query), limit, start, ncbiEmail, ncbiTool)
	var w wireSearch
	if err := c.get(ctx, u, &w); err != nil {
		return nil, 0, err
	}
	count := 0
	fmt.Sscanf(w.ESearchResult.Count, "%d", &count)
	return w.ESearchResult.IDList, count, nil
}

// FetchProjects fetches project details for the given UIDs (up to ~500 per
// call; callers should batch if needed).
func (c *Client) FetchProjects(ctx context.Context, ids []string) ([]*Project, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	u := fmt.Sprintf("%s/esummary.fcgi?db=bioproject&id=%s&retmode=json&email=%s&tool=%s",
		c.cfg.BaseURL, strings.Join(ids, ","), ncbiEmail, ncbiTool)
	var w wireSummary
	if err := c.get(ctx, u, &w); err != nil {
		return nil, err
	}
	rawUIDs, ok := w.Result["uids"]
	if !ok {
		return nil, fmt.Errorf("no uids in esummary response")
	}
	var uids []string
	if err := json.Unmarshal(rawUIDs, &uids); err != nil {
		return nil, err
	}
	var projects []*Project
	for _, uid := range uids {
		raw, ok := w.Result[uid]
		if !ok {
			continue
		}
		var wp wireProject
		if err := json.Unmarshal(raw, &wp); err != nil {
			continue
		}
		projects = append(projects, toProject(wp))
	}
	return projects, nil
}

// GetProject fetches a single BioProject by its numeric UID.
func (c *Client) GetProject(ctx context.Context, uid string) (*Project, error) {
	projects, err := c.FetchProjects(ctx, []string{uid})
	if err != nil {
		return nil, err
	}
	if len(projects) == 0 {
		return nil, fmt.Errorf("project %s not found", uid)
	}
	return projects[0], nil
}

// SearchAndFetch searches BioProject and returns full Project records.
func (c *Client) SearchAndFetch(ctx context.Context, query string, limit, start int) ([]*Project, int, error) {
	ids, total, err := c.Search(ctx, query, limit, start)
	if err != nil {
		return nil, 0, err
	}
	if len(ids) == 0 {
		return nil, total, nil
	}
	projects, err := c.FetchProjects(ctx, ids)
	if err != nil {
		return nil, 0, err
	}
	return projects, total, nil
}
