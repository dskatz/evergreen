package validator

import (
	"bytes"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/evergreen-ci/evergreen"
	"github.com/evergreen-ci/evergreen/agent"
	"github.com/evergreen-ci/evergreen/agent/command"
	"github.com/evergreen-ci/evergreen/model"
	"github.com/evergreen-ci/evergreen/model/distro"
	"github.com/evergreen-ci/evergreen/model/task"
	"github.com/evergreen-ci/evergreen/thirdparty"
	"github.com/evergreen-ci/evergreen/util"
	"github.com/evergreen-ci/utility"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/level"
	"github.com/pkg/errors"
)

type projectValidator func(*model.Project) ValidationErrors

type projectConfigValidator func(config *model.ProjectConfig) ValidationErrors

type projectSettingsValidator func(*evergreen.Settings, *model.Project, *model.ProjectRef, bool) ValidationErrors

// bool indicates if we should still run the validator if the project is complex
type longValidator func(*model.Project, bool) ValidationErrors

type ValidationErrorLevel int64

const (
	Error ValidationErrorLevel = iota
	Warning
	unauthorizedCharacters                  = "|"
	EC2HostCreateTotalLimit                 = 1000
	DockerHostCreateTotalLimit              = 200
	HostCreateLimitPerTask                  = 3
	maxTaskSyncCommandsForDependenciesCheck = 300 // this should take about one second
)

func (vel ValidationErrorLevel) String() string {
	switch vel {
	case Error:
		return "ERROR"
	case Warning:
		return "WARNING"
	}
	return "?"
}

type ValidationError struct {
	Level   ValidationErrorLevel `json:"level"`
	Message string               `json:"message"`
}

type ValidationErrors []ValidationError

func (v ValidationErrors) Raw() interface{} {
	return v
}
func (v ValidationErrors) Loggable() bool {
	return len(v) > 0
}
func (v ValidationErrors) String() string {
	out := ""
	for i, validationErr := range v {
		if i > 0 {
			out += "\n"
		}
		out += fmt.Sprintf("%s: %s", validationErr.Level.String(), validationErr.Message)
	}

	return out
}
func (v ValidationErrors) Annotate(key string, value interface{}) error {
	return nil
}
func (v ValidationErrors) Priority() level.Priority {
	return level.Info
}
func (v ValidationErrors) SetPriority(_ level.Priority) error {
	return nil
}

// AtLevel returns all validation errors that match the given level.
func (v ValidationErrors) AtLevel(level ValidationErrorLevel) ValidationErrors {
	errs := ValidationErrors{}
	for _, err := range v {
		if err.Level == level {
			errs = append(errs, err)
		}
	}
	return errs
}

// HasError returns true if any of the errors are at the error level.
func (v ValidationErrors) HasError() bool {
	for _, err := range v {
		if err.Level == Error {
			return true
		}
	}
	return false
}

type ValidationInput struct {
	ProjectYaml []byte `json:"project_yaml" yaml:"project_yaml"`
	Quiet       bool   `json:"quiet" yaml:"quiet"`
	IncludeLong bool   `json:"include_long" yaml:"include_long"`
	ProjectID   string `json:"project_id" yaml:"project_id"`
}

// Functions used to validate the syntax of a project configuration file.
var projectErrorValidators = []projectValidator{
	validateBVFields,
	validateDependencyGraph,
	validatePluginCommands,
	validateProjectFields,
	validateTaskDependencies,
	validateTaskNames,
	validateBVNames,
	validateBVBatchTimes,
	validateDisplayTaskNames,
	validateBVTaskNames,
	validateAllDependenciesSpec,
	validateProjectTaskNames,
	validateProjectTaskIdsAndTags,
	validateParameters,
	validateTaskGroups,
	validateHostCreates,
	validateDuplicateBVTasks,
	validateGenerateTasks,
}

// Functions used to validate the syntax of project configs representing properties found on the project page.
var projectConfigErrorValidators = []projectConfigValidator{
	validateProjectConfigAliases,
	validateProjectConfigPlugins,
	validateProjectConfigContainers,
}

// Functions used to validate the semantics of a project configuration file.
var projectWarningValidators = []projectValidator{
	checkTaskGroups,
	checkProjectFields,
	checkTaskRuns,
	checkModules,
	checkTasks,
	checkBuildVariants,
}

var projectSettingsValidators = []projectSettingsValidator{
	validateTaskSyncSettings,
	validateVersionControl,
	validateContainers,
}

// These validators have the potential to be very long, and may not be fully run unless specified.
var longErrorValidators = []longValidator{
	validateTaskSyncCommands,
}

func (vr ValidationError) Error() string {
	return vr.Message
}

func ValidationErrorsToString(ves ValidationErrors) string {
	var s bytes.Buffer
	if len(ves) == 0 {
		return s.String()
	}
	for _, ve := range ves {
		s.WriteString(ve.Error())
		s.WriteString("\n")
	}
	return s.String()
}

// getDistros creates a slice of all distro IDs and aliases.
func getDistros() (ids []string, aliases []string, err error) {
	return getDistrosForProject("")
}

// getDistrosForProject creates a slice of all valid distro IDs and a slice of
// all valid aliases for a project. If projectID is empty, it returns all distro
// IDs and all aliases.
func getDistrosForProject(projectID string) (ids []string, aliases []string, err error) {
	// create a slice of all known distros
	distros, err := distro.Find(distro.All)
	if err != nil {
		return nil, nil, err
	}
	for _, d := range distros {
		if projectID != "" && len(d.ValidProjects) > 0 {
			if utility.StringSliceContains(d.ValidProjects, projectID) {
				ids = append(ids, d.Id)
				for _, alias := range d.Aliases {
					if !utility.StringSliceContains(aliases, alias) {
						aliases = append(aliases, alias)
					}
				}
			}
		} else {
			ids = append(ids, d.Id)
			for _, alias := range d.Aliases {
				if !utility.StringSliceContains(aliases, alias) {
					aliases = append(aliases, alias)
				}
			}
		}
	}
	return ids, aliases, nil
}

// verify that the project configuration semantics is valid
func CheckProjectWarnings(project *model.Project) ValidationErrors {
	validationErrs := ValidationErrors{}
	for _, projectWarningValidator := range projectWarningValidators {
		validationErrs = append(validationErrs,
			projectWarningValidator(project)...)
	}
	return validationErrs
}

func CheckAliasWarnings(project *model.Project, aliases model.ProjectAliases) ValidationErrors {
	return validateAliasCoverage(project, aliases)
}

// verify that the project configuration syntax is valid
func CheckProjectErrors(project *model.Project, includeLong bool) ValidationErrors {
	validationErrs := ValidationErrors{}
	for _, projectErrorValidator := range projectErrorValidators {
		validationErrs = append(validationErrs,
			projectErrorValidator(project)...)
	}
	for _, longSyntaxValidator := range longErrorValidators {
		validationErrs = append(validationErrs,
			longSyntaxValidator(project, includeLong)...)
	}

	// get distro IDs and aliases for ensureReferentialIntegrity validation
	distroIDs, distroAliases, err := getDistrosForProject(project.Identifier)
	if err != nil {
		validationErrs = append(validationErrs, ValidationError{Message: "can't get distros from database"})
	}
	containerNameMap := map[string]bool{}
	for _, container := range project.Containers {
		if containerNameMap[container.Name] {
			validationErrs = append(validationErrs, ValidationError{Message: fmt.Sprintf("container '%s' is defined multiple times", container.Name)})
		}
		containerNameMap[container.Name] = true
	}
	validationErrs = append(validationErrs, ensureReferentialIntegrity(project, containerNameMap, distroIDs, distroAliases)...)
	return validationErrs
}

func CheckPatchedProjectConfigErrors(patchedProjectConfig string) ValidationErrors {
	validationErrs := ValidationErrors{}
	if len(patchedProjectConfig) <= 0 {
		return validationErrs
	}
	projectConfig, err := model.CreateProjectConfig([]byte(patchedProjectConfig), "")
	if err != nil {
		validationErrs = append(validationErrs, ValidationError{
			Message: fmt.Sprintf("Error unmarshalling patched project config: %s", err.Error()),
		})
		return validationErrs
	}
	return CheckProjectConfigErrors(projectConfig)
}

// verify that the project configuration syntax is valid
func CheckProjectConfigErrors(projectConfig *model.ProjectConfig) ValidationErrors {
	validationErrs := ValidationErrors{}
	if projectConfig == nil {
		return validationErrs
	}
	for _, projectConfigErrorValidator := range projectConfigErrorValidators {
		validationErrs = append(validationErrs,
			projectConfigErrorValidator(projectConfig)...)
	}
	return validationErrs
}

// CheckProjectSettings checks the project configuration against the project
// settings.
func CheckProjectSettings(settings *evergreen.Settings, p *model.Project, ref *model.ProjectRef, isConfigDefined bool) ValidationErrors {
	var errs ValidationErrors
	for _, validateSettings := range projectSettingsValidators {
		errs = append(errs, validateSettings(settings, p, ref, isConfigDefined)...)
	}
	return errs
}

