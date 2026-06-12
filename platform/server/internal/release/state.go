package release

type Status string

const (
	StatusRequested        Status = "Requested"
	StatusValidated        Status = "Validated"
	StatusQueued           Status = "Queued"
	StatusJenkinsTriggered Status = "JenkinsTriggered"
	StatusJenkinsRunning   Status = "JenkinsRunning"
	StatusGitOpsUpdated    Status = "GitOpsUpdated"
	StatusArgoSyncing      Status = "ArgoSyncing"
	StatusRolloutChecking  Status = "RolloutChecking"
	StatusSucceeded        Status = "Succeeded"
	StatusFailed           Status = "Failed"
	StatusTimeout          Status = "Timeout"
	StatusCanceled         Status = "Canceled"
)

func IsTerminal(status Status) bool {
	switch status {
	case StatusSucceeded, StatusFailed, StatusTimeout, StatusCanceled:
		return true
	default:
		return false
	}
}
