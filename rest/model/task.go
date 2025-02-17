package model

import (
	"fmt"
	"time"

	"github.com/evergreen-ci/evergreen"
	"github.com/evergreen-ci/evergreen/apimodels"
	"github.com/evergreen-ci/evergreen/model"
	"github.com/evergreen-ci/evergreen/model/artifact"
	"github.com/evergreen-ci/evergreen/model/host"
	"github.com/evergreen-ci/evergreen/model/task"
	"github.com/evergreen-ci/utility"
	"github.com/mongodb/grip"
	"github.com/pkg/errors"
)

const (
	TaskLogLinkFormat        = "%s/task_log_raw/%s/%d?type=%s"
	ParsleyTaskLogLinkFormat = "%s/evergreen/%s/%d/%s"
	EventLogLinkFormat       = "%s/event_log/task/%s"
)

// APITask is the model to be returned by the API whenever tasks are fetched.
type APITask struct {
	Id                          *string             `json:"task_id"`
	ProjectId                   *string             `json:"project_id"`
	ProjectIdentifier           *string             `json:"project_identifier"`
	CreateTime                  *time.Time          `json:"create_time"`
	DispatchTime                *time.Time          `json:"dispatch_time"`
	ScheduledTime               *time.Time          `json:"scheduled_time"`
	ContainerAllocatedTime      *time.Time          `json:"container_allocated_time"`
	StartTime                   *time.Time          `json:"start_time"`
	FinishTime                  *time.Time          `json:"finish_time"`
	IngestTime                  *time.Time          `json:"ingest_time"`
	ActivatedTime               *time.Time          `json:"activated_time"`
	Version                     *string             `json:"version_id"`
	Revision                    *string             `json:"revision"`
	Priority                    int64               `json:"priority"`
	Activated                   bool                `json:"activated"`
	ActivatedBy                 *string             `json:"activated_by"`
	ContainerAllocated          bool                `json:"container_allocated"`
	ContainerAllocationAttempts int                 `json:"container_allocation_attempts"`
	BuildId                     *string             `json:"build_id"`
	DistroId                    *string             `json:"distro_id"`
	Container                   *string             `json:"container"`
	ContainerOpts               APIContainerOptions `json:"container_options"`
	BuildVariant                *string             `json:"build_variant"`
	BuildVariantDisplayName     *string             `json:"build_variant_display_name"`
	DependsOn                   []APIDependency     `json:"depends_on"`
	DisplayName                 *string             `json:"display_name"`
	HostId                      *string             `json:"host_id"`
	PodID                       *string             `json:"pod_id,omitempty"`
	Execution                   int                 `json:"execution"`
	Order                       int                 `json:"order"`
	Status                      *string             `json:"status"`
	DisplayStatus               *string             `json:"display_status"`
	Details                     ApiTaskEndDetail    `json:"status_details"`
	Logs                        LogLinks            `json:"logs"`
	ParsleyLogs                 LogLinks            `json:"parsley_logs"`
	TimeTaken                   APIDuration         `json:"time_taken_ms"`
	ExpectedDuration            APIDuration         `json:"expected_duration_ms"`
	EstimatedStart              APIDuration         `json:"est_wait_to_start_ms"`
	PreviousExecutions          []APITask           `json:"previous_executions,omitempty"`
	GenerateTask                bool                `json:"generate_task"`
	GeneratedBy                 string              `json:"generated_by"`
	Artifacts                   []APIFile           `json:"artifacts"`
	DisplayOnly                 bool                `json:"display_only"`
	ParentTaskId                string              `json:"parent_task_id"`
	ExecutionTasks              []*string           `json:"execution_tasks,omitempty"`
	Tags                        []*string           `json:"tags,omitempty"`
	Mainline                    bool                `json:"mainline"`
	TaskGroup                   string              `json:"task_group,omitempty"`
	TaskGroupMaxHosts           int                 `json:"task_group_max_hosts,omitempty"`
	Blocked                     bool                `json:"blocked"`
	Requester                   *string             `json:"requester"`
	TestResults                 []APITest           `json:"test_results"`
	Aborted                     bool                `json:"aborted"`
	AbortInfo                   APIAbortInfo        `json:"abort_info,omitempty"`
	CanSync                     bool                `json:"can_sync,omitempty"`
	SyncAtEndOpts               APISyncAtEndOptions `json:"sync_at_end_opts"`
	AMI                         *string             `json:"ami"`
	MustHaveResults             bool                `json:"must_have_test_results"`
	BaseTask                    APIBaseTaskInfo     `json:"base_task"`
	ResetWhenFinished           bool                `json:"reset_when_finished"`
	// These fields are used by graphql gen, but do not need to be exposed
	// via Evergreen's user-facing API.
	OverrideDependencies bool   `json:"-"`
	Archived             bool   `json:"archived"`
	ResultsService       string `json:"-"`
	HasCedarResults      bool   `json:"-"`
	ResultsFailed        bool   `json:"-"`
}