// checks if the project configuration has errors
func CheckProjectConfigurationIsValid(settings *evergreen.Settings, project *model.Project, pref *model.ProjectRef) error {
	catcher := grip.NewBasicCatcher()
	projectErrors := CheckProjectErrors(project, false)
	if len(projectErrors) != 0 {
		if errs := projectErrors.AtLevel(Error); len(errs) != 0 {
			catcher.Errorf("project contains errors: %s", ValidationErrorsToString(errs))
		}
	}

	if settingsErrs := CheckProjectSettings(settings, project, pref, false); len(settingsErrs) != 0 {
		if errs := settingsErrs.AtLevel(Error); len(errs) != 0 {
			catcher.Errorf("project contains errors related to project settings: %s", ValidationErrorsToString(errs))
		}
	}
	return catcher.Resolve()
}

// ensure that if any task spec references 'model.AllDependencies', it
// references no other dependency within the variant
func validateAllDependenciesSpec(project *model.Project) ValidationErrors {
	errs := ValidationErrors{}
	for _, task := range project.Tasks {
		coveredVariants := map[string]bool{}
		if len(task.DependsOn) > 1 {
			for _, dependency := range task.DependsOn {
				if dependency.Name == model.AllDependencies {
					// incorrect if no variant specified or this variant has already been covered
					if dependency.Variant == "" || coveredVariants[dependency.Variant] {
						errs = append(errs,
							ValidationError{
								Message: fmt.Sprintf("task '%s' contains the all dependencies (%s)' "+
									"specification and other explicit dependencies or duplicate variants",
									task.Name, model.AllDependencies),
							},
						)
					}
					coveredVariants[dependency.Variant] = true
				}
			}
		}
	}
	return errs
}

func validateContainers(settings *evergreen.Settings, project *model.Project, ref *model.ProjectRef, _ bool) ValidationErrors {
	errs := ValidationErrors{}
	err := model.ValidateContainers(settings.Providers.AWS.Pod.ECS, ref, project.Containers)
	if err != nil {
		errs = append(errs,
			ValidationError{
				Message: errors.Wrap(err, "error validating containers").Error(),
				Level:   Error,
			},
		)
	}
	return errs
}

// validateDependencyGraph returns a non-nil ValidationErrors if the dependency graph contains cycles.
func validateDependencyGraph(project *model.Project) ValidationErrors {
	errs := ValidationErrors{}
	graph := project.DependencyGraph()
	for _, cycle := range graph.Cycles() {
		var nodeStrings []string
		for _, task := range cycle {
			nodeStrings = append(nodeStrings, task.String())
		}
		errs = append(errs, ValidationError{
			Level:   Error,
			Message: fmt.Sprintf("tasks [%s] form a dependency cycle", strings.Join(nodeStrings, ", ")),
		})
	}

	return errs
}

// tvToTaskUnit generates all task-variant pairs mapped to their corresponding
// task unit within a build variant.
func tvToTaskUnit(p *model.Project) map[model.TVPair]model.BuildVariantTaskUnit {
	// map of task name and variant -> BuildVariantTaskUnit
	tasksByNameAndVariant := map[model.TVPair]model.BuildVariantTaskUnit{}

	// generate task nodes for every task and variant combination

	taskGroups := map[string]struct{}{}
	for _, tg := range p.TaskGroups {
		taskGroups[tg.Name] = struct{}{}
	}
	for _, bv := range p.BuildVariants {
		tasksToAdd := []model.BuildVariantTaskUnit{}
		for _, t := range bv.Tasks {
			if _, ok := taskGroups[t.Name]; ok {
				tasksToAdd = append(tasksToAdd, model.CreateTasksFromGroup(t, p, "")...)
			} else {
				tasksToAdd = append(tasksToAdd, t)
			}
		}
		for _, t := range tasksToAdd {
			t.Variant = bv.Name
			node := model.TVPair{
				Variant:  bv.Name,
				TaskName: t.Name,
			}

			tasksByNameAndVariant[node] = t
		}
	}
	return tasksByNameAndVariant
}

func validateProjectConfigAliases(pc *model.ProjectConfig) ValidationErrors {
	errs := []string{}
	pc.SetInternalAliases()
	errs = append(errs, model.ValidateProjectAliases(pc.GitHubPRAliases, "GitHub PR Aliases")...)
	errs = append(errs, model.ValidateProjectAliases(pc.GitHubChecksAliases, "GitHub Checks Aliases")...)
	errs = append(errs, model.ValidateProjectAliases(pc.CommitQueueAliases, "Commit Queue Aliases")...)
	errs = append(errs, model.ValidateProjectAliases(pc.PatchAliases, "Patch Aliases")...)
	errs = append(errs, model.ValidateProjectAliases(pc.GitTagAliases, "Git Tag Aliases")...)

	validationErrs := ValidationErrors{}
	for _, errorMsg := range errs {
		validationErrs = append(validationErrs, ValidationError{
			Message: fmt.Sprintf("error validating aliases: %s", errorMsg),
			Level:   Error,
		})
	}
	return validationErrs
}

// validateAliasCoverage validates that all commit queue aliases defined match some variants/tasks.
func validateAliasCoverage(p *model.Project, aliases model.ProjectAliases) ValidationErrors {
	aliasMap := map[string]model.ProjectAlias{}
	for _, a := range aliases {
		aliasMap[a.ID.Hex()] = a
	}
	aliasNeedsVariant, aliasNeedsTask, err := getAliasCoverage(p, aliasMap)
	if err != nil {
		return ValidationErrors{
			{
				Message: "error checking alias coverage, continuing without validation",
				Level:   Warning,
			},
		}
	}
	return constructAliasWarnings(aliasMap, aliasNeedsVariant, aliasNeedsTask)
}

// constructAliasWarnings returns validation errors given a map of aliases, and whether they need variants/tasks to match.
func constructAliasWarnings(aliasMap map[string]model.ProjectAlias, aliasNeedsVariant, aliasNeedsTask map[string]bool) ValidationErrors {
	res := ValidationErrors{}
	errs := []string{}
	for aliasID, a := range aliasMap {
		needsVariant := aliasNeedsVariant[aliasID]
		needsTask := aliasNeedsTask[aliasID]
		if !needsVariant && !needsTask {
			continue
		}

		msgComponents := []string{}
		switch a.Alias {
		case evergreen.CommitQueueAlias:
			msgComponents = append(msgComponents, "Commit queue alias")
		case evergreen.GithubPRAlias:
			msgComponents = append(msgComponents, "GitHub PR alias")
		case evergreen.GitTagAlias:
			msgComponents = append(msgComponents, "Git tag alias")
		case evergreen.GithubChecksAlias:
			msgComponents = append(msgComponents, "GitHub check alias")
		default:
			msgComponents = append(msgComponents, "Patch alias")
		}
		if len(a.VariantTags) > 0 {
			msgComponents = append(msgComponents, fmt.Sprintf("matching variant tags '%v'", a.VariantTags))
		} else {
			msgComponents = append(msgComponents, fmt.Sprintf("matching variant regexp '%s'", a.Variant))
		}
		if needsVariant {
			msgComponents = append(msgComponents, "has no matching variants")
		} else {
			// This is only relevant information if the alias matches the variant but not the task.
			if len(a.TaskTags) > 0 {
				msgComponents = append(msgComponents, fmt.Sprintf("and matching task tags '%v'", a.TaskTags))
			} else {
				msgComponents = append(msgComponents, fmt.Sprintf("and matching task regexp '%s'", a.Task))
			}
			msgComponents = append(msgComponents, "has no matching tasks")
		}
		errs = append(errs, strings.Join(msgComponents, " "))
	}
	sort.Strings(errs)
	for _, err := range errs {
		res = append(res, ValidationError{
			Message: err,
			Level:   Warning,
		})
	}

	return res
}

// getAliasCoverage returns a map of aliases that don't match variants and a map of aliases that don't match tasks.
func getAliasCoverage(p *model.Project, aliasMap map[string]model.ProjectAlias) (map[string]bool, map[string]bool, error) {
	type taskInfo struct {
		name string
		tags []string
	}
	aliasNeedsVariant := map[string]bool{}
	aliasNeedsTask := map[string]bool{}
	bvtCache := map[string]taskInfo{}
	for a := range aliasMap {
		aliasNeedsVariant[a] = true
		aliasNeedsTask[a] = true
	}
	for _, bv := range p.BuildVariants {
		for aliasID, alias := range aliasMap {
			if !aliasNeedsVariant[aliasID] && !aliasNeedsTask[aliasID] { // Have already found both variants and tasks.
				continue
			}
			// If we still need a task to match the variant, still check if the alias matches, so we know if checking tasks is needed.
			matchesThisVariant, err := alias.HasMatchingVariant(bv.Name, bv.Tags)
			if err != nil {
				return nil, nil, err
			}
			if !matchesThisVariant { // If the variant doesn't match, then there's no reason to keep checking tasks.
				continue
			}
			aliasNeedsVariant[aliasID] = false
			// Loop through all tasks to verify if there is task coverage.
			for _, t := range bv.Tasks {
				var name string
				var tags []string
				if info, ok := bvtCache[t.Name]; ok {
					name = info.name
					tags = info.tags
				} else {
					name, tags, _ = p.GetTaskNameAndTags(t)
					// Even if we can't find the name/tags, still store it, so we don't try again.
					bvtCache[t.Name] = taskInfo{name: name, tags: tags}
				}
				if name != "" {
					if t.IsGroup {
						matchesTaskGroupTask, err := aliasMatchesTaskGroupTask(p, alias, name)
						if err != nil {
							return nil, nil, err
						}
						if matchesTaskGroupTask {
							aliasNeedsTask[aliasID] = false
							break
						}
					}
					matchesThisTask, err := alias.HasMatchingTask(name, tags)
					if err != nil {
						return nil, nil, err
					}
					if matchesThisTask {
						aliasNeedsTask[aliasID] = false
						break
					}
				}
			}
		}
	}
	return aliasNeedsVariant, aliasNeedsTask, nil
}

