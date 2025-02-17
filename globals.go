package evergreen

import (
	"os"
	"strings"
	"time"

	"github.com/evergreen-ci/utility"
	"github.com/mongodb/grip"
	"github.com/pkg/errors"
)

const (
	User            = "mci"
	GithubPatchUser = "github_pull_request"
	ParentPatchUser = "parent_patch"

	HostRunning       = "running"
	HostTerminated    = "terminated"
	HostUninitialized = "initializing"
	// HostBuilding is an intermediate state indicating that the intent host is
	// attempting to create a real host from an intent host, but has not
	// successfully done so yet.
	HostBuilding = "building"
	// HostBuildingFailed is a failure state indicating that an intent host was
	// attempting to create a host but failed during creation. Hosts that fail
	// to build will terminate shortly.
	HostBuildingFailed = "building-failed"
	HostStarting       = "starting"
	HostProvisioning   = "provisioning"
	// HostProvisionFailed is a failure state indicating that a host was
	// successfully created (i.e. requested from the cloud provider) but failed
	// while it was starting up. Hosts that fail to provisoin will terminate
	// shortly.
	HostProvisionFailed = "provision failed"
	HostQuarantined     = "quarantined"
	HostDecommissioned  = "decommissioned"

	HostStopping = "stopping"
	HostStopped  = "stopped"

	HostExternalUserName = "external"

	// Task statuses stored in the database (i.e. (Task).Status):

	// TaskUndispatched indicates either:
	//  1. a task is not scheduled to run (when Task.Activated == false)
	//  2. a task is scheduled to run (when Task.Activated == true)
	TaskUndispatched = "undispatched"

	// TaskDispatched indicates that an agent has received the task, but
	// the agent has not yet told Evergreen that it's running the task
	TaskDispatched = "dispatched"

	// TaskStarted indicates a task is running on an agent.
	TaskStarted = "started"

	// The task statuses below indicate that a task has finished.
	// TaskSucceeded indicates that the task has finished and is successful.
	TaskSucceeded = "success"

	// TaskFailed indicates that the task has finished and failed. This
	// encompasses any task failure, regardless of the specific failure reason
	// which can be found in the task end details.
	TaskFailed = "failed"

	// Task statuses used for the UI or other special-case purposes:

	// TaskUnscheduled indicates that the task is undispatched and is not
	// scheduled to eventually run. This is a display status, so it's only used
	// in the UI.
	TaskUnscheduled = "unscheduled"
	// TaskInactive is a deprecated legacy status that used to mean that the
	// task was not scheduled to run. This is equivalent to the TaskUnscheduled
	// display status. These are not stored in the task status (although they
	// used to be for very old tasks) but may be still used in some outdated
	// pieces of code.
	TaskInactive = "inactive"

	// TaskWillRun indicates that the task is scheduled to eventually run,
	// unless one of its dependencies becomes unattainable. This is a display
	// status, so it's only used in the UI.
	TaskWillRun = "will-run"

	// All other task failure reasons other than TaskFailed are display
	// statuses, so they're only used in the UI. These are not stored in the
	// task status (although they used to be for very old tasks).
	TaskSystemFailed = "system-failed"
	TaskTestTimedOut = "test-timed-out"
	TaskSetupFailed  = "setup-failed"

	// TaskAborted indicates that the task was aborted while it was running.
	TaskAborted = "aborted"

	// TaskStatusBlocked indicates that the task cannot run because it is
	// blocked by an unattainable dependency. This is a display status, so it's
	// only used in the UI.
	TaskStatusBlocked = "blocked"

	// TaskKnownIssue indicates that the task has failed and is being tracked by
	// a linked issue in the task annotations. This is a display status, so it's
	// only used in the UI.
	TaskKnownIssue = "known-issue"

	// TaskStatusPending is a special state that's used for one specific return
	// value. Generally do not use this status as it is neither a meaningful
	// status in the UI nor in the back end.
	TaskStatusPending = "pending"

	// TaskAll is not a status, but rather a UI filter indicating that it should
	// select all tasks regardless of their status.
	TaskAll = "all"

	// Task Command Types
	CommandTypeTest   = "test"
	CommandTypeSystem = "system"
	CommandTypeSetup  = "setup"

	// Task descriptions
	//
	// TaskDescriptionHeartbeat indicates that a task failed because it did not
	// send a heartbeat while it was running. Tasks are expected to send
	// periodic heartbeats back to the app server indicating the task is still
	// actively running, or else they are considered stale.
	TaskDescriptionHeartbeat = "heartbeat"
	// TaskDescriptionStranded indicates that a task failed because its
	// underlying runtime environment (i.e. container or host) encountered an
	// issue. For example, if a host is terminated while the task is still
	// running, the task is considered stranded.
	TaskDescriptionStranded  = "stranded"
	TaskDescriptionNoResults = "expected test results, but none attached"
	// TaskDescriptionContainerUnallocatable indicates that the reason a
	// container task failed is because it cannot be allocated a container.
	TaskDescriptionContainerUnallocatable = "container task cannot be allocated"
	// TaskDescriptionAborted indicates that the reason a task failed is specifically
	// because it was manually aborted.
	TaskDescriptionAborted = "aborted"

	// Task Statuses that are currently used only by the UI, and in tests
	// (these may be used in old tasks as actual task statuses rather than just
	// task display statuses).
	TaskSystemUnresponse = "system-unresponsive"
	TaskSystemTimedOut   = "system-timed-out"
	TaskTimedOut         = "task-timed-out"

	// TaskConflict is used only in communication with the Agent
	TaskConflict = "task-conflict"

	TestFailedStatus         = "fail"
	TestSilentlyFailedStatus = "silentfail"
	TestSkippedStatus        = "skip"
	TestSucceededStatus      = "pass"

	BuildStarted   = "started"
	BuildCreated   = "created"
	BuildFailed    = "failed"
	BuildSucceeded = "success"

	VersionStarted   = "started"
	VersionCreated   = "created"
	VersionFailed    = "failed"
	VersionSucceeded = "success"

	PatchCreated     = "created"
	PatchStarted     = "started"
	PatchSucceeded   = "succeeded"
	PatchFailed      = "failed"
	PatchAllOutcomes = "*"

	// VersionAborted and PatchAborted are display statuses only and not stored in the DB
	VersionAborted = "aborted"
	PatchAborted   = "aborted"

	PushLogPushing = "pushing"
	PushLogSuccess = "success"

	HostTypeStatic = "static"

	MergeTestStarted   = "started"
	MergeTestSucceeded = "succeeded"
	MergeTestFailed    = "failed"
	EnqueueFailed      = "failed to enqueue"

	// maximum task (zero based) execution number
	MaxTaskExecution = 9

	// maximum task priority
	MaxTaskPriority = 100

	DisabledTaskPriority = int64(-1)

	// if a patch has NumTasksForLargePatch number of tasks or greater, we log to splunk for investigation
	NumTasksForLargePatch = 10000

	// LogMessage struct versions
	LogmessageFormatTimestamp = 1
	LogmessageCurrentVersion  = LogmessageFormatTimestamp

	DefaultEvergreenConfig = ".evergreen.yml"

	EvergreenHome   = "EVGHOME"
	MongodbUrl      = "MONGO_URL"
	MongodbAuthFile = "MONGO_CREDS_FILE"

	// Special logging output targets
	LocalLoggingOverride          = "LOCAL"
	StandardOutputLoggingOverride = "STDOUT"

	DefaultTaskActivator   = ""
	StepbackTaskActivator  = "stepback"
	APIServerTaskActivator = "apiserver"

	// StaleContainerTaskMonitor is the special name representing the unit
	// responsible for monitoring container tasks that have not dispatched but
	// have waiting for a long time since their activation.
	StaleContainerTaskMonitor = "stale-container-task-monitor"

	// Restart Types
	RestartVersions = "versions"
	RestartTasks    = "tasks"

	RestRoutePrefix = "rest"
	APIRoutePrefix  = "api"

	AgentAPIVersion  = 2
	APIRoutePrefixV2 = "/rest/v2"

	AgentMonitorTag = "agent-monitor"
	HostFetchTag    = "host-fetch"

	DegradedLoggingPercent = 10

	SetupScriptName               = "setup.sh"
	TempSetupScriptName           = "setup-temp.sh"
	PowerShellSetupScriptName     = "setup.ps1"
	PowerShellTempSetupScriptName = "setup-temp.ps1"

	RoutePaginatorNextPageHeaderKey = "Link"

	PlannerVersionLegacy  = "legacy"
	PlannerVersionTunable = "tunable"

	// TODO: EVG-18706 all distros use DispatcherVersionRevisedWithDependencies, we may be able to remove these and their custom logic
	DispatcherVersionLegacy                  = "legacy"
	DispatcherVersionRevised                 = "revised"
	DispatcherVersionRevisedWithDependencies = "revised-with-dependencies"

	// maximum turnaround we want to maintain for all hosts for a given distro
	MaxDurationPerDistroHost               = 30 * time.Minute
	MaxDurationPerDistroHostWithContainers = 2 * time.Minute

	// Spawn hosts
	SpawnHostExpireDays                 = 30
	HostExpireDays                      = 10
	ExpireOnFormat                      = "2006-01-02"
	DefaultMaxSpawnHostsPerUser         = 3
	DefaultSpawnHostExpiration          = 24 * time.Hour
	SpawnHostRespawns                   = 2
	SpawnHostNoExpirationDuration       = 7 * 24 * time.Hour
	MaxSpawnHostExpirationDurationHours = 24 * time.Hour * 14
	UnattachedVolumeExpiration          = 24 * time.Hour * 30
	DefaultMaxVolumeSizePerUser         = 500
	DefaultUnexpirableHostsPerUser      = 1
	DefaultUnexpirableVolumesPerUser    = 1

	// host resource tag names
	TagName             = "name"
	TagDistro           = "distro"
	TagEvergreenService = "evergreen-service"
	TagUsername         = "username"
	TagOwner            = "owner"
	TagMode             = "mode"
	TagStartTime        = "start-time"
	TagExpireOn         = "expire-on"

	FinderVersionLegacy    = "legacy"
	FinderVersionParallel  = "parallel"
	FinderVersionPipeline  = "pipeline"
	FinderVersionAlternate = "alternate"

	HostAllocatorDeficit     = "deficit"
	HostAllocatorUtilization = "utilization"

	HostAllocatorRoundDown    = "round-down"
	HostAllocatorRoundUp      = "round-up"
	HostAllocatorRoundDefault = ""

	HostAllocatorWaitsOverThreshFeedback = "waits-over-thresh-feedback"
	HostAllocatorNoFeedback              = "no-feedback"
	HostAllocatorUseDefaultFeedback      = ""

	HostsOverallocatedTerminate  = "terminate-hosts-when-overallocated"
	HostsOverallocatedIgnore     = "no-terminations-when-overallocated"
	HostsOverallocatedUseDefault = ""

	// CommitQueueAlias and GithubPRAlias are special aliases to specify variants and tasks for commit queue and GitHub PR patches
	CommitQueueAlias  = "__commit_queue"
	GithubPRAlias     = "__github"
	GithubChecksAlias = "__github_checks"
	GitTagAlias       = "__git_tag"

	MergeTaskVariant = "commit-queue-merge"
	MergeTaskName    = "merge-patch"
	MergeTaskGroup   = "merge-task-group"

	DefaultJasperPort = 2385

	GlobalGitHubTokenExpansion = "global_github_oauth_token"
	githubAppPrivateKey        = "github_app_private_key"

	VSCodePort = 2021

	// DefaultTaskSyncAtEndTimeout is the default timeout for task sync at the
	// end of a patch.
	DefaultTaskSyncAtEndTimeout = time.Hour

	DefaultShutdownWaitSeconds = 10

	SaveGenerateTasksError     = "error saving config in `generate.tasks`"
	TasksAlreadyGeneratedError = "generator already ran and generated tasks"
	KeyTooLargeToIndexError    = "key too large to index"
	InvalidDivideInputError    = "$divide only supports numeric types"

	// Valid types of performing git clone
	CloneMethodLegacySSH = "legacy-ssh"
	CloneMethodOAuth     = "oauth"

	// ContainerHealthDashboard is the name of the Splunk dashboard that displays
	// charts relating to the health of container tasks.
	ContainerHealthDashboard = "container task health dashboard"

	// PRTasksRunningDescription is the description for a GitHub PR status
	// indicating that there are still running tasks.
	PRTasksRunningDescription = "tasks are running"
)

