package catalog

import (
	"path/filepath"
	"testing"
)

func TestLoadServiceCatalog(t *testing.T) {
	catalog, err := Load(filepath.Clean("../../configs/service-catalog.yaml"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if catalog.Version != "v1" {
		t.Fatalf("Version = %q, want v1", catalog.Version)
	}
	service, ok := catalog.ServiceByName("forum-api")
	if !ok {
		t.Fatalf("ServiceByName(forum-api) = false")
	}
	environment, ok := service.EnvironmentByName("dev")
	if !ok {
		t.Fatalf("EnvironmentByName(dev) = false")
	}
	if !BranchAllowed(environment.BranchPolicy, "feature/add-login") {
		t.Fatalf("BranchAllowed(feature/add-login) = false")
	}
}

func TestBranchAllowed(t *testing.T) {
	policy := BranchPolicy{
		AllowedBranches: []string{"main", "develop", "feature/*"},
	}
	cases := map[string]bool{
		"main":            true,
		"develop":         true,
		"feature/x":       true,
		"feature/foo/bar": true,
		"bugfix/x":        false,
		"feat":            false,
	}
	for branch, want := range cases {
		if got := BranchAllowed(policy, branch); got != want {
			t.Fatalf("BranchAllowed(%q) = %v, want %v", branch, got, want)
		}
	}
}