func aliasMatchesTaskGroupTask(p *model.Project, alias model.ProjectAlias, tgName string) (bool, error) {
	tg := p.FindTaskGroup(tgName)
	if tg == nil {
		return false, errors.Errorf("definition for task group '%s' not found", tgName)
	}
	for _, tgTask := range tg.Tasks {
		t := p.FindProjectTask(tgTask)
		if t == nil {
			return false, errors.Errorf("task '%s' in task group '%s' not found", tgTask, tgName)
		}
		matchesTaskInTaskGroup, err := alias.HasMatchingTask(t.Name, t.Tags)
		if err != nil {
			return false, err
		}
		if matchesTaskInTaskGroup {
			return true, nil
		}
	}
	return false, nil
}

func validateProjectConfigContainers(pc *model.ProjectConfig) ValidationErrors {
	errs := ValidationErrors{}
	for _, size := range pc.ContainerSizeDefinitions {
		if size.Name == "" {
			errs = append(errs, ValidationError{
				Message: "container size name cannot be empty",
				Level:   Error,
			})
		}

		if err := size.Validate(evergreen.GetEnvironment().Settings().Providers.AWS.Pod.ECS); err != nil {
			errs = append(errs,
				ValidationError{
					Message: errors.Wrap(err, "error validating container resources").Error(),
					Level:   Error,
				},
			)
		}
	}
	return errs
}

func validateProjectConfigPlugins(pc *model.ProjectConfig) ValidationErrors {
	errs := ValidationErrors{}
	annotationSettings := pc.TaskAnnotationSettings
	var webhook *evergreen.WebHook
	if annotationSettings != nil {
		webhook = &annotationSettings.FileTicketWebhook
	}
	// skip validation if no build baron configuration exists
	if pc.BuildBaronSettings == nil {
		return ValidationErrors{}
	}
	err := model.ValidateBbProject(pc.Project, *pc.BuildBaronSettings, webhook)
	if err != nil {
		errs = append(errs,
			ValidationError{
				Message: errors.Wrap(err, "error validating build baron config").Error(),
				Level:   Error,
			},
		)
	}
	return errs
}

func hasValidRunOn(runOn []string) bool {
	for _, d := range runOn {
		if d != "" {
			return true
		}
	}
	return false
}

// Ensures that the project has at least one buildvariant and also that all the
// fields required for any buildvariant definition are present
func validateBVFields(project *model.Project) ValidationErrors {
	errs := ValidationErrors{}
	if len(project.BuildVariants) == 0 {
		return ValidationErrors{
			{
				Message: "must specify at least one buildvariant",
			},
		}
	}

	for _, buildVariant := range project.BuildVariants {
		if buildVariant.Name == "" {
			errs = append(errs,
				ValidationError{
					Message: "all buildvariants must have a name",
				},
			)
		}
		if len(buildVariant.Tasks) == 0 {
			errs = append(errs,
				ValidationError{
					Message: fmt.Sprintf("buildvariant '%s' must have at least one task",
						buildVariant.Name),
				},
			)
		}
		bvHasValidDistro := false
		for _, runOn := range buildVariant.RunOn {
			if runOn != "" {
				bvHasValidDistro = true
				break
			}
		}
		if bvHasValidDistro { // don't need to check if tasks have run_on defined since we have a variant default
			continue
		}

		for _, task := range buildVariant.Tasks {
			taskHasValidDistro := hasValidRunOn(task.RunOn)
			if taskHasValidDistro {
				break
			}
			if task.IsGroup {
				for _, t := range project.FindTaskGroup(task.Name).Tasks {
					pt := project.FindProjectTask(t)
					if pt != nil {
						if hasValidRunOn(pt.RunOn) {
							taskHasValidDistro = true
							break
						}
					}
				}
			} else {
				// check for a default in the task definition
				pt := project.FindProjectTask(task.Name)
				if pt != nil {
					taskHasValidDistro = hasValidRunOn(pt.RunOn)
				}
			}
			if !taskHasValidDistro {
				errs = append(errs,
					ValidationError{
						Message: fmt.Sprintf("buildvariant '%s' "+
							"must either specify run_on field or have every task specify run_on",
							buildVariant.Name),
					},
				)
				break
			}
		}
	}
	return errs
}

// Checks that the basic fields that are required by any project are present and
// valid.
func validateProjectFields(project *model.Project) ValidationErrors {
	errs := ValidationErrors{}

	if project.BatchTime < 0 {
		errs = append(errs,
			ValidationError{
				Message: "'batchtime' must be non-negative",
			},
		)
	}

	if project.CommandType != "" {
		if !utility.StringSliceContains(evergreen.ValidCommandTypes, project.CommandType) {
			errs = append(errs,
				ValidationError{
					Message: fmt.Sprintf("invalid command type: %s", project.CommandType),
				},
			)
		}
	}
	return errs
}

func checkProjectFields(project *model.Project) ValidationErrors {
	errs := ValidationErrors{}

	if project.BatchTime > math.MaxInt32 {
		// Error level is warning for backwards compatibility with
		// existing projects. This value will be capped at MaxInt32
		// in ProjectRef.getBatchTime()
		errs = append(errs,
			ValidationError{
				Message: fmt.Sprintf("'batchtime' should not exceed %d", math.MaxInt32),
				Level:   Warning,
			},
		)
	}

	return errs
}

func validateBuildVariantTaskNames(task string, variant string, allTaskNames map[string]bool, taskGroupTaskSet map[string]string) []ValidationError {
	var errs []ValidationError
	if _, ok := allTaskNames[task]; !ok {
		if task == "" {
			errs = append(errs, ValidationError{
				Message: fmt.Sprintf("tasks for buildvariant '%s' must each have a name field",
					variant),
				Level: Error,
			})

		} else {
			errs = append(errs, ValidationError{
				Message: fmt.Sprintf("buildvariant '%s' references a non-existent task '%s'",
					variant, task),
				Level: Error,
			})
		}
	}
	return errs
}

// ensureReferentialIntegrity checks all fields that reference other entities defined in the YAML and ensure that they are referring to valid names.
func ensureReferentialIntegrity(project *model.Project, containerNameMap map[string]bool, distroIDs []string, distroAliases []string) ValidationErrors {
	errs := ValidationErrors{}
	// create a set of all the task names
	allTaskNames := map[string]bool{}
	taskGroupTaskSet := map[string]string{}
	for _, task := range project.Tasks {
		allTaskNames[task.Name] = true
	}
	for _, taskGroup := range project.TaskGroups {
		allTaskNames[taskGroup.Name] = true
		for _, task := range taskGroup.Tasks {
			taskGroupTaskSet[task] = taskGroup.Name
		}
	}

	for _, buildVariant := range project.BuildVariants {
		buildVariantTasks := map[string]bool{}
		for _, task := range buildVariant.Tasks {
			if task.TaskGroup != nil {
				for _, taskGroupTask := range task.TaskGroup.Tasks {
					errs = append(errs, validateBuildVariantTaskNames(taskGroupTask, buildVariant.Name, allTaskNames, taskGroupTaskSet)...)
				}
			}
			errs = append(errs, validateBuildVariantTaskNames(task.Name, buildVariant.Name, allTaskNames, taskGroupTaskSet)...)
			if _, ok := taskGroupTaskSet[task.Name]; ok {
				errs = append(errs,
					ValidationError{
						Message: fmt.Sprintf("task '%s' in build variant '%s' is already referenced in task group '%s'",
							task.Name, buildVariant.Name, taskGroupTaskSet[task.Name]),
						Level: Warning,
					})
			}
			buildVariantTasks[task.Name] = true
			runOnHasDistro := false
			runOnHasContainer := false
			for _, name := range task.RunOn {
				if !utility.StringSliceContains(distroIDs, name) && !utility.StringSliceContains(distroAliases, name) && !containerNameMap[name] {
					errs = append(errs,
						ValidationError{
							Message: fmt.Sprintf("task '%s' in buildvariant '%s' references a nonexistent distro or container named '%s'",
								task.Name, buildVariant.Name, name),
							Level: Warning,
						},
					)
				} else if utility.StringSliceContains(distroIDs, name) && containerNameMap[name] {
					errs = append(errs,
						ValidationError{
							Message: fmt.Sprintf("task '%s' in buildvariant '%s' "+
								"references a container name overlapping with an existing distro '%s', the container "+
								"configuration will override the distro",
								task.Name, buildVariant.Name, name),
							Level: Warning,
						},
					)
				}
				if utility.StringSliceContains(distroIDs, name) {
					runOnHasDistro = true
				}
				if containerNameMap[name] {
					runOnHasContainer = true
				}
			}
			errs = append(errs, checkRunOn(runOnHasDistro, runOnHasContainer, task.RunOn)...)
		}
		runOnHasDistro := false
		runOnHasContainer := false
		for _, name := range buildVariant.RunOn {
			if !utility.StringSliceContains(distroIDs, name) && !utility.StringSliceContains(distroAliases, name) && !containerNameMap[name] {
				errs = append(errs,
					ValidationError{
						Message: fmt.Sprintf("buildvariant '%s' references a nonexistent distro or container named '%s'",
							buildVariant.Name, name),
						Level: Warning,
					},
				)
			} else if utility.StringSliceContains(distroIDs, name) && containerNameMap[name] {
				errs = append(errs,
					ValidationError{
						Message: fmt.Sprintf("buildvariant '%s' "+
							"references a container name overlapping with an existing distro '%s', the container "+
							"configuration will override the distro",
							buildVariant.Name, name),
						Level: Warning,
					},
				)
			}
			if utility.StringSliceContains(distroIDs, name) {
				runOnHasDistro = true
			}
			if containerNameMap[name] {
				runOnHasContainer = true
			}
		}
		errs = append(errs, checkRunOn(runOnHasDistro, runOnHasContainer, buildVariant.RunOn)...)
	}
	return errs
}

