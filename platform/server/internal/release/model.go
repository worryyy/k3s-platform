package release

import "time"

type Release struct {
	ID          string `json:"id"`
	ServiceName string `json:"service_name"`
	Environment string `json:"environment"`
	Branch      string `json:"branch"`

	Status   Status `json:"status"`
	Operator string `json:"operator"`

	JenkinsJob         *string `json:"jenkins_job,omitempty"`
	JenkinsBuildNumber *int    `json:"jenkins_build_number,omitempty"`

	CommitSHA   *string `json:"commit_sha,omitempty"`
	ImageRepo   *string `json:"image_repo,omitempty"`
	ImageTag    *string `json:"image_tag,omitempty"`
	ImageDigest *string `json:"image_digest,omitempty"`

	ArgoCDApp  *string `json:"argocd_app,omitempty"`
	Namespace  *string `json:"namespace,omitempty"`
	Deployment *string `json:"deployment,omitempty"`

	ErrorMessage *string `json:"error_message,omitempty"`

	StartedAt  *time.Time `json:"started_at,omitempty"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Event struct {
	ID        int64                  `json:"id"`
	ReleaseID string                 `json:"release_id"`
	Status    Status                 `json:"status"`
	Message   string                 `json:"message"`
	Detail    map[string]interface{} `json:"detail,omitempty"`
	CreatedAt time.Time              `json:"created_at"`
}

type CreateReleaseInput struct {
	ID          string
	ServiceName string
	Environment string
	Branch      string
	Operator    string

	JenkinsJob string
	ImageRepo  string
	ArgoCDApp  string
	Namespace  string
	Deployment string
}