var TaskStatuses = []string{
	TaskStarted,
	TaskSucceeded,
	TaskFailed,
	TaskSystemFailed,
	TaskTestTimedOut,
	TaskSetupFailed,
	TaskAborted,
	TaskStatusBlocked,
	TaskStatusPending,
	TaskKnownIssue,
	TaskSystemUnresponse,
	TaskSystemTimedOut,
	TaskTimedOut,
	TaskWillRun,
	TaskUnscheduled,
	TaskUndispatched,
	TaskDispatched,
}

var InternalAliases = []string{
	CommitQueueAlias,
	GithubPRAlias,
	GithubChecksAlias,
	GitTagAlias,
}

// TaskNonGenericFailureStatuses represents some kind of specific abnormal
// failure mode. These are display statuses used in the UI.
var TaskNonGenericFailureStatuses = []string{
	TaskTimedOut,
	TaskSystemFailed,
	TaskTestTimedOut,
	TaskSetupFailed,
	TaskSystemUnresponse,
	TaskSystemTimedOut,
}

var TaskSystemFailures = []string{
	TaskSystemFailed,
	TaskTimedOut,
	TaskSystemUnresponse,
	TaskSystemTimedOut,
	TaskTestTimedOut,
}

// TaskFailureStatuses represent all the ways that a completed task can fail,
// inclusive of display statuses such as system failures.
var TaskFailureStatuses = append([]string{TaskFailed}, TaskNonGenericFailureStatuses...)