func checkRunOn(runOnHasDistro, runOnHasContainer bool, runOn []string) []ValidationError {
	if runOnHasContainer && runOnHasDistro {
		return []ValidationError{{
			Message: "run_on cannot contain a mixture of containers and distros",
			Level:   Error,
		}}

	} else if runOnHasContainer && len(runOn) > 1 {
		return []ValidationError{{
			Message: "only one container can be used from run_on; the first container in the list will be used",
			Level:   Warning,
		}}
	}
	return nil
}

// validateTaskNames ensures the task names do not contain unauthorized characters.
func validateTaskNames(project *model.Project) ValidationErrors {
	unauthorizedTaskCharacters := unauthorizedCharacters + " "
	errs := ValidationErrors{}
	for _, task := range project.Tasks {
		if strings.ContainsAny(strings.TrimSpace(task.Name), unauthorizedTaskCharacters) {
			errs = append(errs,
				ValidationError{
					Message: fmt.Sprintf("task name '%s' contains unauthorized characters ('%s')",
						task.Name, unauthorizedTaskCharacters),
				})
		}
	}
	return errs
}

func checkTaskNames(project *model.Project, task *model.ProjectTask) ValidationErrors {
	errs := ValidationErrors{}
	// Warn against commas because the CLI allows users to specify
	// tasks separated by commas in their patches.
	if strings.Contains(task.Name, ",") {
		errs = append(errs, ValidationError{
			Level:   Warning,
			Message: fmt.Sprintf("task name '%s' should not contain commas", task.Name),
		})
	}
	// Warn against using "*" since it is ambiguous with the
	// all-dependencies specification (also "*").
	if task.Name == model.AllDependencies {
		errs = append(errs, ValidationError{
			Level:   Warning,
			Message: "task should not be named '*' because it is ambiguous with the all-dependencies '*' specification",
		})
	}
	// Warn against using "all" since it is ambiguous with the special "all"
	// task specifier when creating patches.
	if task.Name == "all" {
		errs = append(errs, ValidationError{
			Level:   Warning,
			Message: "task should not be named 'all' because it is ambiguous in task specifications for patches",
		})
	}
	return errs
}

// checkModules checks to make sure that the module's name, branch, and repo are correct.
func checkModules(project *model.Project) ValidationErrors {
	errs := ValidationErrors{}
	moduleNames := map[string]bool{}

	for _, module := range project.Modules {
		// Warn if name is a duplicate or empty
		if module.Name == "" {
			errs = append(errs, ValidationError{
				Level:   Warning,
				Message: "module cannot have an empty name",
			})
		} else if moduleNames[module.Name] {
			errs = append(errs, ValidationError{
				Level:   Warning,
				Message: fmt.Sprintf("module '%s' already exists; the first module name defined will be used", module.Name),
			})
		} else {
			moduleNames[module.Name] = true
		}

		// Warn if branch is empty
		if module.Branch == "" {
			errs = append(errs, ValidationError{
				Level:   Warning,
				Message: fmt.Sprintf("module '%s' should have a set branch", module.Name),
			})
		}

		// Warn if repo is empty or does not conform to Git URL format
		owner, repo, err := thirdparty.ParseGitUrl(module.Repo)
		if err != nil {
			errs = append(errs, ValidationError{
				Level:   Warning,
				Message: errors.Wrapf(err, "module '%s' does not have a valid repo URL format", module.Name).Error(),
			})
		} else if owner == "" || repo == "" {
			errs = append(errs, ValidationError{
				Level:   Warning,
				Message: fmt.Sprintf("module '%s' repo '%s' is missing an owner or repo name", module.Name, module.Repo),
			})
		}
	}

	return errs
}

// Ensures there aren't any duplicate buildvariant names specified in the given
// project and that the names do not contain unauthorized characters.
func validateBVNames(project *model.Project) ValidationErrors {
	errs := ValidationErrors{}
	buildVariantNames := map[string]bool{}

	for _, buildVariant := range project.BuildVariants {
		if _, ok := buildVariantNames[buildVariant.Name]; ok {
			errs = append(errs,
				ValidationError{
					Message: fmt.Sprintf("buildvariant '%s' already exists", buildVariant.Name),
				},
			)
		}
		buildVariantNames[buildVariant.Name] = true
		dispName := buildVariant.DisplayName
		if dispName == "" {
			errs = append(errs,
				ValidationError{
					Message: fmt.Sprintf("buildvariant '%s' does not have a display name", buildVariant.Name),
				},
			)
		} else if dispName == evergreen.MergeTaskVariant {
			errs = append(errs, ValidationError{
				Message: fmt.Sprintf("the variant name '%s' is reserved for the commit queue", evergreen.MergeTaskVariant),
			})
		}

		if strings.ContainsAny(buildVariant.Name, unauthorizedCharacters) {
			errs = append(errs,
				ValidationError{
					Message: fmt.Sprintf("buildvariant name '%s' contains unauthorized characters (%s)",
						buildVariant.Name, unauthorizedCharacters),
				})
		}

	}

	return errs
}

func checkBVNames(buildVariant *model.BuildVariant) ValidationErrors {
	errs := ValidationErrors{}

	// Warn against commas because the CLI allows users to specify
	// variants separated by commas in their patches.
	if strings.Contains(buildVariant.Name, ",") {
		errs = append(errs, ValidationError{
			Level:   Warning,
			Message: fmt.Sprintf("buildvariant name '%s' should not contains commas", buildVariant.Name),
		})
	}
	// Warn against using "*" since it is ambiguous with the
	// all-dependencies specification (also "*").
	if buildVariant.Name == model.AllVariants {
		errs = append(errs, ValidationError{
			Level:   Warning,
			Message: "buildvariant should not be named '*' because it is ambiguous with the all-variants '*' specification",
		})
	}
	// Warn against using "all" since it is ambiguous with the special "all"
	// task specifier when creating patches.
	if buildVariant.Name == "all" {
		errs = append(errs, ValidationError{
			Level:   Warning,
			Message: "buildvariant should not be named 'all' because it is ambiguous in buildvariant specifications for patches",
		})
	}

	return errs
}

func checkLoggerConfig(task *model.ProjectTask) ValidationErrors {
	errs := ValidationErrors{}

	for _, command := range task.Commands {
		if err := command.Loggers.IsValid(); err != nil {
			errs = append(errs, ValidationError{
				Message: errors.Wrapf(err, "error in logger config for command %s in task %s", command.DisplayName, task.Name).Error(),
				Level:   Warning,
			})
		}
	}

	return errs
}

// Ensures there aren't any duplicate task names specified for any buildvariant
// in this project
func validateBVTaskNames(project *model.Project) ValidationErrors {
	errs := ValidationErrors{}
	for _, buildVariant := range project.BuildVariants {
		buildVariantTasks := map[string]bool{}
		for _, task := range buildVariant.Tasks {
			if _, ok := buildVariantTasks[task.Name]; ok {
				errs = append(errs,
					ValidationError{
						Message: fmt.Sprintf("task '%s' in buildvariant '%s' already exists",
							task.Name, buildVariant.Name),
					},
				)
			}
			buildVariantTasks[task.Name] = true
		}
	}
	return errs
}

func validateBVBatchTimes(project *model.Project) ValidationErrors {
	errs := ValidationErrors{}
	for _, buildVariant := range project.BuildVariants {
		// check task batchtimes first
		for _, t := range buildVariant.Tasks {

			if t.CronBatchTime == "" {
				continue
			}
			// otherwise, cron batchtime is set
			if t.BatchTime != nil {
				errs = append(errs,
					ValidationError{
						Message: fmt.Sprintf("task '%s' cannot specify cron and batchtime for variant '%s'", t.Name, buildVariant.Name),
						Level:   Error,
					})
			}
			if _, err := model.GetNextCronTime(time.Now(), t.CronBatchTime); err != nil {
				errs = append(errs,
					ValidationError{
						Message: errors.Wrapf(err, "task cron batchtime '%s' has invalid syntax for task '%s' for build variant '%s'",
							t.CronBatchTime, t.Name, buildVariant.Name).Error(),
						Level: Error,
					},
				)
			}
		}

		if buildVariant.CronBatchTime == "" {
			continue
		}
		if buildVariant.BatchTime != nil {
			errs = append(errs,
				ValidationError{
					Message: fmt.Sprintf("variant '%s' cannot specify cron and batchtime", buildVariant.Name),
					Level:   Error,
				})
		}
		if _, err := model.GetNextCronTime(time.Now(), buildVariant.CronBatchTime); err != nil {
			errs = append(errs,
				ValidationError{
					Message: errors.Wrapf(err, "cron batchtime '%s' has invalid syntax", buildVariant.CronBatchTime).Error(),
					Level:   Error,
				},
			)
		}
	}
	return errs
}