type APIAbortInfo struct {
	User       string `json:"user,omitempty"`
	TaskID     string `json:"task_id,omitempty"`
	NewVersion string `json:"new_version,omitempty"`
	PRClosed   bool   `json:"pr_closed,omitempty"`
}

type LogLinks struct {
	AllLogLink    *string `json:"all_log"`
	TaskLogLink   *string `json:"task_log"`
	AgentLogLink  *string `json:"agent_log"`
	SystemLogLink *string `json:"system_log"`
	EventLogLink  *string `json:"event_log,omitempty"`
}

type ApiTaskEndDetail struct {
	Status      *string           `json:"status"`
	Type        *string           `json:"type"`
	Description *string           `json:"desc"`
	TimedOut    bool              `json:"timed_out"`
	TimeoutType *string           `json:"timeout_type"`
	OOMTracker  APIOomTrackerInfo `json:"oom_tracker_info"`
}

func (at *ApiTaskEndDetail) BuildFromService(t apimodels.TaskEndDetail) error {
	at.Status = utility.ToStringPtr(t.Status)
	at.Type = utility.ToStringPtr(t.Type)
	at.Description = utility.ToStringPtr(t.Description)
	at.TimedOut = t.TimedOut
	at.TimeoutType = utility.ToStringPtr(t.TimeoutType)

	apiOomTracker := APIOomTrackerInfo{}
	apiOomTracker.BuildFromService(t.OOMTracker)
	at.OOMTracker = apiOomTracker

	return nil
}

func (ad *ApiTaskEndDetail) ToService() apimodels.TaskEndDetail {
	return apimodels.TaskEndDetail{
		Status:      utility.FromStringPtr(ad.Status),
		Type:        utility.FromStringPtr(ad.Type),
		Description: utility.FromStringPtr(ad.Description),
		TimedOut:    ad.TimedOut,
		TimeoutType: utility.FromStringPtr(ad.TimeoutType),
		OOMTracker:  ad.OOMTracker.ToService(),
	}
}

type APIOomTrackerInfo struct {
	Detected bool  `json:"detected"`
	Pids     []int `json:"pids"`
}

func (at *APIOomTrackerInfo) BuildFromService(t *apimodels.OOMTrackerInfo) {
	if t != nil {
		at.Detected = t.Detected
		at.Pids = t.Pids
	}
}

func (ad *APIOomTrackerInfo) ToService() *apimodels.OOMTrackerInfo {
	return &apimodels.OOMTrackerInfo{
		Detected: ad.Detected,
		Pids:     ad.Pids,
	}
}

// BuildPreviousExecutions adds the given previous executions to the given API task.
func (at *APITask) BuildPreviousExecutions(tasks []task.Task, logURL, parsleyURL string) error {
	at.PreviousExecutions = make([]APITask, len(tasks))
	for i := range at.PreviousExecutions {
		if err := at.PreviousExecutions[i].BuildFromService(&tasks[i], &APITaskArgs{
			IncludeProjectIdentifier: true,
			IncludeAMI:               true,
			IncludeArtifacts:         true,
			LogURL:                   logURL,
			ParsleyLogURL:            parsleyURL,
		}); err != nil {
			return errors.Wrapf(err, "converting previous task execution at index %d to API model", i)
		}
	}

	return nil
}

