package bioproject

import (
	"context"
	"fmt"
	"strings"

	"github.com/tamnd/any-cli/kit"
	"github.com/tamnd/any-cli/kit/errs"
)

// domain.go exposes bioproject as a kit Domain: a driver that a multi-domain
// host (ant) enables with a single blank import,
//
//	import _ "github.com/tamnd/bioproject-cli/bioproject"
//
// exactly as a database/sql program enables a driver with `import _
// "github.com/lib/pq"`. The init below registers it; the host then
// dereferences bioproject:// URIs by routing to the operations Register
// installs. The same Domain also builds the standalone bioproject binary
// (see cli.NewApp), so the binary and a host share one source of truth.
func init() { kit.Register(Domain{}) }

// Domain is the bioproject driver. It carries no state; the per-run client is
// built by the factory Register hands kit.
type Domain struct{}

// Info describes the scheme, the hostnames a pasted link is matched against,
// and the identity reused for the binary's help and version.
func (Domain) Info() kit.DomainInfo {
	return kit.DomainInfo{
		Scheme: "bioproject",
		Hosts:  []string{Host},
		Identity: kit.Identity{
			Binary: "bioproject",
			Short:  "A command line for NCBI BioProject.",
			Long: `A command line for NCBI BioProject.

bioproject reads public project records from NCBI BioProject, which archives
genomics and other large-scale biological projects submitted by researchers
worldwide. No API key required. 1M+ project records indexed.`,
			Site: "https://www.ncbi.nlm.nih.gov/bioproject/",
			Repo: "https://github.com/tamnd/bioproject-cli",
		},
	}
}

// Register installs the client factory and every operation onto app.
func (Domain) Register(app *kit.App) {
	app.SetClient(newClient)

	kit.Handle(app, kit.OpMeta{Name: "search", Group: "read", List: true,
		Summary: "Search BioProject records by keyword (--limit, --start)",
		Args:    []kit.Arg{{Name: "query", Help: "search query (e.g. RNA-seq cancer, BRCA1)"}}}, searchProjects)

	kit.Handle(app, kit.OpMeta{Name: "project", Group: "read", Single: true,
		Summary: "Get a single BioProject by numeric UID", URIType: "project", Resolver: true,
		Args: []kit.Arg{{Name: "uid", Help: "BioProject numeric UID (e.g. 638671)"}}}, getProject)

	kit.Handle(app, kit.OpMeta{Name: "organism", Group: "read", List: true,
		Summary: "List BioProject records for an organism (--limit, --start)",
		Args:    []kit.Arg{{Name: "name", Help: "organism name (e.g. Homo sapiens, Mus musculus)"}}}, organismProjects)
}

// newClient builds the BioProject client from the host-resolved config.
func newClient(_ context.Context, cfg kit.Config) (any, error) {
	c := DefaultConfig()
	if cfg.UserAgent != "" {
		c.UserAgent = cfg.UserAgent
	}
	if cfg.Rate > 0 {
		c.Rate = cfg.Rate
	}
	if cfg.Retries > 0 {
		c.Retries = cfg.Retries
	}
	if cfg.Timeout > 0 {
		c.Timeout = cfg.Timeout
	}
	return NewClient(c), nil
}

// --- inputs ---

type searchInput struct {
	Query  string  `kit:"arg"          help:"search query"`
	Limit  int     `kit:"flag,inherit" help:"max results"`
	Start  int     `kit:"flag"         help:"offset for pagination"`
	Client *Client `kit:"inject"`
}

type projectInput struct {
	UID    string  `kit:"arg"    help:"BioProject numeric UID"`
	Client *Client `kit:"inject"`
}

type organismInput struct {
	Name   string  `kit:"arg"          help:"organism name"`
	Limit  int     `kit:"flag,inherit" help:"max results"`
	Start  int     `kit:"flag"         help:"offset for pagination"`
	Client *Client `kit:"inject"`
}

// --- handlers ---

func searchProjects(ctx context.Context, in searchInput, emit func(*Project) error) error {
	limit := in.Limit
	if limit <= 0 {
		limit = 20
	}
	projects, _, err := in.Client.SearchAndFetch(ctx, in.Query, limit, in.Start)
	if err != nil {
		return err
	}
	for _, p := range projects {
		if err := emit(p); err != nil {
			return err
		}
	}
	return nil
}

func getProject(ctx context.Context, in projectInput, emit func(*Project) error) error {
	p, err := in.Client.GetProject(ctx, in.UID)
	if err != nil {
		return err
	}
	return emit(p)
}

func organismProjects(ctx context.Context, in organismInput, emit func(*Project) error) error {
	limit := in.Limit
	if limit <= 0 {
		limit = 20
	}
	query := fmt.Sprintf("%s[orgn]", in.Name)
	projects, _, err := in.Client.SearchAndFetch(ctx, query, limit, in.Start)
	if err != nil {
		return err
	}
	for _, p := range projects {
		if err := emit(p); err != nil {
			return err
		}
	}
	return nil
}

// Classify turns any accepted input into the canonical (type, id).
// Any non-empty string maps to ("project", input).
func (Domain) Classify(input string) (string, string, error) {
	s := strings.TrimSpace(input)
	if s == "" {
		return "", "", errs.Usage("empty BioProject reference")
	}
	return "project", s, nil
}

// Locate is the inverse: the live https URL for a (type, id).
func (Domain) Locate(t, id string) (string, error) {
	switch t {
	case "project":
		return fmt.Sprintf("https://www.ncbi.nlm.nih.gov/bioproject/%s", id), nil
	default:
		return "", errs.Usage("bioproject has no resource type %q", t)
	}
}
