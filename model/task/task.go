package task

import (
	"fmt"
	"time"

	"github.com/evergreen-ci/evergreen"
	"github.com/evergreen-ci/evergreen/apimodels"
	"github.com/evergreen-ci/evergreen/db"
	"github.com/evergreen-ci/evergreen/model/event"
	"github.com/evergreen-ci/evergreen/util"
	"gopkg.in/mgo.v2/bson"
)

var (
	AgentHeartbeat = "heartbeat"
)

type Task struct {
	Id     string `bson:"_id" json:"id"`
	Secret string `bson:"secret" json:"secret"`

	// time information for task
	// create - the time we created this task in our database
	// dispatch - the time the task runner starts up the agent on the host
	// push - the time the commit generating this build was pushed to the remote
	// scheduled - the time the commit is scheduled
	// start - the time the agent starts the task on the host after spinning it up
	// finish - the time the task was completed on the remote host
	CreateTime    time.Time `bson:"create_time" json:"create_time"`
	DispatchTime  time.Time `bson:"dispatch_time" json:"dispatch_time"`
	PushTime      time.Time `bson:"push_time" json:"push_time"`
	ScheduledTime time.Time `bson:"scheduled_time" json:"scheduled_time"`
	StartTime     time.Time `bson:"start_time" json:"start_time"`
	FinishTime    time.Time `bson:"finish_time" json:"finish_time"`

	Version  string `bson:"version" json:"version,omitempty"`
	Project  string `bson:"branch" json:"branch,omitempty"`
	Revision string `bson:"gitspec" json:"gitspec"`
	Priority int64  `bson:"priority" json:"priority"`

	// only relevant if the task is running.  the time of the last heartbeat
	// sent back by the agent
	LastHeartbeat time.Time `bson:"last_heartbeat"`

	// used to indicate whether task should be scheduled to run
	Activated     bool         `bson:"activated" json:"activated"`
	ActivatedBy   string       `bson:"activated_by" json:"activated_by"`
	BuildId       string       `bson:"build_id" json:"build_id"`
	DistroId      string       `bson:"distro" json:"distro"`
	BuildVariant  string       `bson:"build_variant" json:"build_variant"`
	DependsOn     []Dependency `bson:"depends_on" json:"depends_on"`
	NumDependents int          `bson:"num_dependents,omitempty" json:"num_dependents,omitempty"`

	// Human-readable name
	DisplayName string `bson:"display_name" json:"display_name"`

	// Tags that describe the task
	Tags []string `bson:"tags,omitempty" json:"tags,omitempty"`

	// The host the task was run on
	HostId string `bson:"host_id" json:"host_id"`

	// the number of times this task has been restarted
	Restarts            int    `bson:"restarts" json:"restarts",omitempty`
	Execution           int    `bson:"execution" json:"execution"`
	OldTaskId           string `bson:"old_task_id,omitempty" json:"old_task_id",omitempty`
	Archived            bool   `bson:"archived,omitempty" json:"archived",omitempty`
	RevisionOrderNumber int    `bson:"order,omitempty" json:"order,omitempty"`

	// task requester - this is used to help tell the
	// reason this task was created. e.g. it could be
	// because the repotracker requested it (via tracking the
	// repository) or it was triggered by a developer
	// patch request
	Requester string `bson:"r" json:"r"`

	// Status represents the various stages the task could be in
	Status  string                  `bson:"status" json:"status"`
	Details apimodels.TaskEndDetail `bson:"details" json:"task_end_details"`
	Aborted bool                    `bson:"abort,omitempty" json:"abort"`

	// TimeTaken is how long the task took to execute.  meaningless if the task is not finished
	TimeTaken time.Duration `bson:"time_taken" json:"time_taken"`

	// how long we expect the task to take from start to finish
	ExpectedDuration time.Duration `bson:"expected_duration,omitempty" json:"expected_duration,omitempty"`

	// test results captured and sent back by agent
	TestResults []TestResult `bson:"test_results" json:"test_results"`

	// position in queue for the queue where it's closest to the top
	MinQueuePos int `bson:"min_queue_pos" json:"min_queue_pos,omitempty"`
}

