package store

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"github.com/worryyy/devops-platform/platform/server/internal/release"
)

type releaseRecord struct {
	ID          string `gorm:"column:id;primaryKey"`
	ServiceName string `gorm:"column:service_name"`
	Environment string `gorm:"column:environment"`
	Branch      string `gorm:"column:branch"`

	Status   string `gorm:"column:status"`
	Operator string `gorm:"column:operator"`

	JenkinsJob         *string `gorm:"column:jenkins_job"`
	JenkinsBuildNumber *int    `gorm:"column:jenkins_build_number"`

	CommitSHA   *string `gorm:"column:commit_sha"`
	ImageRepo   *string `gorm:"column:image_repo"`
	ImageTag    *string `gorm:"column:image_tag"`
	ImageDigest *string `gorm:"column:image_digest"`

	ArgoCDApp  *string `gorm:"column:argocd_app"`
	Namespace  *string `gorm:"column:namespace"`
	Deployment *string `gorm:"column:deployment"`

	ErrorMessage *string `gorm:"column:error_message"`

	StartedAt  *time.Time `gorm:"column:started_at"`
	FinishedAt *time.Time `gorm:"column:finished_at"`

	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

func (releaseRecord) TableName() string {
	return "releases"
}

func newReleaseRecord(input release.CreateReleaseInput) releaseRecord {
	now := time.Now().UTC()
	return releaseRecord{
		ID:          input.ID,
		ServiceName: input.ServiceName,
		Environment: input.Environment,
		Branch:      input.Branch,
		Status:      string(release.StatusRequested),
		Operator:    input.Operator,
		JenkinsJob:  stringPtrOrNil(input.JenkinsJob),
		ImageRepo:   stringPtrOrNil(input.ImageRepo),
		ArgoCDApp:   stringPtrOrNil(input.ArgoCDApp),
		Namespace:   stringPtrOrNil(input.Namespace),
		Deployment:  stringPtrOrNil(input.Deployment),
		StartedAt:   &now,
	}
}

func (r releaseRecord) toRelease() release.Release {
	return release.Release{
		ID:                 r.ID,
		ServiceName:        r.ServiceName,
		Environment:        r.Environment,
		Branch:             r.Branch,
		Status:             release.Status(r.Status),
		Operator:           r.Operator,
		JenkinsJob:         r.JenkinsJob,
		JenkinsBuildNumber: r.JenkinsBuildNumber,
		CommitSHA:          r.CommitSHA,
		ImageRepo:          r.ImageRepo,
		ImageTag:           r.ImageTag,
		ImageDigest:        r.ImageDigest,
		ArgoCDApp:          r.ArgoCDApp,
		Namespace:          r.Namespace,
		Deployment:         r.Deployment,
		ErrorMessage:       r.ErrorMessage,
		StartedAt:          r.StartedAt,
		FinishedAt:         r.FinishedAt,
		CreatedAt:          r.CreatedAt,
		UpdatedAt:          r.UpdatedAt,
	}
}

type releaseEventRecord struct {
	ID        int64     `gorm:"column:id;primaryKey"`
	ReleaseID string    `gorm:"column:release_id"`
	Status    string    `gorm:"column:status"`
	Message   string    `gorm:"column:message"`
	Detail    jsonBytes `gorm:"column:detail;type:jsonb"`
	CreatedAt time.Time `gorm:"column:created_at"`
}

func (releaseEventRecord) TableName() string {
	return "release_events"
}

func (r releaseEventRecord) toEvent() (release.Event, error) {
	event := release.Event{
		ID:        r.ID,
		ReleaseID: r.ReleaseID,
		Status:    release.Status(r.Status),
		Message:   r.Message,
		CreatedAt: r.CreatedAt,
	}
	if len(r.Detail) > 0 {
		if err := json.Unmarshal(r.Detail, &event.Detail); err != nil {
			return release.Event{}, fmt.Errorf("decode release event detail: %w", err)
		}
	}
	return event, nil
}

type releaseLockRecord struct {
	ServiceName string `gorm:"column:service_name;primaryKey"`
	Environment string `gorm:"column:environment;primaryKey"`

	ReleaseID   string    `gorm:"column:release_id"`
	LockedUntil time.Time `gorm:"column:locked_until"`
	CreatedAt   time.Time `gorm:"column:created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at"`
}

func (releaseLockRecord) TableName() string {
	return "release_locks"
}

type jsonBytes []byte

func (j jsonBytes) Value() (driver.Value, error) {
	if len(j) == 0 {
		return nil, nil
	}
	if !json.Valid(j) {
		return nil, fmt.Errorf("invalid json payload")
	}
	return string(j), nil
}

func (j *jsonBytes) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}
	switch data := value.(type) {
	case []byte:
		*j = append((*j)[:0], data...)
	case string:
		*j = append((*j)[:0], data...)
	default:
		return fmt.Errorf("unsupported jsonb value type %T", value)
	}
	return nil
}

func stringPtrOrNil(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
