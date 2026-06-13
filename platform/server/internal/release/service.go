package release

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/worryyy/devops-platform/platform/server/internal/catalog"
	"github.com/worryyy/devops-platform/platform/server/internal/pkg/bizerr"
	"github.com/worryyy/devops-platform/platform/server/internal/pkg/platformerr"
	"github.com/worryyy/devops-platform/platform/server/internal/queue"
)

var (
	ErrServiceNotFound     = errors.New("service not found")
	ErrEnvironmentNotFound = errors.New("environment not found")
	ErrBranchNotAllowed    = errors.New("branch not allowed")
)

type Store interface {
	CreateRelease(ctx context.Context, input CreateReleaseInput) error
	GetRelease(ctx context.Context, id string) (Release, error)
	ListReleasesByService(ctx context.Context, serviceName string, limit int) ([]Release, error)
	UpdateReleaseStatus(ctx context.Context, id string, status Status, errorMessage *string) error
	UpdateReleaseStatusIfCurrent(ctx context.Context, id string, from Status, to Status) (bool, error)
	SetJenkinsBuild(ctx context.Context, id, jobName string, buildNumber int) error
	SetGitOpsRevision(ctx context.Context, id, commitSHA string) error
	SetImage(ctx context.Context, id, repo, tag, digest string) error
	AddEvent(ctx context.Context, releaseID string, status Status, message string, detail map[string]interface{}) error
	ListEvents(ctx context.Context, releaseID string) ([]Event, error)
	AcquireReleaseLock(ctx context.Context, serviceName, environment, releaseID string, ttl time.Duration) error
	ExtendReleaseLock(ctx context.Context, releaseID string, ttl time.Duration) error
	ReleaseReleaseLock(ctx context.Context, serviceName, environment, releaseID string) error
}

type Publisher interface {
	PublishReleaseRequested(ctx context.Context, message queue.ReleaseMessage) error
}

type CreateCommand struct {
	Service     string
	Environment string
	Branch      string
	Operator    string
}

type CreateResult struct {
	ReleaseID string `json:"release_id"`
	Status    Status `json:"status"`
}

type Service struct {
	catalog catalog.Catalog
	store   Store
	queue   Publisher
	lockTTL time.Duration
}

func NewService(catalog catalog.Catalog, store Store, publisher Publisher, lockTTL time.Duration) *Service {
	if lockTTL <= 0 {
		lockTTL = time.Hour
	}
	return &Service{catalog: catalog, store: store, queue: publisher, lockTTL: lockTTL}
}

func (s *Service) Catalog() catalog.Catalog {
	return s.catalog
}

func (s *Service) GetService(name string) (catalog.Service, error) {
	service, ok := s.catalog.ServiceByName(name)
	if !ok {
		return catalog.Service{}, bizerr.NotFoundWrap("service not found", ErrServiceNotFound)
	}
	return service, nil
}

func (s *Service) GetRelease(ctx context.Context, id string) (Release, error) {
	item, err := s.store.GetRelease(ctx, id)
	if err != nil {
		if errors.Is(err, platformerr.ErrNotFound) {
			return Release{}, bizerr.NotFoundWrap("release not found", err)
		}
		return Release{}, bizerr.InternalWrap("get release failed", err)
	}
	return item, nil
}

func (s *Service) ListEvents(ctx context.Context, releaseID string) ([]Event, error) {
	events, err := s.store.ListEvents(ctx, releaseID)
	if err != nil {
		return nil, bizerr.InternalWrap("list release events failed", err)
	}
	if events == nil {
		events = []Event{}
	}
	return events, nil
}

func (s *Service) ListReleasesByService(ctx context.Context, serviceName string, limit int) ([]Release, error) {
	if _, ok := s.catalog.ServiceByName(serviceName); !ok {
		return nil, bizerr.NotFoundWrap("service not found", ErrServiceNotFound)
	}
	releases, err := s.store.ListReleasesByService(ctx, serviceName, limit)
	if err != nil {
		return nil, bizerr.InternalWrap("list releases failed", err)
	}
	if releases == nil {
		releases = []Release{}
	}
	return releases, nil
}