var TaskUnstartedStatuses = []string{
	TaskInactive,
	TaskUndispatched,
}

func IsUnstartedTaskStatus(status string) bool {
	return utility.StringSliceContains(TaskUnstartedStatuses, status)
}

func IsFinishedTaskStatus(status string) bool {
	if status == TaskSucceeded ||
		IsFailedTaskStatus(status) {
		return true
	}

	return false
}

func IsFailedTaskStatus(status string) bool {
	return utility.StringSliceContains(TaskFailureStatuses, status)
}

func IsSystemFailedTaskStatus(status string) bool {
	return utility.StringSliceContains(TaskSystemFailures, status)
}

func IsValidTaskEndStatus(status string) bool {
	return status == TaskSucceeded || status == TaskFailed
}

func IsFinishedPatchStatus(status string) bool {
	return status == PatchFailed || status == PatchSucceeded
}

func IsFinishedBuildStatus(status string) bool {
	return status == BuildFailed || status == BuildSucceeded
}

func IsFinishedVersionStatus(status string) bool {
	return status == VersionFailed || status == VersionSucceeded
}

func VersionStatusToPatchStatus(versionStatus string) (string, error) {
	switch versionStatus {
	case VersionCreated:
		return PatchCreated, nil
	case VersionStarted:
		return PatchStarted, nil
	case VersionFailed:
		return PatchFailed, nil
	case VersionSucceeded:
		return PatchSucceeded, nil
	default:
		return "", errors.Errorf("unknown version status: %s", versionStatus)
	}
}

func PatchStatusToVersionStatus(patchStatus string) (string, error) {
	switch patchStatus {
	case PatchCreated:
		return VersionCreated, nil
	case PatchStarted:
		return VersionStarted, nil
	case PatchFailed:
		return VersionFailed, nil
	case PatchSucceeded:
		return VersionSucceeded, nil
	case PatchAborted:
		return VersionAborted, nil
	default:
		return "", errors.Errorf("unknown patch status: %s", patchStatus)
	}
}

