package api

import (
	"time"

	"github.com/worryyy/devops-platform/platform/server/internal/catalog"
	"github.com/worryyy/devops-platform/platform/server/internal/release"
)

type createReleaseRequest struct {
	Service     string `json:"service" binding:"required"`
	Environment string `json:"environment" binding:"required"`
	Branch      string `json:"branch" binding:"required"`
	Operator    string `json:"operator" binding:"required"`
}

type createReleaseResponse struct {
	ReleaseID string         `json:"release_id"`
	Status    release.Status `json:"status"`
}

type serviceStatusResponse struct {
	Service      string                      `json:"service"`
	Environments []environmentStatusResponse `json:"environments"`
}

type environmentStatusResponse struct {
	Name          string           `json:"name"`
	Namespace     string           `json:"namespace"`
	JenkinsJob    string           `json:"jenkins_job"`
	ArgoCDApp     string           `json:"argocd_app"`
	Deployment    string           `json:"deployment"`
	LatestRelease *release.Release `json:"latest_release,omitempty"`
}

type readinessResponse struct {
	Status string `json:"status"`
}

type servicesResponse struct {
	Version  string            `json:"version"`
	Services []catalog.Service `json:"services"`
}

type healthResponse struct {
	Status string    `json:"status"`
	Time   time.Time `json:"time"`
}