// Dependency represents a task that must be completed before the owning
// task can be scheduled.
type Dependency struct {
	TaskId string `bson:"_id" json:"id"`
	Status string `bson:"status" json:"status"`
}

// SetBSON allows us to use dependency representation of both
// just task Ids and of true Dependency structs.
//  TODO eventually drop all of this switching
func (d *Dependency) SetBSON(raw bson.Raw) error {
	// copy the Dependency type to remove this SetBSON method but preserve bson struct tags
	type nakedDep Dependency
	var depCopy nakedDep
	if err := raw.Unmarshal(&depCopy); err == nil {
		if depCopy.TaskId != "" {
			*d = Dependency(depCopy)
			return nil
		}
	}

	// hack to support the legacy depends_on, since we can't just unmarshal a string
	strBytes, _ := bson.Marshal(bson.RawD{{"str", raw}})
	var strStruct struct {
		String string `bson:"str"`
	}
	if err := bson.Unmarshal(strBytes, &strStruct); err == nil {
		if strStruct.String != "" {
			d.TaskId = strStruct.String
			d.Status = evergreen.TaskSucceeded
			return nil
		}
	}

	return bson.SetZero
}

// TestResults is only used when transferring data from agent to api.
type TestResults struct {
	Results []TestResult `json:"results"`
}

type TestResult struct {
	Status    string  `json:"status" bson:"status"`
	TestFile  string  `json:"test_file" bson:"test_file"`
	URL       string  `json:"url" bson:"url,omitempty"`
	URLRaw    string  `json:"url_raw" bson:"url_raw,omitempty"`
	LogId     string  `json:"log_id,omitempty" bson:"log_id,omitempty"`
	LineNum   int     `json:"line_num,omitempty" bson:"line_num,omitempty"`
	ExitCode  int     `json:"exit_code" bson:"exit_code"`
	StartTime float64 `json:"start" bson:"start"`
	EndTime   float64 `json:"end" bson:"end"`

	// LogRaw is not saved in the task
	LogRaw string `json:"log_raw" bson:"log_raw,omitempty"`
}

var (
	AllStatuses = "*"
)

// Abortable returns true if the task can be aborted.
func IsAbortable(t Task) bool {
	return t.Status == evergreen.TaskStarted ||
		t.Status == evergreen.TaskDispatched
}

// IsFinished returns true if the project is no longer running
func IsFinished(t Task) bool {
	return t.Status == evergreen.TaskFailed ||
		t.Status == evergreen.TaskSucceeded ||
		(t.Status == evergreen.TaskUndispatched && t.DispatchTime != util.ZeroTime)
}

// satisfiesDependency checks a task the receiver task depends on
// to see if its status satisfies a dependency. If the "Status" field is
// unset, default to checking that is succeeded.
func (t *Task) satisfiesDependency(depTask *Task) bool {
	for _, dep := range t.DependsOn {
		if dep.TaskId == depTask.Id {
			switch dep.Status {
			case evergreen.TaskSucceeded, "":
				return depTask.Status == evergreen.TaskSucceeded
			case evergreen.TaskFailed:
				return depTask.Status == evergreen.TaskFailed
			case AllStatuses:
				return depTask.Status == evergreen.TaskFailed || depTask.Status == evergreen.TaskSucceeded
			}
		}
	}
	return false
}

// Checks whether the dependencies for the task have all completed successfully.
// If any of the dependencies exist in the map that is passed in, they are
// used to check rather than fetching from the database. All queries
// are cached back into the map for later use.
func (t *Task) DependenciesMet(depCaches map[string]Task) (bool, error) {

	if len(t.DependsOn) == 0 {
		return true, nil
	}

	deps := make([]Task, 0, len(t.DependsOn))

	depIdsToQueryFor := make([]string, 0, len(t.DependsOn))
	for _, dep := range t.DependsOn {
		if cachedDep, ok := depCaches[dep.TaskId]; !ok {
			depIdsToQueryFor = append(depIdsToQueryFor, dep.TaskId)
		} else {
			deps = append(deps, cachedDep)
		}
	}

	if len(depIdsToQueryFor) > 0 {
		newDeps, err := Find(ByIds(depIdsToQueryFor).WithFields(StatusKey))
		if err != nil {
			return false, err
		}

		// add queried dependencies to the cache
		for _, newDep := range newDeps {
			deps = append(deps, newDep)
			depCaches[newDep.Id] = newDep
		}
	}

	for _, depTask := range deps {
		if !t.satisfiesDependency(&depTask) {
			return false, nil
		}
	}

	return true, nil
}