func checkBVBatchTimes(buildVariant *model.BuildVariant) ValidationErrors {
	errs := ValidationErrors{}
	// check task batchtimes first
	for _, t := range buildVariant.Tasks {
		// setting explicitly to true with batchtime will use batchtime
		if utility.FromBoolPtr(t.Activate) && (t.CronBatchTime != "" || t.BatchTime != nil) {
			errs = append(errs,
				ValidationError{
					Message: fmt.Sprintf("task '%s' for variant '%s' activation ignored since batchtime specified",
						t.Name, buildVariant.Name),
					Level: Warning,
				})
		}

	}

	if utility.FromBoolPtr(buildVariant.Activate) && (buildVariant.CronBatchTime != "" || buildVariant.BatchTime != nil) {
		errs = append(errs,
			ValidationError{
				Message: fmt.Sprintf("variant '%s' activation ignored since batchtime specified", buildVariant.Name),
				Level:   Warning,
			})
	}

	return errs
}

func validateDisplayTaskNames(project *model.Project) ValidationErrors {
	errs := ValidationErrors{}

	// build a map of task names
	tn := map[string]struct{}{}
	for _, t := range project.Tasks {
		tn[t.Name] = struct{}{}
	}

	// check display tasks
	for _, bv := range project.BuildVariants {
		for _, dp := range bv.DisplayTasks {
			for _, etn := range dp.ExecTasks {
				if strings.HasPrefix(etn, "display_") {
					errs = append(errs,
						ValidationError{
							Level:   Error,
							Message: fmt.Sprintf("execution task '%s' has prefix 'display_' which is invalid", etn),
						})
				}
			}
		}
	}
	return errs
}

// Helper for validating a set of plugin commands given a project/registry
func validateCommands(section string, project *model.Project,
	commands []model.PluginCommandConf) ValidationErrors {
	errs := ValidationErrors{}

	for _, cmd := range commands {
		commandName := fmt.Sprintf("'%s' command", cmd.Command)
		_, err := command.Render(cmd, project, "")
		if err != nil {
			if cmd.Function != "" {
				commandName = fmt.Sprintf("'%s' function", cmd.Function)
			}
			errs = append(errs, ValidationError{Message: fmt.Sprintf("%s section in %s: %s", section, commandName, err)})
		}
		if cmd.Type != "" {
			if !utility.StringSliceContains(evergreen.ValidCommandTypes, cmd.Type) {
				msg := fmt.Sprintf("%s section in '%s': invalid command type: '%s'", section, commandName, cmd.Type)
				errs = append(errs, ValidationError{Message: msg})
			}
		}
		if cmd.Function != "" && cmd.Command != "" {
			errs = append(errs, ValidationError{
				Level:   Error,
				Message: fmt.Sprintf("cannot specify both command '%s' and function '%s'", cmd.Command, cmd.Function),
			})
		}
	}
	return errs
}

// Ensures there any plugin commands referenced in a project's configuration
// are specified in a valid format
func validatePluginCommands(project *model.Project) ValidationErrors {
	errs := ValidationErrors{}
	seen := make(map[string]bool)

	// validate each function definition
	for funcName, commands := range project.Functions {
		if commands == nil || len(commands.List()) == 0 {
			errs = append(errs,
				ValidationError{
					Message: fmt.Sprintf("'%s' function contains no commands", funcName),
					Level:   Error,
				},
			)
			continue
		}
		valErrs := validateCommands("functions", project, commands.List())
		for _, err := range valErrs {
			errs = append(errs,
				ValidationError{
					Message: fmt.Sprintf("'%s' definition error: %s", funcName, err.Message),
					Level:   err.Level,
				},
			)
		}

		for _, c := range commands.List() {
			if c.Function != "" {
				errs = append(errs,
					ValidationError{
						Message: fmt.Sprintf("can not reference a function within a "+
							"function: '%s' referenced within '%s'", c.Function, funcName),
					},
				)

			}
		}

		// this checks for duplicate function definitions in the project.
		if seen[funcName] {
			errs = append(errs,
				ValidationError{
					Message: fmt.Sprintf(`duplicate definition of "%s"`, funcName),
				},
			)
		}
		seen[funcName] = true
	}

	if project.Pre != nil {
		errs = append(errs, validateCommands("pre", project, project.Pre.List())...)
	}

	if project.Post != nil {
		errs = append(errs, validateCommands("post", project, project.Post.List())...)
	}

	if project.Timeout != nil {
		errs = append(errs, validateCommands("timeout", project, project.Timeout.List())...)
	}

	if project.EarlyTermination != nil {
		errs = append(errs, ValidationError{
			Message: "early_termination block is deprecated and will be removed in the future",
			Level:   Warning,
		})
	}

	// validate project tasks section
	for _, task := range project.Tasks {
		errs = append(errs, validateCommands("tasks", project, task.Commands)...)
	}
	return errs
}

// Ensures there aren't any duplicate task names for this project
func validateProjectTaskNames(project *model.Project) ValidationErrors {
	errs := ValidationErrors{}
	// create a map to hold the task names
	taskNames := map[string]bool{}
	for _, task := range project.Tasks {
		if _, ok := taskNames[task.Name]; ok {
			errs = append(errs,
				ValidationError{
					Message: fmt.Sprintf("task '%s' already exists", task.Name),
				},
			)
		}
		taskNames[task.Name] = true
	}
	return errs
}

// validateProjectTaskIdsAndTags ensures that task tags and ids only contain valid characters
func validateProjectTaskIdsAndTags(project *model.Project) ValidationErrors {
	errs := ValidationErrors{}
	// create a map to hold the task names
	for _, task := range project.Tasks {
		// check task name
		if i := strings.IndexAny(task.Name, model.InvalidCriterionRunes); i == 0 {
			errs = append(errs, ValidationError{
				Message: fmt.Sprintf("task '%s' has invalid name: starts with invalid character %s",
					task.Name, strconv.QuoteRune(rune(task.Name[0])))})
		}
		// check tag names
		for _, tag := range task.Tags {
			if i := strings.IndexAny(tag, model.InvalidCriterionRunes); i == 0 {
				errs = append(errs, ValidationError{
					Message: fmt.Sprintf("task '%s' has invalid tag '%s': starts with invalid character %s",
						task.Name, tag, strconv.QuoteRune(rune(tag[0])))})
			}
			if i := util.IndexWhiteSpace(tag); i != -1 {
				errs = append(errs, ValidationError{
					Message: fmt.Sprintf("task '%s' has invalid tag '%s': tag contains white space",
						task.Name, tag)})
			}
		}
	}
	return errs
}

func checkTaskRuns(project *model.Project) ValidationErrors {
	var errs ValidationErrors
	for _, bvtu := range project.FindAllBuildVariantTasks() {
		if bvtu.SkipOnPatchBuild() && bvtu.SkipOnNonPatchBuild() {
			errs = append(errs, ValidationError{
				Level: Warning,
				Message: fmt.Sprintf("task '%s' will never run because it skips both patch builds and non-patch builds",
					bvtu.Name),
			})
		}
		if bvtu.SkipOnGitTagBuild() && bvtu.SkipOnNonGitTagBuild() {
			errs = append(errs, ValidationError{
				Level: Warning,
				Message: fmt.Sprintf("task '%s' will never run because it skips both git tag builds and non git tag builds",
					bvtu.Name),
			})
		}
		// Git-tag-only builds cannot run in patches.
		if bvtu.SkipOnNonGitTagBuild() && bvtu.SkipOnNonPatchBuild() {
			errs = append(errs, ValidationError{
				Level: Warning,
				Message: fmt.Sprintf("task '%s' will never run because it only runs for git tag builds but also is patch-only",
					bvtu.Name),
			})
		}
		if bvtu.SkipOnNonGitTagBuild() && utility.FromBoolPtr(bvtu.Patchable) {
			errs = append(errs, ValidationError{
				Level: Warning,
				Message: fmt.Sprintf("task '%s' cannot be patchable if it only runs for git tag builds",
					bvtu.Name),
			})
		}
	}
	return errs
}

// validateTaskDependencies ensures that the dependencies for the tasks have the
// correct fields, and that the fields have valid values
func validateTaskDependencies(project *model.Project) ValidationErrors {
	errs := ValidationErrors{}
	for _, task := range project.Tasks {
		// create a set of the dependencies, to check for duplicates
		depNames := map[model.TVPair]bool{}

		for _, dep := range task.DependsOn {
			pair := model.TVPair{TaskName: dep.Name, Variant: dep.Variant}
			// make sure the dependency is not specified more than once
			if depNames[pair] {
				errs = append(errs,
					ValidationError{
						Message: fmt.Sprintf("duplicate dependency '%s' specified for task '%s'",
							dep.Name, task.Name),
					},
				)
			}
			depNames[pair] = true

			// check that the status is valid
			switch dep.Status {
			case evergreen.TaskSucceeded, evergreen.TaskFailed, model.AllStatuses, "":
				// these are all valid
			default:
				errs = append(errs,
					ValidationError{
						Message: fmt.Sprintf("invalid dependency status for task '%s': %s",
							task.Name, dep.Status)})
			}

			// check that name of the dependency task is valid
			if dep.Name != model.AllDependencies && project.FindProjectTask(dep.Name) == nil {
				errs = append(errs,
					ValidationError{
						Level: Error,
						Message: fmt.Sprintf("non-existent task name '%s' in dependencies for task '%s'",
							dep.Name, task.Name),
					},
				)
			}
			if dep.Variant != "" && dep.Variant != model.AllVariants && project.FindBuildVariant(dep.Variant) == nil {
				errs = append(errs, ValidationError{
					Level: Error,
					Message: fmt.Sprintf("non-existent variant name '%s' in dependencies for task '%s'",
						dep.Variant, task.Name),
				})
			}

		}
	}
	return errs
}

