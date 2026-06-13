package api

import (
	"github.com/gin-gonic/gin"
	"github.com/worryyy/devops-platform/platform/server/internal/pkg/responses"
	"github.com/worryyy/devops-platform/platform/server/internal/release"
)

type ServiceHandler struct {
	releases *release.Service
}

func (h ServiceHandler) List(c *gin.Context) {
	catalog := h.releases.Catalog()
	responses.Success.RespData(c, servicesResponse{Version: catalog.Version, Services: catalog.Services})
}

func (h ServiceHandler) Get(c *gin.Context) {
	service, err := h.releases.GetService(c.Param("name"))
	if err != nil {
		responses.Fail(c, err)
		return
	}
	responses.Success.RespData(c, service)
}

func (h ServiceHandler) Status(c *gin.Context) {
	service, err := h.releases.GetService(c.Param("name"))
	if err != nil {
		responses.Fail(c, err)
		return
	}

	releases, err := h.releases.ListReleasesByService(c.Request.Context(), service.Name, 50)
	if err != nil {
		responses.Fail(c, err)
		return
	}

	response := serviceStatusResponse{
		Service:      service.Name,
		Environments: make([]environmentStatusResponse, 0, len(service.Environments)),
	}
	for _, environment := range service.Environments {
		status := environmentStatusResponse{
			Name:       environment.Name,
			Namespace:  environment.Namespace,
			JenkinsJob: environment.Jenkins.JobName,
			ArgoCDApp:  environment.ArgoCD.Application,
			Deployment: environment.Kubernetes.Deployment,
		}
		for i := range releases {
			if releases[i].Environment == environment.Name {
				status.LatestRelease = &releases[i]
				break
			}
		}
		response.Environments = append(response.Environments, status)
	}
	responses.Success.RespData(c, response)
}
