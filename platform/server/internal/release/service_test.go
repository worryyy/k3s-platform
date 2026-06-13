package release

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/worryyy/devops-platform/platform/server/internal/catalog"
	"github.com/worryyy/devops-platform/platform/server/internal/pkg/bizerr"
	"github.com/worryyy/devops-platform/platform/server/internal/pkg/platformerr"
	"github.com/worryyy/devops-platform/platform/server/internal/queue"
)

type fakeStore struct {
	acquireErr  error
	createErr   error
	updateErr   error
	eventErr    error
	releaseErr  error
	release     map[string]Release
	events      map[string][]Event
	acquireKeys []string
	releaseKeys []string
	statuses    []Status
}

func (f *fakeStore) CreateRelease(ctx context.Context, input CreateReleaseInput) error {
	if f.createErr != nil {
		return f.createErr
	}
	if f.release == nil {
		f.release = map[string]Release{}
	}
	f.release[input.ID] = Release{
		ID:          input.ID,
		ServiceName: input.ServiceName,
		Environment: input.Environment,
		Branch:      input.Branch,
		Status:      StatusRequested,
		Operator:    input.Operator,
		JenkinsJob:  &input.JenkinsJob,
		ImageRepo:   &input.ImageRepo,
		ArgoCDApp:   &input.ArgoCDApp,
		Namespace:   &input.Namespace,
		Deployment:  &input.Deployment,
	}
	return nil
}

func (f *fakeStore) GetRelease(ctx context.Context, id string) (Release, error) {
	if f.releaseErr != nil {
		return Release{}, f.releaseErr
	}
	if item, ok := f.release[id]; ok {
		return item, nil
	}
	return Release{}, platformerr.ErrNotFound
}

func (f *fakeStore) ListReleasesByService(ctx context.Context, serviceName string, limit int) ([]Release, error) {
	return nil, nil
}

func (f *fakeStore) UpdateReleaseStatus(ctx context.Context, id string, status Status, errorMessage *string) error {
	if f.updateErr != nil {
		return f.updateErr
	}
	item := f.release[id]
	item.Status = status
	if errorMessage != nil {
		item.ErrorMessage = errorMessage
	}
	f.release[id] = item
	f.statuses = append(f.statuses, status)
	return nil
}

func (f *fakeStore) UpdateReleaseStatusIfCurrent(ctx context.Context, id string, from Status, to Status) (bool, error) {
	if f.updateErr != nil {
		return false, f.updateErr
	}
	item := f.release[id]
	if item.Status != from {
		return false, nil
	}
	item.Status = to
	f.release[id] = item
	f.statuses = append(f.statuses, to)
	return true, nil
}

func (f *fakeStore) SetJenkinsBuild(ctx context.Context, id, jobName string, buildNumber int) error {
	item := f.release[id]
	item.JenkinsJob = &jobName
	item.JenkinsBuildNumber = &buildNumber
	f.release[id] = item
	return nil
}

func (f *fakeStore) SetGitOpsRevision(ctx context.Context, id, commitSHA string) error { return nil }

func (f *fakeStore) SetImage(ctx context.Context, id, repo, tag, digest string) error { return nil }

func (f *fakeStore) AddEvent(ctx context.Context, releaseID string, status Status, message string, detail map[string]interface{}) error {
	if f.eventErr != nil {
		return f.eventErr
	}
	if f.events == nil {
		f.events = map[string][]Event{}
	}
	f.events[releaseID] = append(f.events[releaseID], Event{
		ReleaseID: releaseID,
		Status:    status,
		Message:   message,
		Detail:    detail,
	})
	return nil
}

func (f *fakeStore) ListEvents(ctx context.Context, releaseID string) ([]Event, error) {
	return f.events[releaseID], nil
}

func (f *fakeStore) AcquireReleaseLock(ctx context.Context, serviceName, environment, releaseID string, ttl time.Duration) error {
	f.acquireKeys = append(f.acquireKeys, serviceName+"/"+environment+"/"+releaseID)
	return f.acquireErr
}

func (f *fakeStore) ExtendReleaseLock(ctx context.Context, releaseID string, ttl time.Duration) error {
	return nil
}

func (f *fakeStore) ReleaseReleaseLock(ctx context.Context, serviceName, environment, releaseID string) error {
	f.releaseKeys = append(f.releaseKeys, serviceName+"/"+environment+"/"+releaseID)
	return f.releaseErr
}

type fakePublisher struct {
	published []queue.ReleaseMessage
	err       error
}

func (f *fakePublisher) PublishReleaseRequested(ctx context.Context, message queue.ReleaseMessage) error {
	if f.err != nil {
		return f.err
	}
	f.published = append(f.published, message)
	return nil
}