// FindTaskOnBaseCommit returns the task that is on the base commit.
func (t *Task) FindTaskOnBaseCommit() (*Task, error) {
	return FindOne(ByCommit(t.Revision, t.BuildVariant, t.DisplayName, t.Project, evergreen.RepotrackerVersionRequester))
}

// FindIntermediateTasks returns the tasks from most recent to least recent between two tasks.
func (current *Task) FindIntermediateTasks(previous *Task) ([]Task, error) {
	intermediateTasks, err := Find(ByIntermediateRevisions(previous.RevisionOrderNumber, current.RevisionOrderNumber, current.BuildVariant,
		current.DisplayName, current.Project, current.Requester))
	if err != nil {
		return nil, err
	}

	// reverse the slice of tasks
	intermediateTasksReversed := make([]Task, len(intermediateTasks))
	for idx, t := range intermediateTasks {
		intermediateTasksReversed[len(intermediateTasks)-idx-1] = t
	}
	return intermediateTasksReversed, nil
}

// CountSimilarFailingTasks returns a count of all tasks with the same project,
// same display name, and in other buildvariants, that have failed in the same
// revision
func (t *Task) CountSimilarFailingTasks() (int, error) {
	return Count(ByDifferentFailedBuildVariants(t.Revision, t.BuildVariant, t.DisplayName,
		t.Project, t.Requester))
}

// Find the previously completed task for the same requester + project +
// build variant + display name combination as the specified task
func (t *Task) PreviousCompletedTask(project string,
	statuses []string) (*Task, error) {
	if len(statuses) == 0 {
		statuses = CompletedStatuses
	}
	return FindOne(ByBeforeRevisionWithStatuses(t.RevisionOrderNumber, statuses, t.BuildVariant,
		t.DisplayName, project))
}

// SetExpectedDuration updates the expected duration field for the task
func (t *Task) SetExpectedDuration(duration time.Duration) error {
	return UpdateOne(
		bson.M{
			IdKey: t.Id,
		},
		bson.M{
			"$set": bson.M{
				ExpectedDurationKey: duration,
			},
		},
	)
}

// Mark that the task has been dispatched onto a particular host. Sets the
// running task field on the host and the host id field on the task.
// Returns an error if any of the database updates fail.
func (t *Task) MarkAsDispatched(hostId string, distroId string, dispatchTime time.Time) error {
	t.DispatchTime = dispatchTime
	t.Status = evergreen.TaskDispatched
	t.HostId = hostId
	t.LastHeartbeat = dispatchTime
	t.DistroId = distroId
	return UpdateOne(
		bson.M{
			IdKey: t.Id,
		},
		bson.M{
			"$set": bson.M{
				DispatchTimeKey:  dispatchTime,
				StatusKey:        evergreen.TaskDispatched,
				HostIdKey:        hostId,
				LastHeartbeatKey: dispatchTime,
				DistroIdKey:      distroId,
			},
			"$unset": bson.M{
				AbortedKey:     "",
				TestResultsKey: "",
				DetailsKey:     "",
				MinQueuePosKey: "",
			},
		},
	)

}

