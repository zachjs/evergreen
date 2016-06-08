package model

import (
	"fmt"

	"gopkg.in/yaml.v2"
)

type parseProject struct {
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
	BuildVariants   []parseBV                  `yaml:"buildvariants"`
	Functions       map[string]*YAMLCommandSet `yaml:"functions"`
	Tasks           []parseTask                `yaml:"tasks"`
	ExecTimeoutSecs int                        `yaml:"exec_timeout_secs"`
}

// Unmarshalled from the "tasks" list in the project file
type parseTask struct {
	Name            string              `yaml:"name"`
	Priority        int64               `yaml:"priority"`
	ExecTimeoutSecs int                 `yaml:"exec_timeout_secs"`
	DisableCleanup  bool                `yaml:"disable_cleanup"`
	DependsOn       parseDependencies   `yaml:"depends_on"`
	Requires        TaskSelectors       `yaml:"requires"`
	Commands        []PluginCommandConf `yaml:"commands"`
	Tags            []string            `yaml:"tags"`

	// Use a *bool so that there are 3 possible states:
	//   1. nil   = not overriding the project setting (default)
	//   2. true  = overriding the project setting with true
	//   3. false = overriding the project setting with false
	Patchable *bool `yaml:"patchable"`
	Stepback  *bool `yaml:"stepback"`
}

type parseDependency struct {
	TaskSelector
	Status        string `yaml:"status"`
	PatchOptional bool   `yaml:"patch_optional"`
}

type parseDependencies []parseDependency

func (pds *parseDependencies) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// first check if we are only doing one dependency
	pd := parseDependency{}
	if err := unmarshal(&pd); err == nil {
		*pds = parseDependencies([]parseDependency{pd})
		return nil
	}
	var pdsCopy []parseDependency
	if err := unmarshal(&pdsCopy); err != nil {
		return err
	}
	*pds = parseDependencies(pdsCopy)
	return nil
}

func (pd *parseDependency) UnmarshalYAML(unmarshal func(interface{}) error) error {
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

type parseBV struct {
	Name        string            `yaml:"name"`
	DisplayName string            `yaml:"display_name"`
	Expansions  map[string]string `yaml:"expansions"`
	Modules     []string          `yaml:"modules"`
	Disabled    bool              `yaml:"disabled"`
	Push        bool              `yaml:"push"`
	BatchTime   *int              `yaml:"batchtime"`
	Stepback    *bool             `yaml:"stepback"`
	RunOn       []string          `yaml:"run_on"` //TODO make this a StringSlice
	Tasks       []parseBVTask     `yaml:"tasks"`
}

type parseBVTask struct {
	Name            string            `yaml:"name"`
	Patchable       *bool             `yaml:"patchable"`
	Priority        int64             `yaml:"priority"`
	DependsOn       parseDependencies `yaml:"depends_on"`
	Requires        TaskSelectors     `yaml:"requires"`
	ExecTimeoutSecs int               `yaml:"exec_timeout_secs"`
	Stepback        *bool             `yaml:"stepback"`
	Distros         []string          `yaml:"distros"` //TODO accept "run_on" here
}

func (pbvt *parseBVTask) UnmarshalYAML(unmarshal func(interface{}) error) error {
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
	type copyType parseBVTask
	var cpy copyType
	if err := unmarshal(&cpy); err != nil {
		return err
	}
	if cpy.Name == "" {
		return fmt.Errorf("task selector must have a name")
	}
	*pbvt = parseBVTask(cpy)
	return nil
}

// // // //
// // // //
// // // //
// // // //

// LoadProjectInto loads the raw data from the config file into project
// and sets the project's identifier field to identifier. Tags are expanded.
func LoadProjectInto(data []byte, identifier string, project *Project) error {
	if err := yaml.Unmarshal(data, project); err != nil {
		return fmt.Errorf("parse error unmarshalling project: %v", err)
	}
	// expand task definitions
	if err := project.EvaluateTags(); err != nil {
		return fmt.Errorf("error evaluating project tags: %v", err)
	}
	project.Identifier = identifier
	return nil
}

type projectParser struct {
	p        *parseProject
	errors   []string
	warnings []string
}

func (pp *projectParser) FromYAML(yml []byte) (*Project, []string, []string) {
	// create intermediate project
	//   handle special fields
	//   handle boring fields
	// expand things
	//   create definitions map and stub matrix variants
	//   expand tasks
	//   create variants

	return nil, nil, nil
}

func (pp *projectParser) appendError(err string) {
	pp.errors = append(pp.errors, err)
}

func (pp *projectParser) createIntermediateProject(yml []byte) bool {
	pp.p = &parseProject{}
	err := yaml.Unmarshal(yml, pp.p)
	if err != nil {
		pp.appendError(err.Error())
		return false
	}
	return true
}