type ModificationAction string

// Common OTEL attribute keys
const (
	TaskIDOtelAttribute            = "evergreen.task.id"
	TaskNameOtelAttribute          = "evergreen.task.name"
	TaskExecutionOtelAttribute     = "evergreen.task.execution"
	TaskStatusOtelAttribute        = "evergreen.task.status"
	VersionIDOtelAttribute         = "evergreen.version.id"
	VersionRequesterOtelAttribute  = "evergreen.version.requester"
	BuildIDOtelAttribute           = "evergreen.build.id"
	BuildNameOtelAttribute         = "evergreen.build.name"
	ProjectIdentifierOtelAttribute = "evergreen.project.identifier"
	ProjectIDOtelAttribute         = "evergreen.project.id"
	DistroIDOtelAttribute          = "evergreen.distro.id"
)

const (
	RestartAction     ModificationAction = "restart"
	SetActiveAction   ModificationAction = "set_active"
	SetPriorityAction ModificationAction = "set_priority"
	AbortAction       ModificationAction = "abort"
)

// Constants for Evergreen package names (including legacy ones).
const (
	UIPackage      = "EVERGREEN_UI"
	RESTV2Package  = "EVERGREEN_REST_V2"
	MonitorPackage = "EVERGREEN_MONITOR"
)

const (
	AuthTokenCookie     = "mci-token"
	TaskHeader          = "Task-Id"
	TaskSecretHeader    = "Task-Secret"
	HostHeader          = "Host-Id"
	HostSecretHeader    = "Host-Secret"
	PodHeader           = "Pod-Id"
	PodSecretHeader     = "Pod-Secret"
	ContentTypeHeader   = "Content-Type"
	ContentTypeValue    = "application/json"
	ContentLengthHeader = "Content-Length"
	APIUserHeader       = "Api-User"
	APIKeyHeader        = "Api-Key"
)

const (
	// CredentialsCollection is the collection containing TLS credentials to
	// connect to a Jasper service running on a host.
	CredentialsCollection = "credentials"
	// CAName is the name of the root CA for the TLS credentials.
	CAName = "evergreen"
)

// Constants related to cloud providers and provider-specific settings.
const (
	ProviderNameEc2OnDemand = "ec2-ondemand"
	ProviderNameEc2Fleet    = "ec2-fleet"
	ProviderNameDocker      = "docker"
	ProviderNameDockerMock  = "docker-mock"
	ProviderNameGce         = "gce"
	ProviderNameStatic      = "static"
	ProviderNameOpenstack   = "openstack"
	ProviderNameVsphere     = "vsphere"
	ProviderNameMock        = "mock"

	// DefaultEC2Region is the default region where hosts should be spawned.
	DefaultEC2Region = "us-east-1"
	// DefaultEBSType is Amazon's default EBS type.
	DefaultEBSType = "gp3"
	// DefaultEBSAvailabilityZone is the default availability zone for EBS
	// volumes. This may be a temporary default.
	DefaultEBSAvailabilityZone = "us-east-1a"
)

// IsEc2Provider returns true if the provider is ec2.
func IsEc2Provider(provider string) bool {
	return provider == ProviderNameEc2OnDemand ||
		provider == ProviderNameEc2Fleet
}

// IsDockerProvider returns true if the provider is docker.
func IsDockerProvider(provider string) bool {
	return provider == ProviderNameDocker ||
		provider == ProviderNameDockerMock
}

var (
	// ProviderSpawnable includes all cloud provider types where hosts can be
	// dynamically created and terminated according to need. This has no
	// relation to spawn hosts.
	ProviderSpawnable = []string{
		ProviderNameEc2OnDemand,
		ProviderNameEc2Fleet,
		ProviderNameGce,
		ProviderNameOpenstack,
		ProviderNameVsphere,
		ProviderNameMock,
		ProviderNameDocker,
	}

	// ProviderUserSpawnable includes all cloud provider types where a user can
	// request a dynamically created host for purposes such as host.create and
	// spawn hosts.
	ProviderUserSpawnable = []string{
		ProviderNameEc2OnDemand,
		ProviderNameEc2Fleet,
		ProviderNameGce,
		ProviderNameOpenstack,
		ProviderNameVsphere,
	}

	ProviderContainer = []string{
		ProviderNameDocker,
	}

	// ProviderSpotEc2Type includes all cloud provider types that manage EC2
	// spot instances.
	ProviderSpotEc2Type = []string{
		ProviderNameEc2Fleet,
	}

	// ProviderEc2Type includes all cloud provider types that manage EC2
	// instances.
	ProviderEc2Type = []string{
		ProviderNameEc2Fleet,
		ProviderNameEc2OnDemand,
	}
)

const (
	DefaultServiceConfigurationFileName = "/etc/mci_settings.yml"
	DefaultDatabaseURL                  = "mongodb://localhost:27017"
	DefaultDatabaseName                 = "mci"
	DefaultDatabaseWriteMode            = "majority"
	DefaultDatabaseReadMode             = "majority"

	DefaultAmboyDatabaseURL = "mongodb://localhost:27017"

	// database and config directory, set to the testing version by default for safety
	NotificationsFile = "mci-notifications.yml"
	ClientDirectory   = "clients"

	// version requester types
	PatchVersionRequester       = "patch_request"
	GithubPRRequester           = "github_pull_request"
	GitTagRequester             = "git_tag_request"
	RepotrackerVersionRequester = "gitter_request"
	TriggerRequester            = "trigger_request"
	MergeTestRequester          = "merge_test"           // Evergreen commit queue
	AdHocRequester              = "ad_hoc"               // periodic build
	GithubMergeRequester        = "github_merge_request" // GitHub merge queue
)