// MarkAsUndispatched marks that the task has been undispatched from a
// particular host. Unsets the running task field on the host and the
// host id field on the task
// Returns an error if any of the database updates fail.
func (t *Task) MarkAsUndispatched() error {
	// then, update the task document
	t.Status = evergreen.TaskUndispatched

	return UpdateOne(
		bson.M{
			IdKey: t.Id,
		},
		bson.M{
			"$set": bson.M{
				StatusKey: evergreen.TaskUndispatched,
			},
			"$unset": bson.M{
				DispatchTimeKey:  util.ZeroTime,
				LastHeartbeatKey: util.ZeroTime,
				DistroIdKey:      "",
				HostIdKey:        "",
				AbortedKey:       "",
				TestResultsKey:   "",
				DetailsKey:       "",
				MinQueuePosKey:   "",
			},
		},
	)
}

// UpdateMinQueuePos takes a taskId-to-MinQueuePosition map and updates
// the min_queue_pos field in each specified task to the specified MinQueuePos
func UpdateMinQueuePos(taskIdToMinQueuePos map[string]int) error {
	for taskId, minQueuePos := range taskIdToMinQueuePos {
		err := UpdateOne(
			bson.M{
				IdKey: taskId,
			},
			bson.M{
				"$set": bson.M{
					MinQueuePosKey: minQueuePos,
				},
			},
		)
		if err != nil {
			return err
		}
	}
	return nil
}

// SetTasksScheduledTime takes a list of tasks and a time, and then sets
// the scheduled time in the database for the tasks if it is currently unset
func SetTasksScheduledTime(tasks []Task, scheduledTime time.Time) error {
	var ids []string
	for i := range tasks {
		tasks[i].ScheduledTime = scheduledTime
		ids = append(ids, tasks[i].Id)
	}
	info, err := UpdateAll(
		bson.M{
			IdKey: bson.M{
				"$in": ids,
			},
			ScheduledTimeKey: bson.M{
				"$lte": util.ZeroTime,
			},
		},
		bson.M{
			"$set": bson.M{
				ScheduledTimeKey: scheduledTime,
			},
		},
	)
	if err != nil {
		return err
	}

	if info.Updated > 0 {
		for _, t := range tasks {
			event.LogTaskScheduled(t.Id, scheduledTime)
		}
	}
	return nil

}

// SetAborted sets the abort field of task to aborted
func (t *Task) SetAborted() error {
	t.Aborted = true
	return UpdateOne(
		bson.M{
			IdKey: t.Id,
		},
		bson.M{
			"$set": bson.M{
				AbortedKey: true,
			},
		},
	)
}

// ActivateTask will set the ActivatedBy field to the caller and set the active state to be true
func (t *Task) ActivateTask(caller string) error {
	t.ActivatedBy = caller
	t.Activated = true
	return UpdateOne(bson.M{
		IdKey: t.Id,
	},
		bson.M{
			"$set": bson.M{
				ActivatedKey:   true,
				ActivatedByKey: caller,
			},
			"$unset": bson.M{
				MinQueuePosKey: "",
			},
		})
}

// DeactivateTask will set the ActivatedBy field to the caller and set the active state to be false and deschedule the task
func (t *Task) DeactivateTask(caller string) error {
	t.ActivatedBy = caller
	t.Activated = false
	t.ScheduledTime = util.ZeroTime
	return UpdateOne(
		bson.M{
			IdKey: t.Id,
		},
		bson.M{
			"$set": bson.M{
				ActivatedKey:     false,
				ScheduledTimeKey: util.ZeroTime,
			},
			"$unset": bson.M{
				MinQueuePosKey: "",
			},
		},
	)
}

// MarkEnd handles the Task updates associated with ending a task.
func (t *Task) MarkEnd(caller string, finishTime time.Time, detail *apimodels.TaskEndDetail) error {
	// record that the task has finished, in memory and in the db
	t.Status = detail.Status
	t.FinishTime = finishTime
	t.TimeTaken = finishTime.Sub(t.StartTime)
	t.Details = *detail
	return UpdateOne(
		bson.M{
			IdKey: t.Id,
		},
		bson.M{
			"$set": bson.M{
				FinishTimeKey: finishTime,
				StatusKey:     detail.Status,
				TimeTakenKey:  t.TimeTaken,
				DetailsKey:    t.Details,
			},
			"$unset": bson.M{
				AbortedKey: "",
			},
		})

}