// buildTask converts from a service level task by loading the data
// into the appropriate fields of the APITask.
func (at *APITask) buildTask(t *task.Task) error {
	id := t.Id
	// Old tasks are stored in a separate collection with ID set to
	// "old_task_ID" + "_" + "execution_number". This ID is not exposed to the user,
	// however. Instead in the UI executions are represented with a "/" and could be
	// represented in other ways elsewhere. The correct way to represent an old task is
	// with the same ID as the last execution, since semantically the tasks differ in
	// their execution number, not in their ID.
	if t.OldTaskId != "" {
		id = t.OldTaskId
	}
	*at = APITask{
		Id:                          utility.ToStringPtr(id),
		ProjectId:                   utility.ToStringPtr(t.Project),
		CreateTime:                  ToTimePtr(t.CreateTime),
		DispatchTime:                ToTimePtr(t.DispatchTime),
		ScheduledTime:               ToTimePtr(t.ScheduledTime),
		ContainerAllocatedTime:      ToTimePtr(t.ContainerAllocatedTime),
		StartTime:                   ToTimePtr(t.StartTime),
		FinishTime:                  ToTimePtr(t.FinishTime),
		IngestTime:                  ToTimePtr(t.IngestTime),
		ActivatedTime:               ToTimePtr(t.ActivatedTime),
		Version:                     utility.ToStringPtr(t.Version),
		Revision:                    utility.ToStringPtr(t.Revision),
		Priority:                    t.Priority,
		Activated:                   t.Activated,
		ActivatedBy:                 utility.ToStringPtr(t.ActivatedBy),
		ContainerAllocated:          t.ContainerAllocated,
		ContainerAllocationAttempts: t.ContainerAllocationAttempts,
		BuildId:                     utility.ToStringPtr(t.BuildId),
		DistroId:                    utility.ToStringPtr(t.DistroId),
		Container:                   utility.ToStringPtr(t.Container),
		BuildVariant:                utility.ToStringPtr(t.BuildVariant),
		BuildVariantDisplayName:     utility.ToStringPtr(t.BuildVariantDisplayName),
		DisplayName:                 utility.ToStringPtr(t.DisplayName),
		HostId:                      utility.ToStringPtr(t.HostId),
		PodID:                       utility.ToStringPtr(t.PodID),
		Tags:                        utility.ToStringPtrSlice(t.Tags),
		Execution:                   t.Execution,
		Order:                       t.RevisionOrderNumber,
		Status:                      utility.ToStringPtr(t.Status),
		DisplayStatus:               utility.ToStringPtr(t.GetDisplayStatus()),
		ExpectedDuration:            NewAPIDuration(t.ExpectedDuration),
		GenerateTask:                t.GenerateTask,
		GeneratedBy:                 t.GeneratedBy,
		DisplayOnly:                 t.DisplayOnly,
		Mainline:                    t.Requester == evergreen.RepotrackerVersionRequester,
		TaskGroup:                   t.TaskGroup,
		TaskGroupMaxHosts:           t.TaskGroupMaxHosts,
		Blocked:                     t.Blocked(),
		Requester:                   utility.ToStringPtr(t.Requester),
		Aborted:                     t.Aborted,
		CanSync:                     t.CanSync,
		ResultsService:              t.ResultsService,
		HasCedarResults:             t.HasCedarResults,
		ResultsFailed:               t.ResultsFailed,
		MustHaveResults:             t.MustHaveResults,
		ResetWhenFinished:           t.ResetWhenFinished,
		ParentTaskId:                utility.FromStringPtr(t.DisplayTaskId),
		SyncAtEndOpts: APISyncAtEndOptions{
			Enabled:  t.SyncAtEndOpts.Enabled,
			Statuses: t.SyncAtEndOpts.Statuses,
			Timeout:  t.SyncAtEndOpts.Timeout,
		},
		AbortInfo: APIAbortInfo{
			NewVersion: t.AbortInfo.NewVersion,
			TaskID:     t.AbortInfo.TaskID,
			User:       t.AbortInfo.User,
			PRClosed:   t.AbortInfo.PRClosed,
		},
	}

	at.ContainerOpts.BuildFromService(t.ContainerOpts)

	if t.BaseTask.Id != "" {
		at.BaseTask = APIBaseTaskInfo{
			Id:     utility.ToStringPtr(t.BaseTask.Id),
			Status: utility.ToStringPtr(t.BaseTask.Status),
		}
	}

	if t.TimeTaken != 0 {
		at.TimeTaken = NewAPIDuration(t.TimeTaken)
	} else if t.Status == evergreen.TaskStarted {
		at.TimeTaken = NewAPIDuration(time.Since(t.StartTime))
	}

	if t.ParentPatchID != "" {
		at.Version = utility.ToStringPtr(t.ParentPatchID)
		if t.ParentPatchNumber != 0 {
			at.Order = t.ParentPatchNumber
		}
	}

	if err := at.Details.BuildFromService(t.Details); err != nil {
		return errors.Wrap(err, "converting task end details to API model")
	}

	if len(t.ExecutionTasks) > 0 {
		ets := []*string{}
		for _, t := range t.ExecutionTasks {
			ets = append(ets, utility.ToStringPtr(t))
		}
		at.ExecutionTasks = ets
	}

	if len(t.DependsOn) > 0 {
		dependsOn := make([]APIDependency, len(t.DependsOn))
		for i, dep := range t.DependsOn {
			apiDep := APIDependency{}
			apiDep.BuildFromService(dep)
			dependsOn[i] = apiDep
		}
		at.DependsOn = dependsOn
	}

	at.OverrideDependencies = t.OverrideDependencies
	at.Archived = t.Archived

	return nil
}