func checkTaskDependencies(task *model.ProjectTask, allTasks map[string]model.ProjectTask) ValidationErrors {
	errs := ValidationErrors{}

	for _, dep := range task.DependsOn {
		dependent, exists := allTasks[dep.Name]
		if !exists {
			continue
		}
		if utility.FromBoolPtr(dependent.PatchOnly) && !utility.FromBoolPtr(task.PatchOnly) {
			errs = append(errs, ValidationError{
				Level:   Warning,
				Message: fmt.Sprintf("Task '%s' depends on patch-only task '%s'. Both will only run in patches", task.Name, dep.Name),
			})
		}
		if !utility.FromBoolTPtr(dependent.Patchable) && utility.FromBoolTPtr(task.Patchable) {
			errs = append(errs, ValidationError{
				Level:   Warning,
				Message: fmt.Sprintf("Task '%s' depends on non-patchable task '%s'. Neither will run in patches", task.Name, dep.Name),
			})
		}
		if utility.FromBoolPtr(dependent.GitTagOnly) && !utility.FromBoolPtr(task.GitTagOnly) {
			errs = append(errs, ValidationError{
				Level:   Warning,
				Message: fmt.Sprintf("Task '%s' depends on git-tag-only task '%s'. Both will only run when pushing git tags", task.Name, dep.Name),
			})
		}
	}

	return errs
}

func validateParameters(p *model.Project) ValidationErrors {
	errs := ValidationErrors{}

	names := map[string]bool{}
	for _, param := range p.Parameters {
		if _, ok := names[param.Parameter.Key]; ok {
			errs = append(errs, ValidationError{
				Level:   Error,
				Message: fmt.Sprintf("parameter '%s' is defined multiple times", param.Parameter.Key),
			})
			names[param.Parameter.Key] = true
		}
		if strings.Contains(param.Parameter.Key, "=") {
			errs = append(errs, ValidationError{
				Level:   Error,
				Message: fmt.Sprintf("parameter name '%s' cannot contain `=`", param.Parameter.Key),
			})
		}
		if param.Parameter.Key == "" {
			errs = append(errs, ValidationError{
				Level:   Error,
				Message: "parameter name is missing",
			})
		}
	}
	return errs
}

func validateTaskGroups(p *model.Project) ValidationErrors {
	errs := ValidationErrors{}
	taskGroups := p.TaskGroups
	for _, bv := range p.BuildVariants {
		for _, t := range bv.Tasks {
			if t.TaskGroup != nil {
				taskGroups = append(taskGroups, *t.TaskGroup)
			}
		}
	}
	for _, tg := range taskGroups {
		// validate that there is at least 1 task
		if len(tg.Tasks) < 1 {
			errs = append(errs, ValidationError{
				Message: fmt.Sprintf("task group %s must have at least 1 task", tg.Name),
				Level:   Error,
			})
		}
		// validate that the task group is not named the same as a task
		for _, t := range p.Tasks {
			if t.Name == tg.Name {
				errs = append(errs, ValidationError{
					Message: fmt.Sprintf("%s is used as a name for both a task and task group", t.Name),
					Level:   Error,
				})
			}
		}
		// validate that a task is not listed twice in a task group
		counts := make(map[string]int)
		for _, name := range tg.Tasks {
			counts[name]++
		}
		for name, count := range counts {
			if count > 1 {
				errs = append(errs, ValidationError{
					Message: fmt.Sprintf("%s is listed in task group %s %d times", name, tg.Name, count),
					Level:   Error,
				})
			}
		}
		// validate that attach commands aren't used in the teardown_group phase
		if tg.TeardownGroup != nil {
			for _, cmd := range tg.TeardownGroup.List() {
				if utility.StringSliceContains(evergreen.AttachCommands, cmd.Command) {
					errs = append(errs, ValidationError{
						Message: fmt.Sprintf("%s cannot be used in the group teardown stage", cmd.Command),
						Level:   Error,
					})
				}
			}
		}
	}

	return errs
}

func checkTaskGroups(p *model.Project) ValidationErrors {
	errs := ValidationErrors{}
	tasksInTaskGroups := map[string]string{}
	names := map[string]bool{}
	taskGroups := p.TaskGroups
	for _, bv := range p.BuildVariants {
		for _, t := range bv.Tasks {
			if t.TaskGroup != nil {
				taskGroups = append(taskGroups, *t.TaskGroup)
			}
		}
	}
	for _, tg := range taskGroups {
		if _, ok := names[tg.Name]; ok {
			errs = append(errs, ValidationError{
				Level:   Warning,
				Message: fmt.Sprintf("task group '%s' is defined multiple times; only the first will be used", tg.Name),
			})
		}
		names[tg.Name] = true
		if tg.MaxHosts < 1 {
			errs = append(errs, ValidationError{
				Message: fmt.Sprintf("task group %s has number of hosts %d less than 1", tg.Name, tg.MaxHosts),
				Level:   Warning,
			})
		}
		if len(tg.Tasks) == 1 {
			continue
		}
		if tg.MaxHosts > len(tg.Tasks) {
			errs = append(errs, ValidationError{
				Message: fmt.Sprintf("task group %s has max number of hosts %d greater than the number of tasks %d", tg.Name, tg.MaxHosts, len(tg.Tasks)),
				Level:   Warning,
			})
		}
		for _, t := range tg.Tasks {
			tasksInTaskGroups[t] = tg.Name
		}
	}

	return errs
}

// validateDuplicateBVTasks ensures that no task is used multiple times
// in any given build variant.
func validateDuplicateBVTasks(p *model.Project) ValidationErrors {
	errors := []ValidationError{}

	for _, bv := range p.BuildVariants {
		tasksFound := map[string]interface{}{}
		for _, t := range bv.Tasks {

			if t.IsGroup {
				tg := t.TaskGroup
				if tg == nil {
					tg = p.FindTaskGroup(t.Name)
				}
				if tg == nil {
					continue
				}
				for _, tgTask := range tg.Tasks {
					err := checkOrAddTask(tgTask, bv.Name, tasksFound)
					if err != nil {
						errors = append(errors, *err)
					}
				}
			} else {
				err := checkOrAddTask(t.Name, bv.Name, tasksFound)
				if err != nil {
					errors = append(errors, *err)
				}
			}

		}
	}

	return errors
}

func checkOrAddTask(task, variant string, tasksFound map[string]interface{}) *ValidationError {
	if _, found := tasksFound[task]; found {
		return &ValidationError{
			Message: fmt.Sprintf("task '%s' in '%s' is listed more than once, likely through a task group", task, variant),
			Level:   Error,
		}
	}
	tasksFound[task] = nil
	return nil
}

func validateHostCreates(p *model.Project) ValidationErrors {
	counts := tasksThatCallHostCreateByProvider(p)
	errs := validateTimesCalledPerTask(p, counts.All, evergreen.HostCreateCommandName, HostCreateLimitPerTask, Error)
	errs = append(errs, validateHostCreateTotals(p, counts)...)
	return errs
}

type hostCreateCounts struct {
	Docker map[string]int
	EC2    map[string]int
	All    map[string]int
}

// tasksThatCallHostCreateByProvider is similar to TasksThatCallCommand in the model package, except the output is
// split into host.create for Docker hosts and host.create for non-docker hosts, so limits can be validated separately.
func tasksThatCallHostCreateByProvider(p *model.Project) hostCreateCounts {
	// get all functions that call the command.
	ec2Fs := map[string]int{}
	dockerFs := map[string]int{}
	for f, cmds := range p.Functions {
		if cmds == nil {
			continue
		}
		for _, c := range cmds.List() {
			if c.Command == evergreen.HostCreateCommandName {
				provider, ok := c.Params["provider"]
				if ok && provider.(string) == evergreen.ProviderNameDocker {
					dockerFs[f] += 1
				} else {
					ec2Fs[f] += 1
				}
			}
		}
	}

	// get all tasks that call the command.
	counts := hostCreateCounts{
		Docker: map[string]int{},
		EC2:    map[string]int{},
		All:    map[string]int{},
	}
	for _, t := range p.Tasks {
		for _, c := range t.Commands {
			if c.Function != "" {
				if times, ok := ec2Fs[c.Function]; ok {
					counts.EC2[t.Name] += times
					counts.All[t.Name] += times
				}
				if times, ok := dockerFs[c.Function]; ok {
					counts.Docker[t.Name] += times
					counts.All[t.Name] += times
				}
			}
			if c.Command == evergreen.HostCreateCommandName {
				provider, ok := c.Params["provider"]
				if ok && provider.(string) == evergreen.ProviderNameDocker {
					counts.Docker[t.Name] += 1
					counts.All[t.Name] += 1
				} else {
					counts.EC2[t.Name] += 1
					counts.All[t.Name] += 1
				}
			}
		}
	}

	return counts
}

