package bioproject

import (
	"testing"
)

// These tests are offline: they exercise the URI driver's pure string
// functions. The client's HTTP behaviour is covered in bioproject_test.go.

func TestDomainInfo(t *testing.T) {
	info := Domain{}.Info()
	if info.Scheme != "bioproject" {
		t.Errorf("Scheme = %q, want bioproject", info.Scheme)
	}
	if len(info.Hosts) == 0 || info.Hosts[0] != Host {
		t.Errorf("Hosts = %v, want [%s]", info.Hosts, Host)
	}
	if info.Identity.Binary != "bioproject" {
		t.Errorf("Identity.Binary = %q, want bioproject", info.Identity.Binary)
	}
}

func TestClassify(t *testing.T) {
	cases := []struct {
		in  string
		typ string
		id  string
	}{
		{"638671", "project", "638671"},
		{"PRJNA638671", "project", "PRJNA638671"},
		{"RNA-seq cancer", "project", "RNA-seq cancer"},
	}
	for _, tc := range cases {
		typ, id, err := Domain{}.Classify(tc.in)
		if err != nil {
			t.Errorf("Classify(%q) error: %v", tc.in, err)
			continue
		}
		if typ != tc.typ {
			t.Errorf("Classify(%q) type = %q, want %q", tc.in, typ, tc.typ)
		}
		if id != tc.id {
			t.Errorf("Classify(%q) id = %q, want %q", tc.in, id, tc.id)
		}
	}
}

func TestClassifyEmpty(t *testing.T) {
	_, _, err := Domain{}.Classify("")
	if err == nil {
		t.Error("Classify(\"\") expected error, got nil")
	}
	_, _, err = Domain{}.Classify("   ")
	if err == nil {
		t.Error("Classify(\"   \") expected error, got nil")
	}
}

func TestLocate(t *testing.T) {
	cases := []struct {
		typ  string
		id   string
		want string
	}{
		{"project", "638671", "https://www.ncbi.nlm.nih.gov/bioproject/638671"},
		{"project", "PRJNA638671", "https://www.ncbi.nlm.nih.gov/bioproject/PRJNA638671"},
	}
	for _, tc := range cases {
		got, err := Domain{}.Locate(tc.typ, tc.id)
		if err != nil {
			t.Errorf("Locate(%q, %q) error: %v", tc.typ, tc.id, err)
			continue
		}
		if got != tc.want {
			t.Errorf("Locate(%q, %q) = %q, want %q", tc.typ, tc.id, got, tc.want)
		}
	}
}

func TestLocateInvalidType(t *testing.T) {
	_, err := Domain{}.Locate("page", "something")
	if err == nil {
		t.Error("expected error for unknown type")
	}
}