type APITaskArgs struct {
	IncludeProjectIdentifier bool
	IncludeAMI               bool
	IncludeArtifacts         bool
	LogURL                   string
	ParsleyLogURL            string
}

// BuildFromService converts from a service level task by loading the data
// into the appropriate fields of the APITask. It takes optional arguments to populate
// additional fields.
func (at *APITask) BuildFromService(t *task.Task, args *APITaskArgs) error {
	err := at.buildTask(t)
	if err != nil {
		return err
	}
	if args == nil {
		return nil
	}
	baseTaskID := t.Id
	if t.OldTaskId != "" {
		baseTaskID = t.OldTaskId
	}
	if args.LogURL != "" {
		ll := LogLinks{
			AllLogLink:    utility.ToStringPtr(fmt.Sprintf(TaskLogLinkFormat, args.LogURL, baseTaskID, t.Execution, "ALL")),
			TaskLogLink:   utility.ToStringPtr(fmt.Sprintf(TaskLogLinkFormat, args.LogURL, baseTaskID, t.Execution, "T")),
			AgentLogLink:  utility.ToStringPtr(fmt.Sprintf(TaskLogLinkFormat, args.LogURL, baseTaskID, t.Execution, "E")),
			SystemLogLink: utility.ToStringPtr(fmt.Sprintf(TaskLogLinkFormat, args.LogURL, baseTaskID, t.Execution, "S")),
			EventLogLink:  utility.ToStringPtr(fmt.Sprintf(EventLogLinkFormat, args.LogURL, baseTaskID)),
		}
		at.Logs = ll
	}
	if args.ParsleyLogURL != "" {
		ll := LogLinks{
			AllLogLink:    utility.ToStringPtr(fmt.Sprintf(ParsleyTaskLogLinkFormat, args.ParsleyLogURL, baseTaskID, t.Execution, "all")),
			TaskLogLink:   utility.ToStringPtr(fmt.Sprintf(ParsleyTaskLogLinkFormat, args.ParsleyLogURL, baseTaskID, t.Execution, "task")),
			AgentLogLink:  utility.ToStringPtr(fmt.Sprintf(ParsleyTaskLogLinkFormat, args.ParsleyLogURL, baseTaskID, t.Execution, "agent")),
			SystemLogLink: utility.ToStringPtr(fmt.Sprintf(ParsleyTaskLogLinkFormat, args.ParsleyLogURL, baseTaskID, t.Execution, "system")),
		}
		at.ParsleyLogs = ll
	}
	if args.IncludeAMI {
		if err := at.GetAMI(); err != nil {
			return errors.Wrap(err, "getting AMI")
		}
	}
	if args.IncludeArtifacts {
		if err := at.getArtifacts(); err != nil {
			return errors.Wrap(err, "getting artifacts")
		}
	}
	if args.IncludeProjectIdentifier {
		at.GetProjectIdentifier()
	}

	return nil
}

