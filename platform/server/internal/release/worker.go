package release

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/worryyy/k3s-platform/platform/server/internal/catalog"
	"github.com/worryyy/k3s-platform/platform/server/internal/integrations/argocd"
	k8s "github.com/worryyy/k3s-platform/platform/server/internal/integrations/kubernetes"
	"github.com/worryyy/k3s-platform/platform/server/internal/queue"
)

type WorkerConfig struct {
	Catalog    catalog.Catalog
	Store      Store
	Jenkins    JenkinsAdapter
	ArgoCD     ArgoCDAdapter
	Kubernetes KubernetesAdapter
	Logger     *slog.Logger

	JenkinsPollInterval time.Duration
	JenkinsTimeout      time.Duration
	ArgoPollInterval    time.Duration
	ArgoTimeout         time.Duration
	RolloutPollInterval time.Duration
	RolloutTimeout      time.Duration
	LockTTL             time.Duration
}

type JenkinsAdapter interface {
	TriggerBuild(jobName string) (int, error)
	GetBuildStatus(jobName string, buildNumber int) (string, error)
	GetBuildLog(jobName string, buildNumber int) (string, error)
}

type ArgoCDAdapter interface {
	GetApplication(ctx context.Context, namespace, name string) (argocd.Application, error)
}

type KubernetesAdapter interface {
	GetDeploymentStatus(ctx context.Context, namespace, name string) (k8s.DeploymentStatus, error)
	ListPodsForDeployment(ctx context.Context, namespace, deployment string) ([]k8s.PodStatus, error)
	GetRecentEvents(ctx context.Context, namespace string) ([]k8s.EventRecord, error)
}

type Worker struct {
	store      Store
	catalog    catalog.Catalog
	jenkins    JenkinsAdapter
	argocd     ArgoCDAdapter
	kubernetes KubernetesAdapter
	logger     *slog.Logger

	jenkinsPollInterval time.Duration
	jenkinsTimeout      time.Duration
	argoPollInterval    time.Duration
	argoTimeout         time.Duration
	rolloutPollInterval time.Duration
	rolloutTimeout      time.Duration
	lockTTL             time.Duration
}

func NewWorker(cfg WorkerConfig) *Worker {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	if cfg.JenkinsPollInterval <= 0 {
		cfg.JenkinsPollInterval = 10 * time.Second
	}
	if cfg.JenkinsTimeout <= 0 {
		cfg.JenkinsTimeout = 45 * time.Minute
	}
	if cfg.ArgoPollInterval <= 0 {
		cfg.ArgoPollInterval = 10 * time.Second
	}
	if cfg.ArgoTimeout <= 0 {
		cfg.ArgoTimeout = 10 * time.Minute
	}
	if cfg.RolloutPollInterval <= 0 {
		cfg.RolloutPollInterval = 5 * time.Second
	}
	if cfg.RolloutTimeout <= 0 {
		cfg.RolloutTimeout = 10 * time.Minute
	}
	if cfg.LockTTL <= 0 {
		cfg.LockTTL = time.Hour
	}
	return &Worker{
		store:               cfg.Store,
		catalog:             cfg.Catalog,
		jenkins:             cfg.Jenkins,
		argocd:              cfg.ArgoCD,
		kubernetes:          cfg.Kubernetes,
		logger:              logger,
		jenkinsPollInterval: cfg.JenkinsPollInterval,
		jenkinsTimeout:      cfg.JenkinsTimeout,
		argoPollInterval:    cfg.ArgoPollInterval,
		argoTimeout:         cfg.ArgoTimeout,
		rolloutPollInterval: cfg.RolloutPollInterval,
		rolloutTimeout:      cfg.RolloutTimeout,
		lockTTL:             cfg.LockTTL,
	}
}

func (w *Worker) HandleReleaseMessage(ctx context.Context, message queue.ReleaseMessage) error {
	return w.ProcessRelease(ctx, message.ReleaseID)
}