// Reset sets the task state to be activated, with a new secret,
// undispatched status and zero time on Start, Scheduled, Dispatch and FinishTime
func (t *Task) Reset() error {
	t.Activated = true
	t.Secret = util.RandomString()
	t.DispatchTime = util.ZeroTime
	t.StartTime = util.ZeroTime
	t.ScheduledTime = util.ZeroTime
	t.FinishTime = util.ZeroTime
	t.TestResults = []TestResult{}
	reset := bson.M{
		"$set": bson.M{
			ActivatedKey:     true,
			SecretKey:        t.Secret,
			StatusKey:        evergreen.TaskUndispatched,
			DispatchTimeKey:  util.ZeroTime,
			StartTimeKey:     util.ZeroTime,
			ScheduledTimeKey: util.ZeroTime,
			FinishTimeKey:    util.ZeroTime,
			TestResultsKey:   []TestResult{},
		},
		"$unset": bson.M{
			DetailsKey: "",
		},
	}

	return UpdateOne(
		bson.M{
			IdKey: t.Id,
		},
		reset,
	)
}

// Reset sets the task state to be activated, with a new secret,
// undispatched status and zero time on Start, Scheduled, Dispatch and FinishTime
func ResetTasks(taskIds []string) error {
	reset := bson.M{
		"$set": bson.M{
			ActivatedKey:     true,
			SecretKey:        util.RandomString(),
			StatusKey:        evergreen.TaskUndispatched,
			DispatchTimeKey:  util.ZeroTime,
			StartTimeKey:     util.ZeroTime,
			ScheduledTimeKey: util.ZeroTime,
			FinishTimeKey:    util.ZeroTime,
			TestResultsKey:   []TestResult{},
		},
		"$unset": bson.M{
			DetailsKey: "",
		},
	}

	_, err := UpdateAll(
		bson.M{
			IdKey: bson.M{"$in": taskIds},
		},
		reset,
	)
	return err

}

// UpdateHeartbeat updates the heartbeat to be the current time
func (t *Task) UpdateHeartbeat() error {
	t.LastHeartbeat = time.Now()
	return UpdateOne(
		bson.M{
			IdKey: t.Id,
		},
		bson.M{
			"$set": bson.M{
				LastHeartbeatKey: t.LastHeartbeat,
			},
		},
	)
}

// SetPriority sets the priority of the tasks and the tasks that they depend on
func (t *Task) SetPriority(priority int64) error {
	t.Priority = priority
	modifier := bson.M{PriorityKey: priority}

	//blacklisted - this task should never run, so unschedule it now
	if priority < 0 {
		modifier[ActivatedKey] = false
	}

	ids, err := t.getRecursiveDependencies()
	if err != nil {
		return fmt.Errorf("error getting task dependencies: %v", err)
	}

	_, err = UpdateAll(
		bson.M{"$or": []bson.M{
			{IdKey: t.Id},
			{IdKey: bson.M{"$in": ids},
				PriorityKey: bson.M{"$lt": priority}},
		}},
		bson.M{"$set": modifier},
	)
	return err
}

// getRecursiveDependencies creates a slice containing t.Id and the Ids of all recursive dependencies.
// We assume there are no dependency cycles.
func (t *Task) getRecursiveDependencies() ([]string, error) {
	recurIds := make([]string, 0, len(t.DependsOn))
	for _, dependency := range t.DependsOn {
		recurIds = append(recurIds, dependency.TaskId)
	}

	recurTasks, err := Find(ByIds(recurIds))
	if err != nil {
		return nil, err
	}

	ids := make([]string, 0)
	for _, recurTask := range recurTasks {
		appendIds, err := recurTask.getRecursiveDependencies()
		if err != nil {
			return nil, err
		}
		ids = append(ids, appendIds...)
	}

	ids = append(ids, t.Id)
	return ids, nil
}