func (at *APITask) GetAMI() error {
	if at.AMI != nil {
		return nil
	}
	if utility.FromStringPtr(at.HostId) != "" {
		h, err := host.FindOneId(utility.FromStringPtr(at.HostId))
		if err != nil {
			return errors.Wrapf(err, "finding host '%s' for task", utility.FromStringPtr(at.HostId))
		}
		if h != nil {
			ami := h.GetAMI()
			if ami != "" {
				at.AMI = utility.ToStringPtr(ami)
			}
		}
	}
	return nil
}

func (at *APITask) GetProjectIdentifier() {
	if at.ProjectIdentifier != nil {
		return
	}
	if utility.FromStringPtr(at.ProjectId) != "" {
		identifier, err := model.GetIdentifierForProject(utility.FromStringPtr(at.ProjectId))
		if err == nil {
			at.ProjectIdentifier = utility.ToStringPtr(identifier)
		}
	}
}

// ToService returns a service layer task using the data from the APITask.
// Wraps ToServiceTask to maintain the model interface.
func (at *APITask) ToService() (*task.Task, error) {
	st := &task.Task{
		Id:                          utility.FromStringPtr(at.Id),
		Project:                     utility.FromStringPtr(at.ProjectId),
		Version:                     utility.FromStringPtr(at.Version),
		Revision:                    utility.FromStringPtr(at.Revision),
		Priority:                    at.Priority,
		Activated:                   at.Activated,
		ActivatedBy:                 utility.FromStringPtr(at.ActivatedBy),
		ContainerAllocated:          at.ContainerAllocated,
		ContainerAllocationAttempts: at.ContainerAllocationAttempts,
		BuildId:                     utility.FromStringPtr(at.BuildId),
		DistroId:                    utility.FromStringPtr(at.DistroId),
		Container:                   utility.FromStringPtr(at.Container),
		ContainerOpts:               at.ContainerOpts.ToService(),
		BuildVariant:                utility.FromStringPtr(at.BuildVariant),
		BuildVariantDisplayName:     utility.FromStringPtr(at.BuildVariantDisplayName),
		DisplayName:                 utility.FromStringPtr(at.DisplayName),
		HostId:                      utility.FromStringPtr(at.HostId),
		PodID:                       utility.FromStringPtr(at.PodID),
		Execution:                   at.Execution,
		RevisionOrderNumber:         at.Order,
		Status:                      utility.FromStringPtr(at.Status),
		DisplayStatus:               utility.FromStringPtr(at.DisplayStatus),
		TimeTaken:                   at.TimeTaken.ToDuration(),
		ExpectedDuration:            at.ExpectedDuration.ToDuration(),
		GenerateTask:                at.GenerateTask,
		GeneratedBy:                 at.GeneratedBy,
		DisplayOnly:                 at.DisplayOnly,
		Requester:                   utility.FromStringPtr(at.Requester),
		CanSync:                     at.CanSync,
		ResultsService:              at.ResultsService,
		HasCedarResults:             at.HasCedarResults,
		ResultsFailed:               at.ResultsFailed,
		MustHaveResults:             at.MustHaveResults,
		SyncAtEndOpts: task.SyncAtEndOptions{
			Enabled:  at.SyncAtEndOpts.Enabled,
			Statuses: at.SyncAtEndOpts.Statuses,
			Timeout:  at.SyncAtEndOpts.Timeout,
		},
		BaseTask: task.BaseTaskInfo{
			Id:     utility.FromStringPtr(at.BaseTask.Id),
			Status: utility.FromStringPtr(at.BaseTask.Status),
		},
		DisplayTaskId:        utility.ToStringPtr(at.ParentTaskId),
		Aborted:              at.Aborted,
		Details:              at.Details.ToService(),
		Archived:             at.Archived,
		OverrideDependencies: at.OverrideDependencies,
	}

	catcher := grip.NewBasicCatcher()
	var err error
	st.CreateTime, err = FromTimePtr(at.CreateTime)
	catcher.Add(err)
	st.DispatchTime, err = FromTimePtr(at.DispatchTime)
	catcher.Add(err)
	st.ScheduledTime, err = FromTimePtr(at.ScheduledTime)
	catcher.Add(err)
	st.ContainerAllocatedTime, err = FromTimePtr(at.ContainerAllocatedTime)
	catcher.Add(err)
	st.StartTime, err = FromTimePtr(at.StartTime)
	catcher.Add(err)
	st.FinishTime, err = FromTimePtr(at.FinishTime)
	catcher.Add(err)
	st.IngestTime, err = FromTimePtr(at.IngestTime)
	catcher.Add(err)
	st.ActivatedTime, err = FromTimePtr(at.ActivatedTime)
	catcher.Add(err)
	if catcher.HasErrors() {
		return nil, catcher.Resolve()
	}

	if len(at.ExecutionTasks) > 0 {
		ets := []string{}
		for _, t := range at.ExecutionTasks {
			ets = append(ets, utility.FromStringPtr(t))
		}
		st.ExecutionTasks = ets
	}

	dependsOn := make([]task.Dependency, len(at.DependsOn))
	for i, dep := range at.DependsOn {
		dependsOn[i].TaskId = dep.TaskId
		dependsOn[i].Status = dep.Status
	}
	st.DependsOn = dependsOn
	return st, nil
}

