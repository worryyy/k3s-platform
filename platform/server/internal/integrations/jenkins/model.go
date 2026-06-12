package jenkins

type BuildStatus string

const (
	BuildRunning BuildStatus = "RUNNING"
	BuildSuccess BuildStatus = "SUCCESS"
	BuildFailure BuildStatus = "FAILURE"
	BuildAborted BuildStatus = "ABORTED"
)