// MarkStart updates the task's start time and sets the status to started
func (t *Task) MarkStart(startTime time.Time) error {
	// record the start time in the in-memory task
	t.StartTime = startTime
	t.Status = evergreen.TaskStarted
	return UpdateOne(
		bson.M{
			IdKey: t.Id,
		},
		bson.M{
			"$set": bson.M{
				StatusKey:    evergreen.TaskStarted,
				StartTimeKey: startTime,
			},
		},
	)
}

// SetResults sets the results of the task in TestResults
func (t *Task) SetResults(results []TestResult) error {
	t.TestResults = results
	return UpdateOne(
		bson.M{
			IdKey: t.Id,
		},
		bson.M{
			"$set": bson.M{
				TestResultsKey: results,
			},
		},
	)
}

// MarkUnscheduled marks the task as undispatched and updates it in the database
func (t *Task) MarkUnscheduled() error {
	t.Status = evergreen.TaskUndispatched
	return UpdateOne(
		bson.M{
			IdKey: t.Id,
		},
		bson.M{
			"$set": bson.M{
				StatusKey: evergreen.TaskUndispatched,
			},
		},
	)

}

// ClearResults sets the TestResults to an empty list
func (t *Task) ClearResults() error {
	t.TestResults = []TestResult{}
	return UpdateOne(
		bson.M{
			IdKey: t.Id,
		},
		bson.M{
			"$set": bson.M{
				TestResultsKey: []TestResult{},
			},
		},
	)
}

// AbortBuild sets the abort flag on all tasks associated with the build which are in an abortable
// state
func AbortBuild(buildId string) error {
	_, err := UpdateAll(
		bson.M{
			BuildIdKey: buildId,
			StatusKey:  bson.M{"$in": evergreen.AbortableStatuses},
		},
		bson.M{"$set": bson.M{AbortedKey: true}},
	)
	return err
}

//String represents the stringified version of a task
func (t *Task) String() (taskStruct string) {
	taskStruct += fmt.Sprintf("Id: %v\n", t.Id)
	taskStruct += fmt.Sprintf("Status: %v\n", t.Status)
	taskStruct += fmt.Sprintf("Host: %v\n", t.HostId)
	taskStruct += fmt.Sprintf("ScheduledTime: %v\n", t.ScheduledTime)
	taskStruct += fmt.Sprintf("DispatchTime: %v\n", t.DispatchTime)
	taskStruct += fmt.Sprintf("StartTime: %v\n", t.StartTime)
	taskStruct += fmt.Sprintf("FinishTime: %v\n", t.FinishTime)
	taskStruct += fmt.Sprintf("TimeTaken: %v\n", t.TimeTaken)
	taskStruct += fmt.Sprintf("Activated: %v\n", t.Activated)
	taskStruct += fmt.Sprintf("Requester: %v\n", t.FinishTime)
	return
}

// Insert writes the b to the db.
func (t *Task) Insert() error {
	return db.Insert(Collection, t)
}

// Inserts the task into the old_tasks collection
func (t *Task) Archive() error {
	var update bson.M

	// only increment restarts if have a current restarts
	// this way restarts will never be set for new tasks but will be
	// maintained for old ones
	if t.Restarts > 0 {
		update = bson.M{"$inc": bson.M{
			ExecutionKey: 1,
			RestartsKey:  1,
		}}
	} else {
		update = bson.M{
			"$inc": bson.M{ExecutionKey: 1},
		}
	}
	err := UpdateOne(
		bson.M{IdKey: t.Id},
		update)
	if err != nil {
		return fmt.Errorf("task.Archive() failed: %v", err)
	}
	archiveTask := *t
	archiveTask.Id = fmt.Sprintf("%v_%v", t.Id, t.Execution)
	archiveTask.OldTaskId = t.Id
	archiveTask.Archived = true
	err = db.Insert(OldCollection, &archiveTask)
	if err != nil {
		return fmt.Errorf("task.Archive() failed: %v", err)
	}
	return nil
}

// GetTestUrl returns the correct relative URL to a test log, given a
// TestResult structure
func GetTestUrl(tr *TestResult) string {
	// Return url if it exists. If there is no test, return empty string.
	if tr.URL != "" || tr.LogId == "" { // If LogId is empty, URL must also be empty
		return tr.URL
	}
	return TestLogPath + tr.LogId
}

