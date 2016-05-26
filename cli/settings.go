package cli

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/evergreen-ci/evergreen/util"
	"github.com/kardianos/osext"
	"github.com/mitchellh/go-homedir"
	"gopkg.in/yaml.v2"
)

// prompt writes a prompt to the user on stdout, reads a newline-terminated response from stdin,
// and returns the result as a string.
func prompt(message string) string {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print(message + " ")
	text, _ := reader.ReadString('\n')
	return strings.TrimSpace(text)
}

// confirm asks the user a yes/no question and returns true/false if they reply with y/yes/n/no.
// if defaultYes is true, allows user to just hit enter without typing an explicit yes.
func confirm(message string, defaultYes bool) bool {
	reply := ""
	yes := []string{"y", "yes"}
	no := []string{"n", "no"}
	if defaultYes {
		yes = append(yes, "")
	}
	for {
		reply = prompt(message)
		if util.SliceContains(yes, strings.ToLower(reply)) {
			return true
		}
		if util.SliceContains(no, strings.ToLower(reply)) {
			return false
		}
	}
}

// LoadSettings attempts to load the settings file
func LoadSettings(opts *Options) (*Settings, error) {
	confPath := opts.ConfFile
	if confPath == "" {
		userHome, err := homedir.Dir()
		if err != nil {
			// workaround for cygwin if we're on windows but couldn't get a homedir
			if runtime.GOOS == "windows" && len(os.Getenv("HOME")) > 0 {
				userHome = os.Getenv("HOME")
			} else {
				return nil, err
			}
		}
		confPath = filepath.Join(userHome, ".evergreen.yml")
	}
	var f io.ReadCloser
	var err, extErr, extOpenErr error
	f, err = os.Open(confPath)
	if err != nil {
		// if we can't find the yml file in the home directory,
		// try to find it in the same directory as where the binary is being run from.
		// If we fail to determine that location, just return the first (outer) error.
		var currentBinPath string
		currentBinPath, extErr = osext.Executable()
		if extErr != nil {
			return nil, err
		}
		f, extOpenErr = os.Open(filepath.Join(filepath.Dir(currentBinPath), ".evergreen.yml"))
		if extOpenErr != nil {
			return nil, err
		}
	}

	settings := &Settings{}
	err = util.ReadYAMLInto(f, settings)
	if err != nil {
		return nil, err
	}
	return settings, nil
}

type Options struct {
	ConfFile string `short:"c" long:"config" description:"path to config file (defaults to ~/.evergreen.yml)"`
}

type ProjectConf struct {
	Name     string   `yaml:"name,omitempty"`
	Default  bool     `yaml:"default,omitempty"`
	Variants []string `yaml:"variants,omitempty"`
	Tasks    []string `yaml:"tasks,omitempty"`
}

// Settings represents the data stored in the user's config file, by default
// located at ~/.evergreen.yml
type Settings struct {
	APIServerHost string        `yaml:"api_server_host,omitempty"`
	UIServerHost  string        `yaml:"ui_server_host,omitempty"`
	APIKey        string        `yaml:"api_key,omitempty"`
	User          string        `yaml:"user,omitempty"`
	Projects      []ProjectConf `yaml:"projects,omitempty"`
	loadedFrom    string        `yaml:"-"`
}

func (s *Settings) Write(opts *Options) error {
	confPath := opts.ConfFile
	if confPath == "" {
		if s.loadedFrom != "" {
			confPath = s.loadedFrom
		}
	}
	if confPath == "" {
		return fmt.Errorf("can't determine output location for settings file")
	}
	yamlData, err := yaml.Marshal(s)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(confPath, yamlData, 0644)
}

func (s *Settings) FindDefaultProject() string {
	for _, p := range s.Projects {
		if p.Default {
			return p.Name
		}
	}
	return ""
}

func (s *Settings) FindDefaultVariants(project string) []string {
	for _, p := range s.Projects {
		if p.Name == project {
			return p.Variants
		}
	}
	return nil
}

func (s *Settings) SetDefaultVariants(project string, variants ...string) {
	for i, p := range s.Projects {
		if p.Name == project {
			s.Projects[i].Variants = variants
			return
		}
	}

	s.Projects = append(s.Projects, ProjectConf{project, true, variants, nil})
}

func (s *Settings) FindDefaultTasks(project string) []string {
	for _, p := range s.Projects {
		if p.Name == project {
			return p.Tasks
		}
	}
	return nil
}

func (s *Settings) SetDefaultTasks(project string, tasks ...string) {
	for i, p := range s.Projects {
		if p.Name == project {
			s.Projects[i].Tasks = tasks
			return
		}
	}

	s.Projects = append(s.Projects, ProjectConf{project, true, nil, tasks})
}

func (s *Settings) SetDefaultProject(name string) {
	var foundDefault bool
	for i, p := range s.Projects {
		if p.Name == name {
			s.Projects[i].Default = true
			foundDefault = true
		} else {
			s.Projects[i].Default = false
		}
	}

	if !foundDefault {
		s.Projects = append(s.Projects, ProjectConf{name, true, []string{}, []string{}})
	}
}
