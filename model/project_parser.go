package model

import (
	"fmt"
	"reflect"
	"strings"

	"gopkg.in/yaml.v2"
)

type parserProject struct {
	Enabled         bool                       `yaml:"enabled"`
	Stepback        bool                       `yaml:"stepback"`
	DisableCleanup  bool                       `yaml:"disable_cleanup"`
	BatchTime       int                        `yaml:"batchtime"`
	Owner           string                     `yaml:"owner"`
	Repo            string                     `yaml:"repo"`
	RemotePath      string                     `yaml:"remote_path"`
	RepoKind        string                     `yaml:"repokind"`
	Branch          string                     `yaml:"branch"`
	Identifier      string                     `yaml:"identifier"`
	DisplayName     string                     `yaml:"display_name"`
	CommandType     string                     `yaml:"command_type"`
	Ignore          []string                   `yaml:"ignore"`
	Pre             *YAMLCommandSet            `yaml:"pre"`
	Post            *YAMLCommandSet            `yaml:"post"`
	Timeout         *YAMLCommandSet            `yaml:"timeout"`
	CallbackTimeout int                        `yaml:"callback_timeout_secs"`
	Modules         []Module                   `yaml:"modules"`
	BuildVariants   []parserBV                 `yaml:"buildvariants"`
	Functions       map[string]*YAMLCommandSet `yaml:"functions"`
	Tasks           []parserTask               `yaml:"tasks"`
	ExecTimeoutSecs int                        `yaml:"exec_timeout_secs"`
}

type parserTask struct {
	Name            string              `yaml:"name"`
	Priority        int64               `yaml:"priority"`
	ExecTimeoutSecs int                 `yaml:"exec_timeout_secs"`
	DisableCleanup  bool                `yaml:"disable_cleanup"`
	DependsOn       parserDependencies  `yaml:"depends_on"`
	Requires        TaskSelectors       `yaml:"requires"`
	Commands        []PluginCommandConf `yaml:"commands"`
	Tags            []string            `yaml:"tags"`
	Stepback        *bool               `yaml:"stepback"`
}

type parserDependency struct {
	TaskSelector
	Status        string `yaml:"status"`
	PatchOptional bool   `yaml:"patch_optional"`
}

type parserDependencies []parserDependency

func (pds *parserDependencies) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// first check if we are only doing one dependency
	pd := parserDependency{}
	if err := unmarshal(&pd); err == nil {
		*pds = parserDependencies([]parserDependency{pd})
		return nil
	}
	var pdsCopy []parserDependency
	if err := unmarshal(&pdsCopy); err != nil {
		return err
	}
	*pds = parserDependencies(pdsCopy)
	return nil
}

func (pd *parserDependency) UnmarshalYAML(unmarshal func(interface{}) error) error {
	if err := unmarshal(&pd.TaskSelector); err != nil {
		return err
	}
	otherFields := struct {
		Status        string `yaml:"status"`
		PatchOptional bool   `yaml:"patch_optional"`
	}{}
	// ignore any errors here; if we're using a single-string selector, this will fail
	unmarshal(&otherFields)
	// TODO validate status
	pd.Status = otherFields.Status
	pd.PatchOptional = otherFields.PatchOptional
	return nil
}

//TODO consider making this a TVSelector
type TaskSelector struct {
	Name    string `yaml:"name"`
	Variant string `yaml:"variant"`
}

type TaskSelectors []TaskSelector

func (tss *TaskSelectors) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// first, attempt to unmarshal just a selector string
	var single TaskSelector
	if err := unmarshal(&single); err == nil {
		*tss = TaskSelectors([]TaskSelector{single})
		return nil
	}
	var slice []TaskSelector
	if err := unmarshal(&slice); err != nil {
		return err
	}
	*tss = TaskSelectors(slice)
	return nil
}

// UnmarshalYAML allows tasks to be referenced as single selector strings.
// This works by first attempting to unmarshal the YAML into a string
// and then falling back to the TaskDependency struct.
func (ts *TaskSelector) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// first, attempt to unmarshal just a selector string
	var onlySelector string
	if err := unmarshal(&onlySelector); err == nil {
		if onlySelector != "" {
			ts.Name = onlySelector
			return nil
		}
	}
	// we define a new type so that we can grab the yaml struct tags without the struct methods,
	// preventing infinte recursion on the UnmarshalYAML() method.
	type copyType TaskSelector
	var tsc copyType
	if err := unmarshal(&tsc); err != nil {
		return err
	}
	if tsc.Name == "" {
		return fmt.Errorf("task selector must have a name")
	}
	*ts = TaskSelector(tsc)
	return nil
}

type parserBV struct {
	Name        string            `yaml:"name"`
	DisplayName string            `yaml:"display_name"`
	Expansions  map[string]string `yaml:"expansions"`
	Modules     []string          `yaml:"modules"`
	Disabled    bool              `yaml:"disabled"`
	Push        bool              `yaml:"push"`
	BatchTime   *int              `yaml:"batchtime"`
	Stepback    *bool             `yaml:"stepback"`
	RunOn       []string          `yaml:"run_on"` //TODO make this a StringSlice
	Tasks       parserBVTasks     `yaml:"tasks"`
}

