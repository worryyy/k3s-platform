package queue

type ReleaseMessage struct {
	ReleaseID   string `json:"release_id"`
	Service     string `json:"service"`
	Environment string `json:"environment"`
	Event       string `json:"event"`
}

const EventReleaseRequested = "ReleaseRequested"