func (w *Worker) ProcessRelease(ctx context.Context, releaseID string) error {
	item, err := w.store.GetRelease(ctx, releaseID)
	if err != nil {
		return err
	}

	if IsTerminal(item.Status) {
		w.logger.Info("release already terminal", "release_id", releaseID, "status", item.Status)
		return nil
	}

	service, ok := w.catalog.ServiceByName(item.ServiceName)
	if !ok {
		return fmt.Errorf("catalog missing service %q", item.ServiceName)
	}
	environment, ok := service.EnvironmentByName(item.Environment)
	if !ok {
		return fmt.Errorf("catalog missing environment %q for service %q", item.Environment, item.ServiceName)
	}

	ctx = context.WithValue(ctx, workerContextKeyReleaseID{}, releaseID)
	if item.Status == StatusRequested || item.Status == StatusValidated {
		if err := w.ensureQueued(ctx, &item); err != nil {
			return err
		}
	}

	if item.Status == StatusQueued {
		if err := w.ensureJenkinsTriggered(ctx, &item, environment); err != nil {
			return err
		}
	}

	if item.Status == StatusJenkinsTriggered || item.Status == StatusJenkinsRunning {
		if err := w.waitForJenkins(ctx, &item, environment); err != nil {
			return err
		}
	}

	if item.Status == StatusGitOpsUpdated || item.Status == StatusArgoSyncing {
		if err := w.waitForArgo(ctx, &item, environment); err != nil {
			return err
		}
	}

	if item.Status == StatusRolloutChecking {
		if err := w.checkRollout(ctx, &item, environment); err != nil {
			return err
		}
	}

	if item.Status == StatusSucceeded || item.Status == StatusFailed || item.Status == StatusTimeout || item.Status == StatusCanceled {
		return nil
	}

	return nil
}

func (w *Worker) ensureQueued(ctx context.Context, item *Release) error {
	if item.Status != StatusRequested && item.Status != StatusValidated {
		return nil
	}
	updated, err := w.store.UpdateReleaseStatusIfCurrent(ctx, item.ID, item.Status, StatusQueued)
	if err != nil {
		return err
	}
	if updated {
		item.Status = StatusQueued
		if err := w.store.AddEvent(ctx, item.ID, StatusQueued, "release message queued", map[string]interface{}{
			"exchange":    "platform.release.exchange",
			"routing_key": "release.requested",
		}); err != nil {
			return err
		}
		return nil
	}
	current, err := w.store.GetRelease(ctx, item.ID)
	if err != nil {
		return err
	}
	*item = current
	return nil
}

func (w *Worker) ensureJenkinsTriggered(ctx context.Context, item *Release, environment catalog.Environment) error {
	if item.Status != StatusQueued {
		return nil
	}
	if item.JenkinsBuildNumber != nil {
		return w.waitForJenkins(ctx, item, environment)
	}

	updated, err := w.store.UpdateReleaseStatusIfCurrent(ctx, item.ID, StatusQueued, StatusJenkinsTriggered)
	if err != nil {
		return err
	}
	if !updated {
		current, err := w.store.GetRelease(ctx, item.ID)
		if err != nil {
			return err
		}
		*item = current
		return nil
	}
	if err := w.store.ExtendReleaseLock(ctx, item.ID, w.lockTTL); err != nil {
		return err
	}
	if err := w.store.AddEvent(ctx, item.ID, StatusJenkinsTriggered, "triggering Jenkins build", map[string]interface{}{
		"job_name": environment.Jenkins.JobName,
	}); err != nil {
		return err
	}

	buildNumber, err := w.jenkins.TriggerBuild(environment.Jenkins.JobName)
	if err != nil {
		return w.fail(ctx, item.ID, StatusFailed, "jenkins trigger failed", err, map[string]interface{}{
			"job_name": environment.Jenkins.JobName,
		})
	}
	if err := w.store.SetJenkinsBuild(ctx, item.ID, environment.Jenkins.JobName, buildNumber); err != nil {
		return err
	}
	item.JenkinsJob = &environment.Jenkins.JobName
	item.JenkinsBuildNumber = &buildNumber
	if err := w.store.UpdateReleaseStatus(ctx, item.ID, StatusJenkinsRunning, nil); err != nil {
		return err
	}
	if err := w.store.AddEvent(ctx, item.ID, StatusJenkinsRunning, "Jenkins build triggered", map[string]interface{}{
		"job_name":     environment.Jenkins.JobName,
		"build_number": buildNumber,
	}); err != nil {
		return err
	}
	return w.waitForJenkins(ctx, item, environment)
}

