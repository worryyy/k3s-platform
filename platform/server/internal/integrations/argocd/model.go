package argocd

type Application struct {
	Name           string `json:"name"`
	Namespace      string `json:"namespace"`
	SyncStatus     string `json:"sync_status"`
	HealthStatus   string `json:"health_status"`
	Revision       string `json:"revision"`
	OperationPhase string `json:"operation_phase"`
}

func (a Application) IsHealthyAndSynced() bool {
	return a.SyncStatus == "Synced" && a.HealthStatus == "Healthy"
}

func (a Application) IsDegraded() bool {
	return a.HealthStatus == "Degraded"
}

func (a Application) OperationFailed() bool {
	return a.OperationPhase == "Failed" || a.OperationPhase == "Error"
}