func TestServiceCreateSuccess(t *testing.T) {
	cat := catalog.Catalog{
		Version: "v1",
		Services: []catalog.Service{
			{
				Name:        "forum-api",
				DisplayName: "Forum API",
				Owner:       "worryyy",
				Environments: []catalog.Environment{
					{
						Name:      "dev",
						Namespace: "app",
						BranchPolicy: catalog.BranchPolicy{
							DefaultBranch:   "main",
							AllowedBranches: []string{"main", "develop", "feature/*"},
						},
						Git: catalog.GitConfig{
							Repo:       "https://github.com/worryyy/forum-app.git",
							ChartPath:  "k3s/charts/forum-api",
							ValuesFile: "k3s/helm-values/workloads/forum-api-business.yaml",
						},
						Image: catalog.ImageConfig{
							Repository: "repo",
							TagPolicy:  "git-sha",
						},
						Jenkins: catalog.JenkinsConfig{
							Mode:    "legacy",
							JobName: "forum-api-pipeline",
						},
						ArgoCD: catalog.ArgoCDConfig{
							Application: "forum-api",
							Namespace:   "argocd",
						},
						Kubernetes: catalog.KubernetesConfig{
							Namespace:  "app",
							Deployment: "forum-api",
							Service:    "forum-api",
							Container:  "forum-api",
						},
						Health: catalog.HealthConfig{
							HealthPath: "/healthz",
							ReadyPath:  "/readyz",
						},
					},
				},
			},
		},
	}

	store := &fakeStore{}
	publisher := &fakePublisher{}
	svc := NewService(cat, store, publisher, time.Hour)

	result, err := svc.Create(context.Background(), CreateCommand{
		Service:     "forum-api",
		Environment: "dev",
		Branch:      "main",
		Operator:    "worryyy",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if result.ReleaseID == "" {
		t.Fatalf("Create() release id is empty")
	}
	if result.Status != StatusQueued {
		t.Fatalf("Create() status = %s, want Queued", result.Status)
	}
	if got := len(publisher.published); got != 1 {
		t.Fatalf("published messages = %d, want 1", got)
	}
	if publisher.published[0].Event != queue.EventReleaseRequested {
		t.Fatalf("published event = %q, want %q", publisher.published[0].Event, queue.EventReleaseRequested)
	}
	events := store.events[result.ReleaseID]
	if len(events) != 3 {
		t.Fatalf("events = %d, want 3", len(events))
	}
	if events[0].Status != StatusRequested || events[1].Status != StatusValidated || events[2].Status != StatusQueued {
		t.Fatalf("events statuses = %#v", []Status{events[0].Status, events[1].Status, events[2].Status})
	}
	if got := len(store.releaseKeys); got != 0 {
		t.Fatalf("release lock should remain held after successful create, got %d release calls", got)
	}
}

func TestServiceCreateRejectsHeldLock(t *testing.T) {
	cat := catalog.Catalog{
		Version:  "v1",
		Services: []catalog.Service{{Name: "forum-api", Environments: []catalog.Environment{{Name: "dev", BranchPolicy: catalog.BranchPolicy{DefaultBranch: "main", AllowedBranches: []string{"main"}}, Git: catalog.GitConfig{Repo: "x", ChartPath: "y", ValuesFile: "z"}, Image: catalog.ImageConfig{Repository: "repo", TagPolicy: "git-sha"}, Jenkins: catalog.JenkinsConfig{Mode: "legacy", JobName: "forum-api-pipeline"}, ArgoCD: catalog.ArgoCDConfig{Application: "forum-api", Namespace: "argocd"}, Kubernetes: catalog.KubernetesConfig{Namespace: "app", Deployment: "forum-api", Service: "forum-api", Container: "forum-api"}, Health: catalog.HealthConfig{HealthPath: "/healthz", ReadyPath: "/readyz"}}}}},
	}
	releaseStore := &fakeStore{acquireErr: platformerr.ErrReleaseLockHeld}
	svc := NewService(cat, releaseStore, &fakePublisher{}, time.Hour)

	_, err := svc.Create(context.Background(), CreateCommand{
		Service:     "forum-api",
		Environment: "dev",
		Branch:      "main",
		Operator:    "worryyy",
	})
	if !errors.Is(err, platformerr.ErrReleaseLockHeld) {
		t.Fatalf("Create() error = %v, want ErrReleaseLockHeld", err)
	}
	assertBizError(t, err, http.StatusConflict, "release already running for service and environment")
}

func TestServiceCreateRejectsUnknownLockError(t *testing.T) {
	lockErr := errors.New("postgres unavailable")
	cat := catalog.Catalog{
		Version:  "v1",
		Services: []catalog.Service{{Name: "forum-api", Environments: []catalog.Environment{{Name: "dev", BranchPolicy: catalog.BranchPolicy{DefaultBranch: "main", AllowedBranches: []string{"main"}}, Git: catalog.GitConfig{Repo: "x", ChartPath: "y", ValuesFile: "z"}, Image: catalog.ImageConfig{Repository: "repo", TagPolicy: "git-sha"}, Jenkins: catalog.JenkinsConfig{Mode: "legacy", JobName: "forum-api-pipeline"}, ArgoCD: catalog.ArgoCDConfig{Application: "forum-api", Namespace: "argocd"}, Kubernetes: catalog.KubernetesConfig{Namespace: "app", Deployment: "forum-api", Service: "forum-api", Container: "forum-api"}, Health: catalog.HealthConfig{HealthPath: "/healthz", ReadyPath: "/readyz"}}}}},
	}
	svc := NewService(cat, &fakeStore{acquireErr: lockErr}, &fakePublisher{}, time.Hour)

	_, err := svc.Create(context.Background(), CreateCommand{
		Service:     "forum-api",
		Environment: "dev",
		Branch:      "main",
		Operator:    "worryyy",
	})
	if !errors.Is(err, lockErr) {
		t.Fatalf("Create() error = %v, want lockErr", err)
	}
	assertBizError(t, err, http.StatusInternalServerError, "acquire release lock failed")
}

func TestServiceCreateRejectsBranch(t *testing.T) {
	cat := catalog.Catalog{
		Version: "v1",
		Services: []catalog.Service{
			{
				Name: "forum-api",
				Environments: []catalog.Environment{
					{
						Name: "dev",
						BranchPolicy: catalog.BranchPolicy{
							DefaultBranch:   "main",
							AllowedBranches: []string{"main"},
						},
					},
				},
			},
		},
	}
	svc := NewService(cat, &fakeStore{}, &fakePublisher{}, time.Hour)

	_, err := svc.Create(context.Background(), CreateCommand{
		Service:     "forum-api",
		Environment: "dev",
		Branch:      "feature/x",
		Operator:    "worryyy",
	})
	if !errors.Is(err, ErrBranchNotAllowed) {
		t.Fatalf("Create() error = %v, want ErrBranchNotAllowed", err)
	}
	assertBizError(t, err, http.StatusBadRequest, "branch not allowed")
}

func TestServiceCreateRejectsUnknownService(t *testing.T) {
	svc := NewService(catalog.Catalog{Version: "v1"}, &fakeStore{}, &fakePublisher{}, time.Hour)

	_, err := svc.Create(context.Background(), CreateCommand{
		Service:     "missing",
		Environment: "dev",
		Branch:      "main",
		Operator:    "worryyy",
	})
	if !errors.Is(err, ErrServiceNotFound) {
		t.Fatalf("Create() error = %v, want ErrServiceNotFound", err)
	}
	assertBizError(t, err, http.StatusNotFound, "service not found")
}

func TestServiceGetReleaseMapsNotFound(t *testing.T) {
	releaseStore := &fakeStore{}
	svc := NewService(catalog.Catalog{Version: "v1"}, releaseStore, &fakePublisher{}, time.Hour)

	_, err := svc.GetRelease(context.Background(), "missing")
	if !errors.Is(err, platformerr.ErrNotFound) {
		t.Fatalf("GetRelease() error = %v, want ErrNotFound", err)
	}
	assertBizError(t, err, http.StatusNotFound, "release not found")
}

func TestServiceGetReleaseMapsStoreFailure(t *testing.T) {
	storeErr := errors.New("postgres unavailable")
	svc := NewService(catalog.Catalog{Version: "v1"}, &fakeStore{releaseErr: storeErr}, &fakePublisher{}, time.Hour)

	_, err := svc.GetRelease(context.Background(), "missing")
	if !errors.Is(err, storeErr) {
		t.Fatalf("GetRelease() error = %v, want storeErr", err)
	}
	assertBizError(t, err, http.StatusInternalServerError, "get release failed")
}

func assertBizError(t *testing.T, err error, code int, message string) {
	t.Helper()
	var businessError *bizerr.Error
	if !errors.As(err, &businessError) {
		t.Fatalf("error %T does not wrap bizerr.Error", err)
	}
	if businessError.Code != code {
		t.Fatalf("Code = %d, want %d", businessError.Code, code)
	}
	if businessError.Message != message {
		t.Fatalf("Message = %q, want %q", businessError.Message, message)
	}
}