func (w *Worker) waitForJenkins(ctx context.Context, item *Release, environment catalog.Environment) error {
	buildNumber, err := w.resolveBuildNumber(ctx, item, environment)
	if err != nil {
		return err
	}
	deadline := time.Now().Add(w.jenkinsTimeout)
	for {
		if err := contextErr(ctx); err != nil {
			return err
		}
		status, err := w.jenkins.GetBuildStatus(environment.Jenkins.JobName, buildNumber)
		if err != nil {
			return err
		}
		switch normalizeJenkinsStatus(status) {
		case "SUCCESS":
			if err := w.store.AddEvent(ctx, item.ID, StatusGitOpsUpdated, "Jenkins build succeeded", map[string]interface{}{
				"job_name":     environment.Jenkins.JobName,
				"build_number": buildNumber,
			}); err != nil {
				return err
			}
			if err := w.store.UpdateReleaseStatus(ctx, item.ID, StatusGitOpsUpdated, nil); err != nil {
				return err
			}
			item.Status = StatusGitOpsUpdated
			item.JenkinsBuildNumber = &buildNumber
			return w.waitForArgo(ctx, item, environment)
		case "FAILURE":
			return w.fail(ctx, item.ID, StatusFailed, "Jenkins build failed", nil, map[string]interface{}{
				"job_name":     environment.Jenkins.JobName,
				"build_number": buildNumber,
				"console":      mustBuildLog(w.jenkins, environment.Jenkins.JobName, buildNumber),
			})
		case "ABORTED":
			return w.fail(ctx, item.ID, StatusCanceled, "Jenkins build aborted", nil, map[string]interface{}{
				"job_name":     environment.Jenkins.JobName,
				"build_number": buildNumber,
			})
		default:
			if time.Now().After(deadline) {
				return w.fail(ctx, item.ID, StatusTimeout, "Jenkins build timeout", nil, map[string]interface{}{
					"job_name":     environment.Jenkins.JobName,
					"build_number": buildNumber,
				})
			}
			if err := w.transition(ctx, item.ID, StatusJenkinsRunning, "Jenkins build running", map[string]interface{}{
				"job_name":      environment.Jenkins.JobName,
				"build_number":  buildNumber,
				"jenkins_state": status,
			}); err != nil {
				return err
			}
			item.Status = StatusJenkinsRunning
			item.JenkinsBuildNumber = &buildNumber
			timer := time.NewTimer(w.jenkinsPollInterval)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
		}
	}
}