type parserBVTask struct {
	Name            string             `yaml:"name"`
	Patchable       *bool              `yaml:"patchable"`
	Priority        int64              `yaml:"priority"`
	DependsOn       parserDependencies `yaml:"depends_on"`
	Requires        TaskSelectors      `yaml:"requires"`
	ExecTimeoutSecs int                `yaml:"exec_timeout_secs"`
	Stepback        *bool              `yaml:"stepback"`
	Distros         []string           `yaml:"distros"` //TODO accept "run_on" here
}

func (pbvt *parserBVTask) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// first, attempt to unmarshal just a selector string
	var onlySelector string
	if err := unmarshal(&onlySelector); err == nil {
		if onlySelector != "" {
			pbvt.Name = onlySelector
			return nil
		}
	}
	// we define a new type so that we can grab the yaml struct tags without the struct methods,
	// preventing infinte recursion on the UnmarshalYAML() method.
	type copyType parserBVTask
	var cpy copyType
	if err := unmarshal(&cpy); err != nil {
		return err
	}
	if cpy.Name == "" {
		return fmt.Errorf("task selector must have a name")
	}
	*pbvt = parserBVTask(cpy)
	return nil
}

type parserBVTasks []parserBVTask

func (pbvts *parserBVTasks) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// first, attempt to unmarshal just a selector string
	var single parserBVTask
	if err := unmarshal(&single); err == nil {
		*pbvts = parserBVTasks([]parserBVTask{single})
		return nil
	}
	var slice []parserBVTask
	if err := unmarshal(&slice); err != nil {
		return err
	}
	*pbvts = parserBVTasks(slice)
	return nil
}

// // // //
// // // //
// // // //
// // // //

// LoadProjectInto loads the raw data from the config file into project
// and sets the project's identifier field to identifier. Tags are evaluateed.
func LoadProjectInto(data []byte, identifier string, project *Project) error {
	pp := projectParser{}
	p, _, errs := pp.FromYAML(data)
	if len(errs) > 0 {
		return fmt.Errorf("error loading project yaml: %v", strings.Join(errs, "\n\t"))
	}
	*project = *p
	project.Identifier = identifier
	return nil
}

type projectParser struct {
	p        *parserProject
	taskEval *taskSelectorEvaluator
	errors   []string
	warnings []string
}

func (pp *projectParser) FromYAML(yml []byte) (*Project, []string, []string) {
	if !pp.createIntermediateProject(yml) {
		return nil, pp.warnings, pp.errors
	}
	p := pp.translateProject()
	return p, pp.warnings, pp.errors
}

func (pp *projectParser) appendError(err string) {
	pp.errors = append(pp.errors, err)
}

func (pp *projectParser) hasErrors() bool {
	return len(pp.errors) > 0
}

func (pp *projectParser) createIntermediateProject(yml []byte) bool {
	pp.p = &parserProject{}
	err := yaml.Unmarshal(yml, pp.p)
	if err != nil {
		pp.appendError(err.Error())
		return false
	}
	return true
}

func (pp *projectParser) translateProject() *Project {
	// Transfer top level fields
	proj := &Project{
		Enabled:         pp.p.Enabled,
		Stepback:        pp.p.Stepback,
		DisableCleanup:  pp.p.DisableCleanup,
		BatchTime:       pp.p.BatchTime,
		Owner:           pp.p.Owner,
		Repo:            pp.p.Repo,
		RemotePath:      pp.p.RemotePath,
		RepoKind:        pp.p.RepoKind,
		Branch:          pp.p.Branch,
		Identifier:      pp.p.Identifier,
		DisplayName:     pp.p.DisplayName,
		CommandType:     pp.p.CommandType,
		Ignore:          pp.p.Ignore,
		Pre:             pp.p.Pre,
		Post:            pp.p.Post,
		Timeout:         pp.p.Timeout,
		CallbackTimeout: pp.p.CallbackTimeout,
		Modules:         pp.p.Modules,
		Functions:       pp.p.Functions,
		ExecTimeoutSecs: pp.p.ExecTimeoutSecs,
	}
	pp.taskEval = NewParserTaskSelectorEvaluator(pp.p.Tasks)
	proj.Tasks = pp.evaluateTasks(pp.p.Tasks)
	proj.BuildVariants = pp.evaluateBuildVariants(pp.p.BuildVariants)
	return proj
}

func (pp *projectParser) evaluateTasks(pts []parserTask) []ProjectTask {
	tasks := []ProjectTask{}
	for _, pt := range pts {
		t := ProjectTask{
			Name:            pt.Name,
			Priority:        pt.Priority,
			ExecTimeoutSecs: pt.ExecTimeoutSecs,
			DisableCleanup:  pt.DisableCleanup,
			Commands:        pt.Commands,
			Tags:            pt.Tags,
			Stepback:        pt.Stepback,
		}
		t.DependsOn = pp.evaluateDependsOn(pt.DependsOn)
		t.Requires = pp.evaluateRequires(pt.Requires)
		tasks = append(tasks, t)
	}
	return tasks
}

