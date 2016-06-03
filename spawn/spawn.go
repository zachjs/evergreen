package spawn

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/10gen-labs/slogger/v1"
	"github.com/evergreen-ci/evergreen"
	"github.com/evergreen-ci/evergreen/cloud/providers"
	"github.com/evergreen-ci/evergreen/command"
	"github.com/evergreen-ci/evergreen/hostinit"
	"github.com/evergreen-ci/evergreen/model/distro"
	"github.com/evergreen-ci/evergreen/model/host"
	"github.com/evergreen-ci/evergreen/model/user"
	"github.com/evergreen-ci/evergreen/util"
	"gopkg.in/yaml.v2"
)

const (
	MaxPerUser        = 3
	DefaultExpiration = time.Duration(24 * time.Hour)
)

var SpawnLimitErr = errors.New("User is already running the max allowed # of spawn hosts")

// BadOptionsErr represents an in valid set of spawn options.
type BadOptionsErr struct {
	message string
}

func (bsoe BadOptionsErr) Error() string {
	return "Invalid spawn options:" + bsoe.message
}

// Spawn handles Spawning hosts for users.
type Spawn struct {
	settings *evergreen.Settings
}

// Options holds the required parameters for spawning a host.
type Options struct {
	Distro    string
	UserName  string
	PublicKey string
	UserData  string
	TaskId    string
}

// New returns an initialized Spawn controller.
func New(settings *evergreen.Settings) Spawn {
	return Spawn{settings}
}

// Validate returns an instance of BadOptionsErr if the SpawnOptions object contains invalid
// data, SpawnLimitErr if the user is already at the spawned host limit, or some other untyped
// instance of Error if something fails during validation.
func (sm Spawn) Validate(so Options) error {
	d, err := distro.FindOne(distro.ById(so.Distro))
	if err != nil {
		return BadOptionsErr{fmt.Sprintf("Invalid dist %v", so.Distro)}
	}

	if !d.SpawnAllowed {
		return BadOptionsErr{fmt.Sprintf("Spawning not allowed for dist %v", so.Distro)}
	}

	// if the user already has too many active spawned hosts, deny the request
	activeSpawnedHosts, err := host.Find(host.ByUserWithRunningStatus(so.UserName))
	if err != nil {
		return fmt.Errorf("Error occurred finding user's current hosts: %v", err)
	}

	if len(activeSpawnedHosts) >= MaxPerUser {
		return SpawnLimitErr
	}

	// validate public key
	rsa := "ssh-rsa"
	dss := "ssh-dss"
	isRSA := strings.HasPrefix(so.PublicKey, rsa)
	isDSS := strings.HasPrefix(so.PublicKey, dss)
	if !isRSA && !isDSS {
		return BadOptionsErr{"key does not start with ssh-rsa or ssh-dss"}
	}

	sections := strings.Split(so.PublicKey, " ")
	if len(sections) < 2 {
		keyType := rsa
		if sections[0] == dss {
			keyType = dss
		}
		return BadOptionsErr{fmt.Sprintf("missing space after '%v'", keyType)}
	}

	// check for valid base64
	if _, err = base64.StdEncoding.DecodeString(sections[1]); err != nil {
		return BadOptionsErr{"key contains invalid base64 string"}
	}

	if d.UserData.File != "" {
		if strings.TrimSpace(so.UserData) == "" {
			return BadOptionsErr{}
		}

		var err error
		switch d.UserData.Validate {
		case distro.UserDataFormatFormURLEncoded:
			_, err = url.ParseQuery(so.UserData)
		case distro.UserDataFormatJSON:
			var out map[string]interface{}
			err = json.Unmarshal([]byte(so.UserData), &out)
		case distro.UserDataFormatYAML:
			var out map[string]interface{}
			err = yaml.Unmarshal([]byte(so.UserData), &out)
		}

		if err != nil {
			return BadOptionsErr{fmt.Sprintf("invalid %v: %v", d.UserData.Validate, err)}
		}
	}
	return nil
}