func (w *Worker) waitForArgo(ctx context.Context, item *Release, environment catalog.Environment) error {
	deadline := time.Now().Add(w.argoTimeout)
	if err := w.transition(ctx, item.ID, StatusArgoSyncing, "waiting for ArgoCD sync", map[string]interface{}{
		"application": environment.ArgoCD.Application,
		"namespace":   environment.ArgoCD.Namespace,
	}); err != nil {
		return err
	}
	item.Status = StatusArgoSyncing
	for {
		if err := contextErr(ctx); err != nil {
			return err
		}
		app, err := w.argocd.GetApplication(ctx, environment.ArgoCD.Namespace, environment.ArgoCD.Application)
		if err != nil {
			return err
		}
		if app.IsHealthyAndSynced() {
			if app.Revision != "" {
				if err := w.store.SetGitOpsRevision(ctx, item.ID, app.Revision); err != nil {
					return err
				}
			}
			if err := w.store.AddEvent(ctx, item.ID, StatusRolloutChecking, "ArgoCD application synced and healthy", map[string]interface{}{
				"application":     environment.ArgoCD.Application,
				"sync_status":     app.SyncStatus,
				"health_status":   app.HealthStatus,
				"revision":        app.Revision,
				"operation_phase": app.OperationPhase,
			}); err != nil {
				return err
			}
			if err := w.store.UpdateReleaseStatus(ctx, item.ID, StatusRolloutChecking, nil); err != nil {
				return err
			}
			item.Status = StatusRolloutChecking
			item.ArgoCDApp = &environment.ArgoCD.Application
			item.Namespace = &environment.Kubernetes.Namespace
			item.Deployment = &environment.Kubernetes.Deployment
			return w.checkRollout(ctx, item, environment)
		}
		if app.IsDegraded() || app.OperationFailed() {
			return w.fail(ctx, item.ID, StatusFailed, "ArgoCD application degraded", nil, map[string]interface{}{
				"application":     environment.ArgoCD.Application,
				"sync_status":     app.SyncStatus,
				"health_status":   app.HealthStatus,
				"revision":        app.Revision,
				"operation_phase": app.OperationPhase,
			})
		}
		if time.Now().After(deadline) {
			return w.fail(ctx, item.ID, StatusTimeout, "ArgoCD sync timeout", nil, map[string]interface{}{
				"application": environment.ArgoCD.Application,
			})
		}
		timer := time.NewTimer(w.argoPollInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func (w *Worker) checkRollout(ctx context.Context, item *Release, environment catalog.Environment) error {
	deadline := time.Now().Add(w.rolloutTimeout)
	for {
		if err := contextErr(ctx); err != nil {
			return err
		}
		status, err := w.kubernetes.GetDeploymentStatus(ctx, environment.Kubernetes.Namespace, environment.Kubernetes.Deployment)
		if err != nil {
			return err
		}
		if status.Success() {
			if err := w.store.AddEvent(ctx, item.ID, StatusSucceeded, "deployment rollout succeeded", map[string]interface{}{
				"namespace":          environment.Kubernetes.Namespace,
				"deployment":         environment.Kubernetes.Deployment,
				"ready_replicas":     status.ReadyReplicas,
				"updated_replicas":   status.UpdatedReplicas,
				"available_replicas": status.AvailableReplicas,
			}); err != nil {
				return err
			}
			if err := w.store.UpdateReleaseStatus(ctx, item.ID, StatusSucceeded, nil); err != nil {
				return err
			}
			item.Status = StatusSucceeded
			return w.store.ReleaseReleaseLock(ctx, item.ServiceName, item.Environment, item.ID)
		}
		if status.HasBlockingPodCondition() {
			detail := map[string]interface{}{
				"namespace":  environment.Kubernetes.Namespace,
				"deployment": environment.Kubernetes.Deployment,
				"pods":       status.Pods,
				"events":     status.Events,
				"conditions": status.Conditions,
			}
			return w.fail(ctx, item.ID, StatusFailed, "deployment rollout blocked", nil, detail)
		}
		if time.Now().After(deadline) {
			return w.fail(ctx, item.ID, StatusTimeout, "deployment rollout timeout", nil, map[string]interface{}{
				"namespace":  environment.Kubernetes.Namespace,
				"deployment": environment.Kubernetes.Deployment,
				"pods":       status.Pods,
				"events":     status.Events,
			})
		}
		if err := w.store.AddEvent(ctx, item.ID, StatusRolloutChecking, "deployment still rolling out", map[string]interface{}{
			"namespace":          environment.Kubernetes.Namespace,
			"deployment":         environment.Kubernetes.Deployment,
			"ready_replicas":     status.ReadyReplicas,
			"updated_replicas":   status.UpdatedReplicas,
			"available_replicas": status.AvailableReplicas,
		}); err != nil {
			return err
		}
		timer := time.NewTimer(w.rolloutPollInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func (w *Worker) transition(ctx context.Context, releaseID string, status Status, message string, detail map[string]interface{}) error {
	if err := w.store.ExtendReleaseLock(ctx, releaseID, w.lockTTL); err != nil {
		return err
	}
	if err := w.store.UpdateReleaseStatus(ctx, releaseID, status, nil); err != nil {
		return err
	}
	return w.store.AddEvent(ctx, releaseID, status, message, detail)
}

func (w *Worker) fail(ctx context.Context, releaseID string, status Status, message string, cause error, detail map[string]interface{}) error {
	if detail == nil {
		detail = map[string]interface{}{}
	}
	if cause != nil {
		detail["error"] = cause.Error()
	}
	errMessage := message
	if cause != nil {
		errMessage = fmt.Sprintf("%s: %v", message, cause)
	}
	if err := w.store.ExtendReleaseLock(ctx, releaseID, w.lockTTL); err != nil {
		return err
	}
	if err := w.store.UpdateReleaseStatus(ctx, releaseID, status, &errMessage); err != nil {
		return err
	}
	if err := w.store.AddEvent(ctx, releaseID, status, message, detail); err != nil {
		return err
	}
	if IsTerminal(status) {
		item, err := w.store.GetRelease(ctx, releaseID)
		if err != nil {
			return err
		}
		return w.store.ReleaseReleaseLock(ctx, item.ServiceName, item.Environment, item.ID)
	}
	return nil
}

func (w *Worker) resolveBuildNumber(ctx context.Context, item *Release, environment catalog.Environment) (int, error) {
	if item.JenkinsBuildNumber != nil {
		return *item.JenkinsBuildNumber, nil
	}
	events, err := w.store.ListEvents(ctx, item.ID)
	if err == nil {
		for i := len(events) - 1; i >= 0; i-- {
			event := events[i]
			if event.Status == StatusJenkinsTriggered || event.Status == StatusJenkinsRunning {
				if value, ok := event.Detail["build_number"]; ok {
					switch typed := value.(type) {
					case float64:
						number := int(typed)
						return number, nil
					case int:
						return typed, nil
					}
				}
			}
		}
	}
	job := environment.Jenkins.JobName
	if item.JenkinsJob != nil && *item.JenkinsJob != "" {
		job = *item.JenkinsJob
	}
	return 0, fmt.Errorf("release %s missing Jenkins build number for job %s", item.ID, job)
}

func normalizeJenkinsStatus(value string) string {
	switch strings.ToUpper(value) {
	case "SUCCESS", "FAILURE", "ABORTED":
		return strings.ToUpper(value)
	case "", "RUNNING", "BUILDING":
		return "RUNNING"
	default:
		return "RUNNING"
	}
}

func mustBuildLog(client JenkinsAdapter, jobName string, buildNumber int) string {
	log, err := client.GetBuildLog(jobName, buildNumber)
	if err != nil {
		return err.Error()
	}
	return log
}

type workerContextKeyReleaseID struct{}

func contextErr(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return nil
}