func validateTimesCalledPerTask(p *model.Project, ts map[string]int, commandName string, times int, level ValidationErrorLevel) ValidationErrors {
	errs := ValidationErrors{}
	for _, bv := range p.BuildVariants {
		for _, t := range bv.Tasks {
			if count, ok := ts[t.Name]; ok {
				if count > times {
					errs = append(errs, ValidationError{
						Message: fmt.Sprintf("build variant '%s' with task '%s' may only call %s %d time(s) but calls it %d time(s)", bv.Name, t.Name, commandName, times, count),
						Level:   level,
					})
				}
			}
		}
	}
	return errs
}

func validateHostCreateTotals(p *model.Project, counts hostCreateCounts) ValidationErrors {
	errs := ValidationErrors{}
	dockerTotal := 0
	ec2Total := 0
	errorFmt := "project config may only call %s %s %d time(s) but it is called %d time(s)"
	for _, bv := range p.BuildVariants {
		for _, t := range bv.Tasks {
			dockerTotal += counts.Docker[t.Name]
			ec2Total += counts.EC2[t.Name]
		}
	}
	if ec2Total > EC2HostCreateTotalLimit {
		errs = append(errs, ValidationError{
			Message: fmt.Sprintf(errorFmt, "ec2", evergreen.HostCreateCommandName, EC2HostCreateTotalLimit, ec2Total),
			Level:   Error,
		})
	}
	if dockerTotal > DockerHostCreateTotalLimit {
		errs = append(errs, ValidationError{
			Message: fmt.Sprintf(errorFmt, "docker", evergreen.HostCreateCommandName, DockerHostCreateTotalLimit, dockerTotal),
			Level:   Error,
		})
	}
	return errs
}

// validateGenerateTasks validates that no task calls 'generate.tasks' more than once, since if one
// does, the server will noop it.
func validateGenerateTasks(p *model.Project) ValidationErrors {
	ts := p.TasksThatCallCommand(evergreen.GenerateTasksCommandName)
	return validateTimesCalledPerTask(p, ts, evergreen.GenerateTasksCommandName, 1, Error)
}

// validateTaskSyncSettings checks that task sync in the project settings have
// enabled task sync for the config.
func validateTaskSyncSettings(_ *evergreen.Settings, p *model.Project, ref *model.ProjectRef, _ bool) ValidationErrors {
	if ref.TaskSync.IsConfigEnabled() {
		return nil
	}
	var errs ValidationErrors
	if s3PushCalls := p.TasksThatCallCommand(evergreen.S3PushCommandName); len(s3PushCalls) != 0 {
		errs = append(errs, ValidationError{
			Level: Error,
			Message: fmt.Sprintf("cannot use %s command in project config when it is disabled by project '%s' settings",
				ref.Identifier, evergreen.S3PushCommandName),
		})
	}
	if s3PullCalls := p.TasksThatCallCommand(evergreen.S3PullCommandName); len(s3PullCalls) != 0 {
		errs = append(errs, ValidationError{
			Level: Error,
			Message: fmt.Sprintf("cannot use %s command in project config when it is disabled by project '%s' settings",
				ref.Identifier, evergreen.S3PullCommandName),
		})
	}
	return errs
}

// validateVersionControl checks if a project with defined project config fields has version control enabled on the project ref.
func validateVersionControl(_ *evergreen.Settings, _ *model.Project, ref *model.ProjectRef, isConfigDefined bool) ValidationErrors {
	var errs ValidationErrors
	if ref.IsVersionControlEnabled() && !isConfigDefined {
		errs = append(errs, ValidationError{
			Level: Warning,
			Message: fmt.Sprintf("version control is enabled for project '%s' but no project config fields have been set.",
				ref.Identifier),
		})
	} else if !ref.IsVersionControlEnabled() && isConfigDefined {
		errs = append(errs, ValidationError{
			Level: Warning,
			Message: fmt.Sprintf("version control is disabled for project '%s'; the currently defined project config fields will not be picked up",
				ref.Identifier),
		})
	}
	return errs
}

// bvsWithTasksThatCallCommand creates a mapping from build variants to tasks
// that run the given command cmd, including the list of matching commands for
// each task. Returns the total number of commands in the map.
func bvsWithTasksThatCallCommand(p *model.Project, cmd string) (map[string]map[string][]model.PluginCommandConf, int, error) {
	// build variant -> tasks that run cmd -> all matching commands
	bvToTasksWithCmds := map[string]map[string][]model.PluginCommandConf{}
	numCmds := 0
	catcher := grip.NewBasicCatcher()

	// addCmdsForTaskInBV adds commands that run for a task in a build variant
	// to the mapping.
	addCmdsForTaskInBV := func(bvToTaskWithCmds map[string]map[string][]model.PluginCommandConf, bv, taskUnit string, cmds []model.PluginCommandConf) {
		if len(cmds) == 0 {
			return
		}
		if _, ok := bvToTaskWithCmds[bv]; !ok {
			bvToTasksWithCmds[bv] = map[string][]model.PluginCommandConf{}
		}
		bvToTasksWithCmds[bv][taskUnit] = append(bvToTasksWithCmds[bv][taskUnit], cmds...)
		numCmds += len(cmds)
	}

	for _, bv := range p.BuildVariants {
		var preAndPostCmds []model.PluginCommandConf
		if p.Pre != nil {
			preAndPostCmds = append(preAndPostCmds, p.CommandsRunOnBV(p.Pre.List(), cmd, bv.Name)...)
		}
		if p.Post != nil {
			preAndPostCmds = append(preAndPostCmds, p.CommandsRunOnBV(p.Post.List(), cmd, bv.Name)...)
		}

		for _, bvtu := range bv.Tasks {
			if bvtu.IsGroup {
				tg := bvtu.TaskGroup
				if tg == nil {
					tg = p.FindTaskGroup(bvtu.Name)
				}
				if tg == nil {
					catcher.Errorf("cannot find definition of task group '%s' used in build variant '%s'", bvtu.Name, bv.Name)
					continue
				}
				// All setup/teardown commands that apply for this build variant
				// will run for this task.
				var setupAndTeardownCmds []model.PluginCommandConf
				if tg.SetupGroup != nil {
					setupAndTeardownCmds = append(setupAndTeardownCmds, p.CommandsRunOnBV(tg.SetupGroup.List(), cmd, bv.Name)...)
				}
				if tg.SetupTask != nil {
					setupAndTeardownCmds = append(setupAndTeardownCmds, p.CommandsRunOnBV(tg.SetupTask.List(), cmd, bv.Name)...)
				}
				if tg.TeardownGroup != nil {
					setupAndTeardownCmds = append(setupAndTeardownCmds, p.CommandsRunOnBV(tg.TeardownGroup.List(), cmd, bv.Name)...)
				}
				if tg.TeardownTask != nil {
					setupAndTeardownCmds = append(setupAndTeardownCmds, p.CommandsRunOnBV(tg.TeardownTask.List(), cmd, bv.Name)...)
				}
				for _, tgTask := range model.CreateTasksFromGroup(bvtu, p, "") {
					addCmdsForTaskInBV(bvToTasksWithCmds, bv.Name, tgTask.Name, setupAndTeardownCmds)
					if projTask := p.FindProjectTask(tgTask.Name); projTask != nil {
						cmds := p.CommandsRunOnBV(projTask.Commands, cmd, bv.Name)
						addCmdsForTaskInBV(bvToTasksWithCmds, bv.Name, tgTask.Name, cmds)
					} else {
						catcher.Errorf("cannot find definition of task '%s' used in task group '%s'", tgTask.Name, tg.Name)
					}
				}
			} else {
				// All pre/post commands that apply for this build variant will
				// run for this task.
				addCmdsForTaskInBV(bvToTasksWithCmds, bv.Name, bvtu.Name, preAndPostCmds)

				projTask := p.FindProjectTask(bvtu.Name)
				if projTask == nil {
					catcher.Errorf("cannot find definition of task '%s'", bvtu.Name)
					continue
				}
				cmds := p.CommandsRunOnBV(projTask.Commands, cmd, bv.Name)
				addCmdsForTaskInBV(bvToTasksWithCmds, bv.Name, bvtu.Name, cmds)
			}
		}
	}
	return bvToTasksWithCmds, numCmds, catcher.Resolve()
}