// CreateHost spawns a host with the given options.
func (sm Spawn) CreateHost(so Options, owner *user.DBUser) (*host.Host, error) {

	// load in the appropriate distro
	d, err := distro.FindOne(distro.ById(so.Distro))
	if err != nil {
		return nil, err
	}

	// get the appropriate cloud manager
	cloudManager, err := providers.GetCloudManager(d.Provider, sm.settings)
	if err != nil {
		return nil, err
	}

	// spawn the host
	h, err := cloudManager.SpawnInstance(d, so.UserName, true)
	if err != nil {
		return nil, err
	}

	// set the expiration time for the host
	expireTime := h.CreationTime.Add(DefaultExpiration)
	err = h.SetExpirationTime(expireTime)
	if err != nil {
		return h, evergreen.Logger.Errorf(slogger.ERROR, "error setting expiration on host %v: %v", h.Id, err)
	}

	// set the user data, if applicable
	if so.UserData != "" {
		err = h.SetUserData(so.UserData)
		if err != nil {
			return h, evergreen.Logger.Errorf(slogger.ERROR,
				"Failed setting userData on host %v: %v", h.Id, err)
		}
	}

	// create a hostinit to take care of setting up the host
	init := &hostinit.HostInit{Settings: sm.settings}

	// for making sure the host doesn't take too long to spawn
	startTime := time.Now()

	// spin until the host is ready for its setup script to be run
	for {
		// make sure we haven't been spinning for too long
		if time.Now().Sub(startTime) > 15*time.Minute {
			if err := h.SetDecommissioned(); err != nil {
				evergreen.Logger.Logf(slogger.ERROR, "error decommissioning host %v: %v", h.Id, err)
			}
			return nil, fmt.Errorf("host took too long to come up")
		}

		time.Sleep(5000 * time.Millisecond)

		evergreen.Logger.Logf(slogger.INFO, "Checking if host %v is up and ready", h.Id)

		// see if the host is ready for its setup script to be run
		ready, err := init.IsHostReady(h)
		if err != nil {
			if err := h.SetDecommissioned(); err != nil {
				evergreen.Logger.Logf(slogger.ERROR, "error decommissioning host %v: %v", h.Id, err)
			}
			return nil, fmt.Errorf("error checking on host %v; decommissioning to save resources: %v",
				h.Id, err)
		}

		// if the host is ready, move on to running the setup script
		if ready {
			break
		}

	}

	evergreen.Logger.Logf(slogger.INFO, "Host %v is ready for its setup script to be run", h.Id)

	// add any extra user-specified data into the setup script
	if h.Distro.UserData.File != "" {
		userDataCmd := fmt.Sprintf("echo \"%v\" > %v\n",
			strings.Replace(so.UserData, "\"", "\\\"", -1), h.Distro.UserData.File)
		// prepend the setup script to add the userdata file
		if strings.HasPrefix(h.Distro.Setup, "#!") {
			firstLF := strings.Index(h.Distro.Setup, "\n")
			h.Distro.Setup = h.Distro.Setup[0:firstLF+1] + userDataCmd + h.Distro.Setup[firstLF+1:]
		} else {
			h.Distro.Setup = userDataCmd + h.Distro.Setup
		}
	}

	// modify the setup script to add the user's public key
	h.Distro.Setup += fmt.Sprintf("\necho \"\n%v\" >> ~%v/.ssh/authorized_keys\n", so.PublicKey, h.Distro.User)

	// replace expansions in the script
	exp := command.NewExpansions(init.Settings.Expansions)
	h.Distro.Setup, err = exp.ExpandString(h.Distro.Setup)
	if err != nil {
		return nil, fmt.Errorf("expansions error: %v", err)
	}

	// provision the host
	err = init.ProvisionHost(h)
	if err != nil {
		return nil, fmt.Errorf("error provisioning host %v: %v", h.Id, err)
	}

	// Put the client binary on the host
	loadClientRes, err := init.LoadClient(h, owner)
	if err != nil {
		// if loading the client fails, don't treat it as a fatal error
		evergreen.Logger.Logf(slogger.WARN, "failed loading client on target machine %v: %v", h.Id, err)
	}

	if len(so.TaskId) > 0 {
		err = sm.fetchRemoteTaskData(so.TaskId, loadClientRes.BinaryPath, loadClientRes.ConfigPath, h)
		// if fetching the remote task data fails, don't treat this as a fatal error.
		evergreen.Logger.Logf(slogger.WARN, "failed to fetch remote task data on target machine %v: %v", h.Id, err)
	}

	return h, nil
}

func (sm *Spawn) fetchRemoteTaskData(taskId, cliPath, confPath string, target *host.Host) error {
	hostSSHInfo, err := util.ParseSSHInfo(target.Host)
	if err != nil {
		return fmt.Errorf("error parsing ssh info %v: %v", target.Host, err)
	}

	cloudHost, err := providers.GetCloudHost(target, sm.settings)
	if err != nil {
		return fmt.Errorf("Failed to get cloud host for %v: %v", target.Id, err)
	}
	sshOptions, err := cloudHost.GetSSHOptions()
	if err != nil {
		return fmt.Errorf("Error getting ssh options for host %v: %v", target.Id, err)
	}
	sshOptions = append(sshOptions, "-o", "UserKnownHostsFile=/dev/null")

	// TESTING ONLY
	// Note for testing - when running locally, if your motu URL is behind a gateway (i.e. not a
	// static IP) the next step will fail because the API server will not be reachable.
	// If you want it to reach your local API server, execute a command here that sets up a reverse ssh tunnel:
	// ssh -f -N -T -R 8080:localhost:8080 -o UserKnownHostsFile=/dev/null
	// ... or, add a time.Sleep() here that gives you enough time to log in and edit the config
	// on the spawnhost manually.

	cmdOutput := &util.CappedWriter{&bytes.Buffer{}, 1024 * 1024}
	makeShellCmd := &command.RemoteCommand{
		CmdString:      fmt.Sprintf("%s -c %s fetch -t %s --source --artifacts", cliPath, confPath, taskId),
		Stdout:         io.MultiWriter(os.Stdout, cmdOutput),
		Stderr:         io.MultiWriter(os.Stderr, cmdOutput),
		RemoteHostName: hostSSHInfo.Hostname,
		User:           target.User,
		Options:        append([]string{"-p", hostSSHInfo.Port}, sshOptions...),
	}

	// run the make shell command with a timeout
	err = util.RunFunctionWithTimeout(makeShellCmd.Run, 10*time.Minute)
	return err

}