var AllRequesterTypes = []string{
	PatchVersionRequester,
	GithubPRRequester,
	GitTagRequester,
	RepotrackerVersionRequester,
	TriggerRequester,
	MergeTestRequester,
	AdHocRequester,
	GithubMergeRequester,
}

// Constants related to requester types.
var (
	SystemVersionRequesterTypes = []string{
		RepotrackerVersionRequester,
		TriggerRequester,
		GitTagRequester,
		AdHocRequester,
	}
)

// Constants for project command names.
const (
	GenerateTasksCommandName      = "generate.tasks"
	HostCreateCommandName         = "host.create"
	S3PushCommandName             = "s3.push"
	S3PullCommandName             = "s3.pull"
	ShellExecCommandName          = "shell.exec"
	AttachResultsCommandName      = "attach.results"
	AttachArtifactsCommandName    = "attach.artifacts"
	AttachXUnitResultsCommandName = "attach.xunit_results"
)

var AttachCommands = []string{
	AttachResultsCommandName,
	AttachArtifactsCommandName,
	AttachXUnitResultsCommandName,
}

type SenderKey int

const (
	SenderGithubStatus = SenderKey(iota)
	SenderEvergreenWebhook
	SenderSlack
	SenderJIRAIssue
	SenderJIRAComment
	SenderEmail
	SenderGeneric
)

func (k SenderKey) Validate() error {
	switch k {
	case SenderGithubStatus, SenderEvergreenWebhook, SenderSlack, SenderJIRAComment, SenderJIRAIssue,
		SenderEmail, SenderGeneric:
		return nil
	default:
		return errors.New("invalid sender defined")
	}
}

func (k SenderKey) String() string {
	switch k {
	case SenderGithubStatus:
		return "github-status"
	case SenderEmail:
		return "email"
	case SenderEvergreenWebhook:
		return "webhook"
	case SenderSlack:
		return "slack"
	case SenderJIRAComment:
		return "jira-comment"
	case SenderJIRAIssue:
		return "jira-issue"
	case SenderGeneric:
		return "generic"
	default:
		return "<error:unknown>"
	}
}

// Recognized Evergreen agent CPU architectures, which should be in the form
// ${GOOS}_${GOARCH}.
const (
	ArchDarwinAmd64  = "darwin_amd64"
	ArchDarwinArm64  = "darwin_arm64"
	ArchLinux386     = "linux_386"
	ArchLinuxPpc64le = "linux_ppc64le"
	ArchLinuxS390x   = "linux_s390x"
	ArchLinuxArm64   = "linux_arm64"
	ArchLinuxAmd64   = "linux_amd64"
	ArchWindows386   = "windows_386"
	ArchWindowsAmd64 = "windows_amd64"
)

// NameTimeFormat is the format in which to log times like instance start time.
const NameTimeFormat = "20060102150405"