// validateTaskSyncCommands validates project's task sync commands.  In
// particular, s3.push should be called at most once per task and s3.pull should
// refer to a valid task running s3.push.  It does not check that the project
// settings allow task syncing - see validateTaskSyncSettings. If run long isn't set,
// we don't validate dependencies if there are too many commands.
func validateTaskSyncCommands(p *model.Project, runLong bool) ValidationErrors {
	errs := ValidationErrors{}

	// A task should not call s3.push multiple times.
	s3PushCalls := p.TasksThatCallCommand(evergreen.S3PushCommandName)
	errs = append(errs, validateTimesCalledPerTask(p, s3PushCalls, evergreen.S3PushCommandName, 1, Warning)...)

	bvToTaskCmds, numCmds, err := bvsWithTasksThatCallCommand(p, evergreen.S3PullCommandName)
	if err != nil {
		errs = append(errs, ValidationError{
			Level:   Error,
			Message: fmt.Sprintf("could not generate map of build variants with tasks that call command '%s': %s", evergreen.S3PullCommandName, err.Error()),
		})
	}

	checkDependencies := numCmds <= maxTaskSyncCommandsForDependenciesCheck || runLong
	if !checkDependencies {
		errs = append(errs, ValidationError{
			Level:   Warning,
			Message: fmt.Sprintf("too many commands using '%s' to check dependencies by default", evergreen.S3PullCommandName),
		})
	}
	for bv, taskCmds := range bvToTaskCmds {
		for task, cmds := range taskCmds {
			for _, cmd := range cmds {
				// This is only possible because we disallow expansions for the
				// task and build variant for s3.pull, which would prevent
				// evaluation of dependencies.
				s3PushTaskName, s3PushBVName, parseErr := parseS3PullParameters(cmd)
				if parseErr != nil {
					errs = append(errs, ValidationError{
						Level:   Error,
						Message: fmt.Sprintf("could not parse parameters for command '%s': %s", cmd.Command, parseErr.Error()),
					})
					continue
				}

				// If no build variant is explicitly stated, the build variant
				// is the same as the build variant of the task running s3.pull.
				if s3PushBVName == "" {
					s3PushBVName = bv
				}

				// Since s3.pull depends on the task running s3.push to run
				// first, ensure that this task for this build variant has a
				// dependency on the referenced task and build variant.
				s3PushTaskNode := model.TVPair{TaskName: s3PushTaskName, Variant: s3PushBVName}
				if checkDependencies {
					s3PullTaskNode := model.TVPair{TaskName: task, Variant: bv}
					if err := validateTVDependsOnTV(s3PullTaskNode, s3PushTaskNode, []string{"", evergreen.TaskSucceeded}, p); err != nil {
						errs = append(errs, ValidationError{
							Level: Error,
							Message: fmt.Sprintf("problem validating that task running command '%s' depends on task running command '%s': %s",
								evergreen.S3PullCommandName, evergreen.S3PushCommandName, err.Error()),
						})
					}
				}

				// Find the task referenced by s3.pull and ensure that it exists
				// and calls s3.push.
				cmds, err := p.CommandsRunOnTV(s3PushTaskNode, evergreen.S3PushCommandName)
				if err != nil {
					errs = append(errs, ValidationError{
						Level: Error,
						Message: fmt.Sprintf("problem validating that task '%s' runs command '%s': %s",
							s3PushTaskName, evergreen.S3PushCommandName, err.Error()),
					})
				} else if len(cmds) == 0 {
					errs = append(errs, ValidationError{
						Level: Error,
						Message: fmt.Sprintf("task '%s' in build variant '%s' does not run command '%s'",
							s3PushTaskName, s3PushBVName, evergreen.S3PushCommandName),
					})
				}
			}
		}
	}

	return errs
}

// validateTVDependsOnTV checks that the dependent task always has a dependency on the depended on task.
// The dependedOnTask and every other task along the path must run on all the same requester types as the dependentTask
// and the dependency on the dependedOnTask must be with a status in statuses, if provided.
func validateTVDependsOnTV(dependentTask, dependedOnTask model.TVPair, statuses []string, project *model.Project) error {
	g := project.DependencyGraph()
	tvTaskUnitMap := tvToTaskUnit(project)

	startNode := task.TaskNode{Name: dependentTask.TaskName, Variant: dependentTask.Variant}
	targetNode := task.TaskNode{Name: dependedOnTask.TaskName, Variant: dependedOnTask.Variant}

	// The traversal function returns whether the current edge should be traversed by the DFS.
	traversal := func(edge task.DependencyEdge) bool {
		from := edge.From
		to := edge.To

		fromTaskUnit := tvTaskUnitMap[model.TVPair{TaskName: from.Name, Variant: from.Variant}]
		toTaskUnit := tvTaskUnitMap[model.TVPair{TaskName: to.Name, Variant: to.Variant}]

		var edgeInfo model.TaskUnitDependency
		for _, dependency := range fromTaskUnit.DependsOn {
			if dependency.Name == to.Name && dependency.Variant == to.Variant {
				edgeInfo = dependency
			}
		}

		// PatchOptional dependencies are skipped when the fromTaskUnit task is running on a patch.
		if edgeInfo.PatchOptional && !(fromTaskUnit.SkipOnPatchBuild() || fromTaskUnit.SkipOnNonGitTagBuild()) {
			return false
		}

		// The dependency is skipped if toTaskUnit doesn't run on all the same requester types that fromTaskUnit runs on.
		for _, rType := range evergreen.AllRequesterTypes {
			if !fromTaskUnit.SkipOnRequester(rType) && toTaskUnit.SkipOnRequester(rType) {
				return false
			}
		}

		// If statuses is specified we need to check the edge's status when the edge points to the target node.
		if statuses != nil && to == targetNode {
			return utility.StringSliceContains(statuses, edgeInfo.Status)
		}

		return true
	}

	if found := g.DepthFirstSearch(startNode, targetNode, traversal); !found {
		dependentBVTask := tvTaskUnitMap[dependentTask]
		runsOnPatches := !(dependentBVTask.SkipOnPatchBuild() || dependentBVTask.SkipOnNonGitTagBuild())
		runsOnNonPatches := !(dependentBVTask.SkipOnNonPatchBuild() || dependentBVTask.SkipOnNonGitTagBuild())
		runsOnGitTag := !(dependentBVTask.SkipOnNonPatchBuild() || dependentBVTask.SkipOnGitTagBuild())

		errMsg := "task '%s' in build variant '%s' must depend on" +
			" task '%s' in build variant '%s' completing"
		if runsOnPatches && runsOnNonPatches {
			errMsg += " for both patches and non-patches"
		} else if runsOnPatches {
			errMsg += " for patches"
		} else if runsOnNonPatches {
			errMsg += " for non-patches"
		} else if runsOnGitTag {
			errMsg += " for git-tag builds"
		}
		errMsg = fmt.Sprintf(errMsg, dependentTask.TaskName, dependentTask.Variant, dependedOnTask.TaskName, dependedOnTask.Variant)

		if statuses != nil {
			errMsg = fmt.Sprintf("%s with status in [%s]", errMsg, strings.Join(statuses, ", "))
		}

		return errors.New(errMsg)
	}
	return nil
}

// parseS3PullParameters returns the parameters from the s3.pull command that
// references the push task.
func parseS3PullParameters(c model.PluginCommandConf) (task, bv string, err error) {
	if len(c.Params) == 0 {
		return "", "", errors.Errorf("command '%s' has no parameters", c.Command)
	}
	var i interface{}
	var ok bool
	var paramName string

	paramName = "task"
	i, ok = c.Params[paramName]
	if !ok {
		return "", "", errors.Errorf("command '%s' needs parameter '%s' defined", c.Command, paramName)
	} else {
		task, ok = i.(string)
		if !ok {
			return "", "", errors.Errorf("command '%s' was supplied parameter '%s' but is not a string argument, got %T", c.Command, paramName, i)
		}
	}

	paramName = "from_build_variant"
	i, ok = c.Params[paramName]
	if !ok {
		return task, "", nil
	}
	bv, ok = i.(string)
	if !ok {
		return "", "", errors.Errorf("command '%s' was supplied parameter '%s' but is not a string argument, got %T", c.Command, paramName, i)
	}
	return task, bv, nil
}

// checkTasks checks whether project tasks contain warnings by checking if each task
// has commands, contains exec_timeout_sec, and has valid logger configs, dependencies and task names.
func checkTasks(project *model.Project) ValidationErrors {
	errs := ValidationErrors{}
	execTimeoutWarningAdded := false
	allTasks := project.FindAllTasksMap()
	for _, task := range project.Tasks {
		if len(task.Commands) == 0 {
			errs = append(errs,
				ValidationError{
					Message: fmt.Sprintf("task '%s' does not contain any commands",
						task.Name),
					Level: Warning,
				},
			)
		}
		if project.ExecTimeoutSecs == 0 && task.ExecTimeoutSecs == 0 && !execTimeoutWarningAdded {
			errs = append(errs,
				ValidationError{
					Message: fmt.Sprintf("no exec_timeout_secs defined at the top-level or on one or more tasks; "+
						"these tasks will default to a timeout of %d hours",
						int(agent.DefaultExecTimeout.Hours())),
					Level: Warning,
				},
			)
			execTimeoutWarningAdded = true
		}
		errs = append(errs, checkLoggerConfig(&task)...)
		errs = append(errs, checkTaskDependencies(&task, allTasks)...)
		errs = append(errs, checkTaskNames(project, &task)...)
	}
	if project.Loggers != nil {
		if err := project.Loggers.IsValid(); err != nil {
			errs = append(errs, ValidationError{
				Message: errors.Wrap(err, "error in project-level logger config").Error(),
				Level:   Warning,
			})
		}
	}
	return errs
}

// checkBuildVariants checks whether project build variants contain warnings by checking if each variant
// has tasks, valid and non-duplicate names, and appropriate batch time settings.
func checkBuildVariants(project *model.Project) ValidationErrors {
	errs := ValidationErrors{}
	displayNames := map[string]int{}
	for _, buildVariant := range project.BuildVariants {
		dispName := buildVariant.DisplayName
		displayNames[dispName] = displayNames[dispName] + 1

		if len(buildVariant.Tasks) == 0 {
			errs = append(errs,
				ValidationError{
					Message: fmt.Sprintf("buildvariant '%s' contains no tasks", buildVariant.Name),
					Level:   Warning,
				},
			)
		}
		errs = append(errs, checkBVNames(&buildVariant)...)
		errs = append(errs, checkBVBatchTimes(&buildVariant)...)
	}

	for k, v := range displayNames {
		if v > 1 {
			errs = append(errs,
				ValidationError{
					Level:   Warning,
					Message: fmt.Sprintf("%d build variants share the same display name: '%s'", v, k),
				},
			)

		}
	}
	return errs
}
