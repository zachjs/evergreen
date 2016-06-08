package model

import (
	"fmt"

	"gopkg.in/yaml.v2"
)

type parseProject struct {
	Enabled         bool            `yaml:"enabled,omitempty" bson:"enabled"` //TODO clean up tags
	Stepback        bool            `yaml:"stepback,omitempty" bson:"stepback"`
	DisableCleanup  bool            `yaml:"disable_cleanup,omitempty" bson:"disable_cleanup,omitempty"`
	BatchTime       int             `yaml:"batchtime,omitempty" bson:"batch_time"`
	Owner           string          `yaml:"owner,omitempty" bson:"owner_name"`
	Repo            string          `yaml:"repo,omitempty" bson:"repo_name"`
	RemotePath      string          `yaml:"remote_path,omitempty" bson:"remote_path"`
	RepoKind        string          `yaml:"repokind,omitempty" bson:"repo_kind"`
	Branch          string          `yaml:"branch,omitempty" bson:"branch_name"`
	Identifier      string          `yaml:"identifier,omitempty" bson:"identifier"`
	DisplayName     string          `yaml:"display_name,omitempty" bson:"display_name"`
	CommandType     string          `yaml:"command_type,omitempty" bson:"command_type"`
	Ignore          []string        `yaml:"ignore,omitempty" bson:"ignore"`
	Pre             *YAMLCommandSet `yaml:"pre,omitempty" bson:"pre"`
	Post            *YAMLCommandSet `yaml:"post,omitempty" bson:"post"`
	Timeout         *YAMLCommandSet `yaml:"timeout,omitempty" bson:"timeout"`
	CallbackTimeout int             `yaml:"callback_timeout_secs,omitempty" bson:"callback_timeout_secs"`
	Modules         []Module        `yaml:"modules,omitempty" bson:"modules"`
	//BuildVariants   []BuildVariant             `yaml:"buildvariants,omitempty" bson:"build_variants"`
	Functions       map[string]*YAMLCommandSet `yaml:"functions,omitempty" bson:"functions"`
	Tasks           []parseTask                `yaml:"tasks,omitempty" bson:"tasks"`
	ExecTimeoutSecs int                        `yaml:"exec_timeout_secs,omitempty" bson:"exec_timeout_secs"`
}

// Unmarshalled from the "tasks" list in the project file
type parseTask struct {
	Name            string              `yaml:"name,omitempty" bson:"name"`
	Priority        int64               `yaml:"priority,omitempty" bson:"priority"`
	ExecTimeoutSecs int                 `yaml:"exec_timeout_secs,omitempty" bson:"exec_timeout_secs"`
	DisableCleanup  bool                `yaml:"disable_cleanup,omitempty" bson:"disable_cleanup,omitempty"`
	DependsOn       parseDependencies   `yaml:"depends_on,omitempty" bson:"depends_on"`
	Requires        []TaskSelector      `yaml:"requires,omitempty" bson:"requires"`
	Commands        []PluginCommandConf `yaml:"commands,omitempty" bson:"commands"`
	Tags            []string            `yaml:"tags,omitempty" bson:"tags"`

	// Use a *bool so that there are 3 possible states:
	//   1. nil   = not overriding the project setting (default)
	//   2. true  = overriding the project setting with true
	//   3. false = overriding the project setting with false
	Patchable *bool `yaml:"patchable,omitempty" bson:"patchable,omitempty"`
	Stepback  *bool `yaml:"stepback,omitempty" bson:"stepback,omitempty"`
}

type parseDependency struct {
	TaskSelector
	Status        string `yaml:"status,omitempty" bson:"status,omitempty"`
	PatchOptional bool   `yaml:"patch_optional,omitempty" bson:"patch_optional,omitempty"`
}

type parseDependencies []parseDependency

func (pds *parseDependencies) UnmarshalYAML(unmarshal func(interface{}) error) error {
	pd := parseDependency{}
	// first check if we are only doing one dependency
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
		Status        string `yaml:"status,omitempty"`
		PatchOptional bool   `yaml:"patch_optional,omitempty"`
	}{}
	// ignore any errors here; if we're using a single-string selector, this will fail
	unmarshal(&otherFields)
	// TODO validate status
	pd.Status = otherFields.Status
	pd.PatchOptional = otherFields.PatchOptional
	return nil
}

//TODO
type TaskSelector struct {
	Name    string `yaml:"name,omitempty" bson:"name"`
	Variant string `yaml:"variant,omitempty" bson:"variant,omitempty"`
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

// // // //
// // // //
// // // //
// // // //

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