var (
	PatchRequesters = []string{
		PatchVersionRequester,
		GithubPRRequester,
		MergeTestRequester,
		GithubMergeRequester,
	}

	SystemActivators = []string{
		DefaultTaskActivator,
		APIServerTaskActivator,
	}

	// UpHostStatus is a list of all host statuses that are considered up.
	UpHostStatus = []string{
		HostRunning,
		HostUninitialized,
		HostBuilding,
		HostStarting,
		HostProvisioning,
		HostProvisionFailed,
		HostStopping,
		HostStopped,
	}

	StartedHostStatus = []string{
		HostBuilding,
		HostStarting,
	}

	// ProvisioningHostStatus describes hosts that have started,
	// but have not yet completed the provisioning process.
	ProvisioningHostStatus = []string{
		HostStarting,
		HostProvisioning,
		HostProvisionFailed,
		HostBuilding,
	}

	// DownHostStatus is a list of all host statuses that are considered down.
	DownHostStatus = []string{
		HostTerminated,
		HostQuarantined,
		HostDecommissioned,
	}

	// NotRunningStatus is a list of host statuses from before the host starts running.
	NotRunningStatus = []string{
		HostUninitialized,
		HostBuilding,
		HostProvisioning,
		HostStarting,
	}

	// Hosts in "initializing" status aren't actually running yet:
	// they're just intents, so this list omits that value.
	ActiveStatus = []string{
		HostRunning,
		HostBuilding,
		HostStarting,
		HostProvisioning,
		HostProvisionFailed,
		HostStopping,
		HostStopped,
	}

	// Set of host status values that can be user set via the API
	ValidUserSetHostStatus = []string{
		HostRunning,
		HostTerminated,
		HostQuarantined,
		HostDecommissioned,
	}

	// Set of valid PlannerSettings.Version strings that can be user set via the API
	ValidTaskPlannerVersions = []string{
		PlannerVersionLegacy,
		PlannerVersionTunable,
	}

	// Set of valid DispatchSettings.Version strings that can be user set via the API
	ValidTaskDispatcherVersions = []string{
		DispatcherVersionLegacy,
		DispatcherVersionRevised,
		DispatcherVersionRevisedWithDependencies,
	}

	// Set of valid FinderSettings.Version strings that can be user set via the API
	ValidTaskFinderVersions = []string{
		FinderVersionLegacy,
		FinderVersionParallel,
		FinderVersionPipeline,
		FinderVersionAlternate,
	}

	// Set of valid Host Allocators types
	ValidHostAllocators = []string{
		HostAllocatorUtilization,
	}

	ValidHostAllocatorRoundingRules = []string{
		HostAllocatorRoundDown,
		HostAllocatorRoundUp,
		HostAllocatorRoundDefault,
	}

	ValidDefaultHostAllocatorRoundingRules = []string{
		HostAllocatorRoundDown,
		HostAllocatorRoundUp,
	}
	ValidHostAllocatorFeedbackRules = []string{
		HostAllocatorWaitsOverThreshFeedback,
		HostAllocatorNoFeedback,
		HostAllocatorUseDefaultFeedback,
	}

	ValidDefaultHostAllocatorFeedbackRules = []string{
		HostAllocatorWaitsOverThreshFeedback,
		HostAllocatorNoFeedback,
	}

	ValidHostsOverallocatedRules = []string{
		HostsOverallocatedUseDefault,
		HostsOverallocatedIgnore,
		HostsOverallocatedTerminate,
	}

	ValidDefaultHostsOverallocatedRules = []string{
		HostsOverallocatedIgnore,
		HostsOverallocatedTerminate,
	}

	// TaskInProgressStatuses have been picked up by an agent but have not
	// finished running.
	TaskInProgressStatuses = []string{TaskStarted, TaskDispatched}
	// TaskCompletedStatuses are statuses for tasks that have finished running.
	// This does not include task display statuses.
	TaskCompletedStatuses = []string{TaskSucceeded, TaskFailed}
	// TaskUncompletedStatuses are all statuses that do not represent a finished state.
	TaskUncompletedStatuses = []string{
		TaskStarted,
		TaskUndispatched,
		TaskDispatched,
		TaskConflict,
		TaskInactive,
	}

	SyncStatuses = []string{TaskSucceeded, TaskFailed}

	ValidCommandTypes = []string{CommandTypeSetup, CommandTypeSystem, CommandTypeTest}

	// Map from valid architectures to display names
	ValidArchDisplayNames = map[string]string{
		ArchWindowsAmd64: "Windows 64-bit",
		ArchLinuxPpc64le: "Linux PowerPC 64-bit",
		ArchLinuxS390x:   "Linux zSeries",
		ArchLinuxArm64:   "Linux ARM 64-bit",
		ArchWindows386:   "Windows 32-bit",
		ArchDarwinAmd64:  "OSX 64-bit",
		ArchDarwinArm64:  "OSX ARM 64-bit",
		ArchLinuxAmd64:   "Linux 64-bit",
		ArchLinux386:     "Linux 32-bit",
	}

	// ValidCloneMethods includes all recognized clone methods.
	ValidCloneMethods = []string{
		CloneMethodLegacySSH,
		CloneMethodOAuth,
	}
)

// FindEvergreenHome finds the directory of the EVGHOME environment variable.
func FindEvergreenHome() string {
	// check if env var is set
	root := os.Getenv(EvergreenHome)
	if len(root) > 0 {
		return root
	}

	grip.Errorf("%s is unset", EvergreenHome)
	return ""
}

// IsSystemActivator returns true when the task activator is Evergreen.
func IsSystemActivator(caller string) bool {
	return utility.StringSliceContains(SystemActivators, caller)
}

func IsPatchRequester(requester string) bool {
	return requester == PatchVersionRequester || IsGitHubPatchRequester(requester)
}

func IsGitHubPatchRequester(requester string) bool {
	return requester == GithubPRRequester || requester == MergeTestRequester || requester == GithubMergeRequester
}

func IsGitTagRequester(requester string) bool {
	return requester == GitTagRequester
}

func IsCommitQueueRequester(requester string) bool {
	return requester == MergeTestRequester
}

func ShouldConsiderBatchtime(requester string) bool {
	return !IsPatchRequester(requester) && requester != AdHocRequester && requester != GitTagRequester
}

func PermissionsDisabledForTests() bool {
	return PermissionSystemDisabled
}

// Constants for permission scopes and resource types.
const (
	SuperUserResourceType = "super_user"
	ProjectResourceType   = "project"
	DistroResourceType    = "distro"

	AllProjectsScope          = "all_projects"
	UnrestrictedProjectsScope = "unrestricted_projects"
	RestrictedProjectsScope   = "restricted_projects"
	AllDistrosScope           = "all_distros"
)

type PermissionLevel struct {
	Description string `json:"description"`
	Value       int    `json:"value"`
}

var (
	UnauthedUserRoles  = []string{"unauthorized_project"}
	ValidResourceTypes = []string{SuperUserResourceType, ProjectResourceType, DistroResourceType}
	// SuperUserPermissions resource ID.
	SuperUserPermissionsID = "super_user"

	// Admin permissions.
	PermissionAdminSettings = "admin_settings"
	PermissionProjectCreate = "project_create"
	PermissionDistroCreate  = "distro_create"
	PermissionRoleModify    = "modify_roles"
	// Project permissions.
	PermissionProjectSettings  = "project_settings"
	PermissionProjectVariables = "project_variables"
	PermissionGitTagVersions   = "project_git_tags"
	PermissionTasks            = "project_tasks"
	PermissionAnnotations      = "project_task_annotations"
	PermissionPatches          = "project_patches"
	PermissionLogs             = "project_logs"
	// Distro permissions.
	PermissionDistroSettings = "distro_settings"
	PermissionHosts          = "distro_hosts"
)