func (at *APITask) getArtifacts() error {
	var err error
	var entries []artifact.Entry
	if at.DisplayOnly {
		ets := []artifact.TaskIDAndExecution{}
		for _, t := range at.ExecutionTasks {
			ets = append(ets, artifact.TaskIDAndExecution{TaskID: *t, Execution: at.Execution})
		}
		if len(ets) > 0 {
			entries, err = artifact.FindAll(artifact.ByTaskIdsAndExecutions(ets))
		}
	} else {
		entries, err = artifact.FindAll(artifact.ByTaskIdAndExecution(utility.FromStringPtr(at.Id), at.Execution))
	}
	if err != nil {
		return errors.Wrap(err, "retrieving artifacts")
	}
	for _, entry := range entries {
		var strippedFiles []artifact.File
		// The route requires a user, so hasUser is always true.
		strippedFiles, err = artifact.StripHiddenFiles(entry.Files, true)
		if err != nil {
			return err
		}
		for _, file := range strippedFiles {
			apiFile := APIFile{}
			apiFile.BuildFromService(file)
			at.Artifacts = append(at.Artifacts, apiFile)
		}
	}

	return nil
}

type APISyncAtEndOptions struct {
	Enabled  bool          `json:"enabled"`
	Statuses []string      `json:"statuses"`
	Timeout  time.Duration `json:"timeout"`
}

type APIDependency struct {
	TaskId string `bson:"_id" json:"id"`
	Status string `bson:"status" json:"status"`
}

func (ad *APIDependency) BuildFromService(dep task.Dependency) {
	ad.TaskId = dep.TaskId
	ad.Status = dep.Status
}

type APIContainerOptions struct {
	CPU            int     `json:"cpu"`
	MemoryMB       int     `json:"memory_mb"`
	WorkingDir     *string `json:"working_dir,omitempty"`
	Image          *string `json:"image,omitempty"`
	RepoCredsName  *string `json:"repo_creds_name,omitempty"`
	OS             *string `json:"os,omitempty"`
	Arch           *string `json:"arch,omitempty"`
	WindowsVersion *string `json:"windows_version,omitempty"`
}

func (o *APIContainerOptions) BuildFromService(dbOpts task.ContainerOptions) {
	o.CPU = dbOpts.CPU
	o.MemoryMB = dbOpts.MemoryMB
	o.WorkingDir = utility.ToStringPtr(dbOpts.WorkingDir)
	o.Image = utility.ToStringPtr(dbOpts.Image)
	o.OS = utility.ToStringPtr(string(dbOpts.OS))
	o.Arch = utility.ToStringPtr(string(dbOpts.Arch))
	o.WindowsVersion = utility.ToStringPtr(string(dbOpts.WindowsVersion))
}

func (o *APIContainerOptions) ToService() task.ContainerOptions {
	return task.ContainerOptions{
		CPU:            o.CPU,
		MemoryMB:       o.MemoryMB,
		WorkingDir:     utility.FromStringPtr(o.WorkingDir),
		Image:          utility.FromStringPtr(o.Image),
		OS:             evergreen.ContainerOS(utility.FromStringPtr(o.OS)),
		Arch:           evergreen.ContainerArch(utility.FromStringPtr(o.Arch)),
		WindowsVersion: evergreen.WindowsVersion(utility.FromStringPtr(o.WindowsVersion)),
	}
}