// Aggregation

// AverageTaskTimeDifference takes two field names (such that field2 happened
// after field1), a field to group on, and a cutoff time.
// It returns the average duration between fields 1 and 2, grouped by
// the groupBy field, including only task documents where both time
// fields happened after the given cutoff time. This information is returned
// as a map from groupBy_field -> avg_time_difference
//
// NOTE: THIS FUNCTION DOES NOT SANITIZE INPUT!
// BAD THINGS CAN HAPPEN IF NON-TIME FIELDNAMES ARE PASSED IN
// OR IF A FIELD OF NON-STRING TYPE IS SUPPLIED FOR groupBy!
func AverageTaskTimeDifference(field1 string, field2 string,
	groupByField string, cutoff time.Time) (map[string]time.Duration, error) {

	// This pipeline returns the average time difference between
	// two time fields, grouped by a given field of "string" type.
	// It assumes field2 happened later than field1.
	// Time difference returned in milliseconds.
	pipeline := []bson.M{
		{"$match": bson.M{
			field1: bson.M{"$gt": cutoff},
			field2: bson.M{"$gt": cutoff}}},
		{"$group": bson.M{
			"_id": "$" + groupByField,
			"avg_time": bson.M{
				"$avg": bson.M{
					"$subtract": []string{"$" + field2, "$" + field1},
				},
			},
		}},
	}

	// anonymous struct for unmarshalling result bson
	// NOTE: This means we can only group by string fields currently
	var results []struct {
		GroupId     string `bson:"_id"`
		AverageTime int64  `bson:"avg_time"`
	}

	err := db.Aggregate(Collection, pipeline, &results)
	if err != nil {
		return nil, fmt.Errorf("Error aggregating task times by [%v, %v]: %v", field1, field2, err)
	}

	avgTimes := make(map[string]time.Duration)
	for _, res := range results {
		avgTimes[res.GroupId] = time.Duration(res.AverageTime) * time.Millisecond
	}

	return avgTimes, nil
}

// ExpectedTaskDuration takes a given project and buildvariant and computes
// the average duration - grouped by task display name - for tasks that have
// completed within a given threshold as determined by the window
func ExpectedTaskDuration(project, buildvariant string, window time.Duration) (map[string]time.Duration, error) {
	pipeline := []bson.M{
		{
			"$match": bson.M{
				BuildVariantKey: buildvariant,
				ProjectKey:      project,
				StatusKey: bson.M{
					"$in": []string{evergreen.TaskSucceeded, evergreen.TaskFailed},
				},
				DetailsKey + "." + TaskEndDetailTimedOut: bson.M{
					"$ne": true,
				},
				FinishTimeKey: bson.M{
					"$gte": time.Now().Add(-window),
				},
				StartTimeKey: bson.M{
					// make sure all documents have a valid start time so we don't
					// return tasks with runtimes of multiple years
					"$gt": util.ZeroTime,
				},
			},
		},
		{
			"$project": bson.M{
				DisplayNameKey: 1,
				TimeTakenKey:   1,
				IdKey:          0,
			},
		},
		{
			"$group": bson.M{
				"_id": fmt.Sprintf("$%v", DisplayNameKey),
				"exp_dur": bson.M{
					"$avg": fmt.Sprintf("$%v", TimeTakenKey),
				},
			},
		},
	}

	// anonymous struct for unmarshalling result bson
	var results []struct {
		DisplayName      string `bson:"_id"`
		ExpectedDuration int64  `bson:"exp_dur"`
	}

	err := db.Aggregate(Collection, pipeline, &results)
	if err != nil {
		return nil, fmt.Errorf("error aggregating task average duration: %v", err)
	}

	expDurations := make(map[string]time.Duration)
	for _, result := range results {
		expDuration := time.Duration(result.ExpectedDuration) * time.Nanosecond
		expDurations[result.DisplayName] = expDuration
	}

	return expDurations, nil
}