// Constants related to permission levels.
var (
	AdminSettingsEdit = PermissionLevel{
		Description: "Edit admin settings",
		Value:       10,
	}
	ProjectCreate = PermissionLevel{
		Description: "Create new projects",
		Value:       10,
	}
	DistroCreate = PermissionLevel{
		Description: "Create new distros",
		Value:       10,
	}
	RoleModify = PermissionLevel{
		Description: "Modify system roles and permissions",
		Value:       10,
	}
	ProjectSettingsEdit = PermissionLevel{
		Description: "Edit project settings",
		Value:       20,
	}
	ProjectSettingsView = PermissionLevel{
		Description: "View project settings",
		Value:       10,
	}
	ProjectSettingsNone = PermissionLevel{
		Description: "No project settings permissions",
		Value:       0,
	}
	GitTagVersionsCreate = PermissionLevel{
		Description: "Create versions with git tags",
		Value:       10,
	}
	GitTagVersionsNone = PermissionLevel{
		Description: "Not able to create versions with git tags",
		Value:       0,
	}
	AnnotationsModify = PermissionLevel{
		Description: "Modify annotations",
		Value:       20,
	}
	AnnotationsView = PermissionLevel{
		Description: "View annotations",
		Value:       10,
	}
	AnnotationsNone = PermissionLevel{
		Description: "No annotations permissions",
		Value:       0,
	}
	TasksAdmin = PermissionLevel{
		Description: "Full tasks permissions",
		Value:       30,
	}
	TasksBasic = PermissionLevel{
		Description: "Basic modifications to tasks",
		Value:       20,
	}
	TasksView = PermissionLevel{
		Description: "View tasks",
		Value:       10,
	}
	TasksNone = PermissionLevel{
		Description: "Not able to view or edit tasks",
		Value:       0,
	}
	PatchSubmitAdmin = PermissionLevel{
		Description: "Submit/edit patches, and submit patches on behalf of users",
		Value:       20,
	}
	PatchSubmit = PermissionLevel{
		Description: "Submit and edit patches",
		Value:       10,
	}
	PatchNone = PermissionLevel{
		Description: "Not able to view or submit patches",
		Value:       0,
	}
	LogsView = PermissionLevel{
		Description: "View logs",
		Value:       10,
	}
	LogsNone = PermissionLevel{
		Description: "Not able to view logs",
		Value:       0,
	}
	DistroSettingsAdmin = PermissionLevel{
		Description: "Remove distro and edit distro settings",
		Value:       30,
	}
	DistroSettingsEdit = PermissionLevel{
		Description: "Edit distro settings",
		Value:       20,
	}
	DistroSettingsView = PermissionLevel{
		Description: "View distro settings",
		Value:       10,
	}
	DistroSettingsNone = PermissionLevel{
		Description: "No distro settings permissions",
		Value:       0,
	}
	HostsEdit = PermissionLevel{
		Description: "Edit hosts",
		Value:       20,
	}
	HostsView = PermissionLevel{
		Description: "View hosts",
		Value:       10,
	}
	HostsNone = PermissionLevel{
		Description: "No hosts permissions",
		Value:       0,
	}
)

// GetDisplayNameForPermissionKey gets the display name associated with a permission key
func GetDisplayNameForPermissionKey(permissionKey string) string {
	switch permissionKey {
	case PermissionProjectSettings:
		return "Project Settings"
	case PermissionTasks:
		return "Tasks"
	case PermissionAnnotations:
		return "Task Annotations"
	case PermissionPatches:
		return "Patches"
	case PermissionGitTagVersions:
		return "Git Tag Versions"
	case PermissionLogs:
		return "Logs"
	case PermissionDistroSettings:
		return "Distro Settings"
	case PermissionHosts:
		return "Distro Hosts"
	default:
		return ""
	}
}

// GetPermissionLevelsForPermissionKey gets all permissions associated with a permission key
func GetPermissionLevelsForPermissionKey(permissionKey string) []PermissionLevel {
	switch permissionKey {
	case PermissionProjectSettings:
		return []PermissionLevel{
			ProjectSettingsEdit,
			ProjectSettingsView,
			ProjectSettingsNone,
		}
	case PermissionTasks:
		return []PermissionLevel{
			TasksAdmin,
			TasksBasic,
			TasksView,
			TasksNone,
		}
	case PermissionAnnotations:
		return []PermissionLevel{
			AnnotationsModify,
			AnnotationsView,
			AnnotationsNone,
		}
	case PermissionPatches:
		return []PermissionLevel{
			PatchSubmit,
			PatchNone,
		}
	case PermissionGitTagVersions:
		return []PermissionLevel{
			GitTagVersionsCreate,
			GitTagVersionsNone,
		}
	case PermissionLogs:
		return []PermissionLevel{
			LogsView,
			LogsNone,
		}
	case PermissionDistroSettings:
		return []PermissionLevel{
			DistroSettingsEdit,
			DistroSettingsView,
			DistroSettingsNone,
		}
	case PermissionHosts:
		return []PermissionLevel{
			HostsEdit,
			HostsView,
			HostsNone,
		}
	default:
		return []PermissionLevel{}
	}
}