func (s *Service) Create(ctx context.Context, command CreateCommand) (CreateResult, error) {
	service, ok := s.catalog.ServiceByName(command.Service)
	if !ok {
		return CreateResult{}, bizerr.NotFoundWrap("service not found", ErrServiceNotFound)
	}
	environment, ok := service.EnvironmentByName(command.Environment)
	if !ok {
		return CreateResult{}, bizerr.NotFoundWrap("environment not found", ErrEnvironmentNotFound)
	}
	if !catalog.BranchAllowed(environment.BranchPolicy, command.Branch) {
		err := fmt.Errorf("%w: %s", ErrBranchNotAllowed, command.Branch)
		return CreateResult{}, bizerr.ParamWrap("branch not allowed", err)
	}

	releaseID, err := NewID(time.Now())
	if err != nil {
		return CreateResult{}, bizerr.InternalWrap("generate release id failed", err)
	}

	if err := s.store.AcquireReleaseLock(ctx, command.Service, command.Environment, releaseID, s.lockTTL); err != nil {
		if errors.Is(err, platformerr.ErrReleaseLockHeld) {
			return CreateResult{}, bizerr.New(http.StatusConflict, "release already running for service and environment", err)
		}
		return CreateResult{}, bizerr.InternalWrap("acquire release lock failed", err)
	}
	lockHeld := true
	releaseLock := func() {
		if lockHeld {
			_ = s.store.ReleaseReleaseLock(context.Background(), command.Service, command.Environment, releaseID)
			lockHeld = false
		}
	}

	input := CreateReleaseInput{
		ID:          releaseID,
		ServiceName: command.Service,
		Environment: command.Environment,
		Branch:      command.Branch,
		Operator:    command.Operator,
		JenkinsJob:  environment.Jenkins.JobName,
		ImageRepo:   environment.Image.Repository,
		ArgoCDApp:   environment.ArgoCD.Application,
		Namespace:   environment.Kubernetes.Namespace,
		Deployment:  environment.Kubernetes.Deployment,
	}
	if err := s.store.CreateRelease(ctx, input); err != nil {
		releaseLock()
		return CreateResult{}, bizerr.InternalWrap("create release failed", err)
	}
	if err := s.store.AddEvent(ctx, releaseID, StatusRequested, "release requested", map[string]interface{}{
		"service":     command.Service,
		"environment": command.Environment,
		"branch":      command.Branch,
		"operator":    command.Operator,
	}); err != nil {
		releaseLock()
		return CreateResult{}, bizerr.InternalWrap("record release event failed", err)
	}

	if err := s.store.UpdateReleaseStatus(ctx, releaseID, StatusValidated, nil); err != nil {
		releaseLock()
		return CreateResult{}, bizerr.InternalWrap("validate release failed", err)
	}
	if err := s.store.AddEvent(ctx, releaseID, StatusValidated, "release request validated", map[string]interface{}{
		"jenkins_job": environment.Jenkins.JobName,
		"argocd_app":  environment.ArgoCD.Application,
		"deployment":  environment.Kubernetes.Deployment,
	}); err != nil {
		releaseLock()
		return CreateResult{}, bizerr.InternalWrap("record release event failed", err)
	}

	message := queue.ReleaseMessage{
		ReleaseID:   releaseID,
		Service:     command.Service,
		Environment: command.Environment,
		Event:       queue.EventReleaseRequested,
	}
	if err := s.queue.PublishReleaseRequested(ctx, message); err != nil {
		errMessage := err.Error()
		_ = s.store.UpdateReleaseStatus(context.Background(), releaseID, StatusFailed, &errMessage)
		_ = s.store.AddEvent(context.Background(), releaseID, StatusFailed, "failed to queue release", map[string]interface{}{"error": err.Error()})
		releaseLock()
		return CreateResult{}, bizerr.InternalWrap("queue release failed", err)
	}

	updated, err := s.store.UpdateReleaseStatusIfCurrent(ctx, releaseID, StatusValidated, StatusQueued)
	if err != nil {
		releaseLock()
		return CreateResult{}, bizerr.InternalWrap("queue release failed", err)
	}
	if updated {
		if err := s.store.AddEvent(ctx, releaseID, StatusQueued, "release message queued", map[string]interface{}{
			"exchange":    "platform.release.exchange",
			"routing_key": "release.requested",
		}); err != nil {
			releaseLock()
			return CreateResult{}, bizerr.InternalWrap("record release event failed", err)
		}
	}
	lockHeld = false
	return CreateResult{ReleaseID: releaseID, Status: StatusQueued}, nil
}

func NewID(now time.Time) (string, error) {
	var raw [3]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("generate release id entropy: %w", err)
	}
	return fmt.Sprintf("rel-%s-%s", now.UTC().Format("20060102-150405"), hex.EncodeToString(raw[:])), nil
}