func (pp *projectParser) evaluateBuildVariants(pbvs []parserBV) []BuildVariant {
	bvs := []BuildVariant{}
	for _, pbv := range pbvs {
		bv := BuildVariant{
			DisplayName: pbv.DisplayName,
			Name:        pbv.Name,
			Expansions:  pbv.Expansions,
			Modules:     pbv.Modules,
			Disabled:    pbv.Disabled,
			Push:        pbv.Push,
			BatchTime:   pbv.BatchTime,
			Stepback:    pbv.Stepback,
			RunOn:       pbv.RunOn,
		}
		bv.Tasks = pp.evaluateBVTasks(pbv.Tasks)
		bvs = append(bvs, bv)
	}
	return bvs
}

func (pp *projectParser) evaluateBVTasks(pbvts []parserBVTask) []BuildVariantTask {
	ts := []BuildVariantTask{}
	tasksByName := map[string]BuildVariantTask{}
	for _, pt := range pbvts {
		names, err := pp.taskEval.evalSelector(ParseSelector(pt.Name))
		if err != nil {
			pp.appendError(err.Error())
			continue
		}
		// create new task definitions--duplicates must have the same status requirements
		for _, name := range names {
			// create a new task by copying the task that selected it,
			// so we can preserve the "Variant" and "Status" field.
			t := BuildVariantTask{
				Name:            name,
				Patchable:       pt.Patchable,
				Priority:        pt.Priority,
				ExecTimeoutSecs: pt.ExecTimeoutSecs,
				Stepback:        pt.Stepback,
				Distros:         pt.Distros,
			}
			t.DependsOn = pp.evaluateDependsOn(pt.DependsOn)
			t.Requires = pp.evaluateRequires(pt.Requires)

			// add the new task if it doesn't already exists (we must avoid conflicting status fields)
			if old, ok := tasksByName[t.Name]; !ok {
				ts = append(ts, t)
				tasksByName[t.Name] = t
			} else {
				// it's already in the new list, so we check to make sure the status definitions match.
				if !reflect.DeepEqual(t, old) {
					pp.appendError(fmt.Sprintf(
						"conflicting definitions of build variant tasks '%v': %v != %v", name, t, old))
					continue
				}
			}
		}
	}
	return ts
}

func (pp *projectParser) evaluateDependsOn(deps []parserDependency) []TaskDependency {
	// This is almost an exact copy of EvaluateTasks.
	newDeps := []TaskDependency{}
	newDepsByNameAndVariant := map[TVPair]TaskDependency{}
	for _, d := range deps {
		if d.Name == AllDependencies {
			// * is a special case for dependencies //TODO--should it be?
			allDep := TaskDependency{
				Name:          AllDependencies,
				Variant:       d.Variant,
				Status:        d.Status,
				PatchOptional: d.PatchOptional,
			}
			newDeps = append(newDeps, allDep)
			newDepsByNameAndVariant[TVPair{d.Variant, d.Name}] = allDep
			continue
		}
		names, err := pp.taskEval.evalSelector(ParseSelector(d.Name))
		if err != nil {
			pp.appendError(err.Error())
			continue
		}
		// create new dependency definitions--duplicates must have the same status requirements
		for _, name := range names {
			// create a newDep by copying the dep that selected it,
			// so we can preserve the "Variant" and "Status" field.
			newDep := TaskDependency{
				Name:          name,
				Variant:       d.Variant,
				Status:        d.Status,
				PatchOptional: d.PatchOptional,
			}
			newDep.Name = name
			// add the new dep if it doesn't already exists (we must avoid conflicting status fields)
			if oldDep, ok := newDepsByNameAndVariant[TVPair{newDep.Variant, newDep.Name}]; !ok {
				newDeps = append(newDeps, newDep)
				newDepsByNameAndVariant[TVPair{newDep.Variant, newDep.Name}] = newDep
			} else {
				// it's already in the new list, so we check to make sure the status definitions match.
				if !reflect.DeepEqual(newDep, oldDep) {
					pp.appendError(fmt.Sprintf(
						"conflicting definitions of dependency '%v': %v != %v", name, newDep, oldDep))
					continue
				}
			}
		}
	}
	return newDeps
}

func (pp *projectParser) evaluateRequires(reqs []TaskSelector) []TaskRequirement {
	newReqs := []TaskRequirement{}
	newReqsByNameAndVariant := map[TVPair]struct{}{}
	for _, r := range reqs {
		names, err := pp.taskEval.evalSelector(ParseSelector(r.Name))
		if err != nil {
			pp.appendError(err.Error())
			continue
		}
		for _, name := range names {
			newReq := TaskRequirement{Name: name, Variant: r.Variant}
			newReq.Name = name
			// add the new req if it doesn't already exists (we must avoid duplicates)
			if _, ok := newReqsByNameAndVariant[TVPair{newReq.Variant, newReq.Name}]; !ok {
				newReqs = append(newReqs, newReq)
				newReqsByNameAndVariant[TVPair{newReq.Variant, newReq.Name}] = struct{}{}
			}
		}
	}
	return newReqs
}