// If adding a new type of permissions, i.e. a new array of permission keys, then you must:
// 1. Add a new field in the APIPermissions model for those permissions.
// 2. Populate the value of that APIPermissions field with the getPermissions function in rest/route/permissions.go

var ProjectPermissions = []string{
	PermissionProjectSettings,
	PermissionTasks,
	PermissionAnnotations,
	PermissionPatches,
	PermissionGitTagVersions,
	PermissionLogs,
}

var DistroPermissions = []string{
	PermissionDistroSettings,
	PermissionHosts,
}

var SuperuserPermissions = []string{
	PermissionAdminSettings,
	PermissionProjectCreate,
	PermissionDistroCreate,
	PermissionRoleModify,
}

const (
	BasicProjectAccessRole     = "basic_project_access"
	BasicDistroAccessRole      = "basic_distro_access"
	SuperUserRole              = "superuser"
	SuperUserProjectAccessRole = "admin_project_access"
	SuperUserDistroAccessRole  = "superuser_distro_access"
)

// Contains both general and superuser access.
var GeneralAccessRoles = []string{
	BasicProjectAccessRole,
	BasicDistroAccessRole,
	SuperUserRole,
	SuperUserProjectAccessRole,
	SuperUserDistroAccessRole,
}

// Constants for Evergreen log types.
const (
	LogTypeAgent  = "agent_log"
	LogTypeTask   = "task_log"
	LogTypeSystem = "system_log"
)

// LogViewer represents recognized viewers for rendering logs.
type LogViewer string

const (
	LogViewerRaw     LogViewer = "raw"
	LogViewerHTML    LogViewer = "html"
	LogViewerLobster LogViewer = "lobster"
	LogViewerParsley LogViewer = "parsley"
)

// ContainerOS denotes the operating system of a running container.
type ContainerOS string

const (
	LinuxOS   ContainerOS = "linux"
	WindowsOS ContainerOS = "windows"
)

// ValidContainerOperatingSystems contains all recognized container operating
// systems.
var ValidContainerOperatingSystems = []ContainerOS{LinuxOS, WindowsOS}

// Validate checks that the container OS is recognized.
func (c ContainerOS) Validate() error {
	switch c {
	case LinuxOS, WindowsOS:
		return nil
	default:
		return errors.Errorf("unrecognized container OS '%s'", c)
	}
}

// ContainerArch represents the CPU architecture necessary to run a container.
type ContainerArch string

const (
	ArchARM64 ContainerArch = "arm64"
	ArchAMD64 ContainerArch = "x86_64"
)

// ValidContainerArchitectures contains all recognized container CPU
// architectures.
var ValidContainerArchitectures = []ContainerArch{ArchARM64, ArchAMD64}

// Validate checks that the container CPU architecture is recognized.
func (c ContainerArch) Validate() error {
	switch c {
	case ArchARM64, ArchAMD64:
		return nil
	default:
		return errors.Errorf("unrecognized CPU architecture '%s'", c)
	}
}

// WindowsVersion specifies the compatibility version of Windows that is required for the container to run.
type WindowsVersion string

const (
	Windows2022 WindowsVersion = "2022"
	Windows2019 WindowsVersion = "2019"
	Windows2016 WindowsVersion = "2016"
)

// ValidWindowsVersions contains all recognized container Windows versions.
var ValidWindowsVersions = []WindowsVersion{Windows2016, Windows2019, Windows2022}

// Validate checks that the container Windows version is recognized.
func (w WindowsVersion) Validate() error {
	switch w {
	case Windows2022, Windows2019, Windows2016:
		return nil
	default:
		return errors.Errorf("unrecognized Windows version '%s'", w)
	}
}

// ParserProjectStorageMethod represents a means to store the parser project.
type ParserProjectStorageMethod string

const (
	// ProjectStorageMethodDB indicates that the parser project is stored as a
	// single document in a DB collection.
	ProjectStorageMethodDB ParserProjectStorageMethod = "db"
	// ProjectStorageMethodS3 indicates that the parser project is stored as a
	// single object in S3.
	ProjectStorageMethodS3 ParserProjectStorageMethod = "s3"
)

const (
	// Valid public key types.
	publicKeyRSA     = "ssh-rsa"
	publicKeyDSS     = "ssh-dss"
	publicKeyED25519 = "ssh-ed25519"
	publicKeyECDSA   = "ecdsa-sha2-nistp256"
)

var validKeyTypes = []string{
	publicKeyRSA,
	publicKeyDSS,
	publicKeyED25519,
	publicKeyECDSA,
}

var sensitiveCollections = []string{"project_vars"}

// ValidateSSHKey errors if the given key does not start with one of the allowed prefixes.
func ValidateSSHKey(key string) error {
	for _, prefix := range validKeyTypes {
		if strings.HasPrefix(key, prefix) {
			return nil
		}
	}
	return errors.Errorf("either an invalid Evergreen-managed key name has been provided, "+
		"or the key value is not one of the valid types: %s", validKeyTypes)
}

// ValidateCloneMethod checks that the clone mechanism is one of the supported
// methods.
func ValidateCloneMethod(method string) error {
	if !utility.StringSliceContains(ValidCloneMethods, method) {
		return errors.Errorf("'%s' is not a valid clone method", method)
	}
	return nil
}
