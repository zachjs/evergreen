package hostinit

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/10gen-labs/slogger/v1"
	"github.com/evergreen-ci/evergreen"
	"github.com/evergreen-ci/evergreen/alerts"
	"github.com/evergreen-ci/evergreen/cloud"
	"github.com/evergreen-ci/evergreen/cloud/providers"
	"github.com/evergreen-ci/evergreen/command"
	"github.com/evergreen-ci/evergreen/model/event"
	"github.com/evergreen-ci/evergreen/model/host"
	"github.com/evergreen-ci/evergreen/model/user"
	"github.com/evergreen-ci/evergreen/notify"
	"github.com/evergreen-ci/evergreen/util"
	"gopkg.in/mgo.v2"
)

const (
	SCPTimeout = time.Minute
)

// Error indicating another hostinit got to the setup first.
var (
	ErrHostAlreadyInitializing = errors.New("Host already initializing")
)

// Longest duration allowed for running setup script.
var (
	SSHTimeoutSeconds = int64(300) // 5 minutes
)

// HostInit is responsible for running setup scripts on Evergreen hosts.
type HostInit struct {
	Settings *evergreen.Settings
}

// setupReadyHosts runs the distro setup script of all hosts that are up and reachable.
func (init *HostInit) setupReadyHosts() error {
	// set SSH timeout duration
	if timeoutSecs := init.Settings.HostInit.SSHTimeoutSeconds; timeoutSecs <= 0 {
		evergreen.Logger.Logf(slogger.WARN, "SSH timeout set to %vs (<= 0s) using %vs instead", timeoutSecs, SSHTimeoutSeconds)
	} else {
		SSHTimeoutSeconds = timeoutSecs
	}

	// find all hosts in the uninitialized state
	uninitializedHosts, err := host.Find(host.IsUninitialized)
	if err != nil {
		return fmt.Errorf("error fetching uninitialized hosts: %v", err)
	}

	evergreen.Logger.Logf(slogger.DEBUG, "There are %v uninitialized hosts",
		len(uninitializedHosts))

	// used for making sure we don't exit before a setup script is done
	wg := &sync.WaitGroup{}

	for _, h := range uninitializedHosts {

		// check whether or not the host is ready for its setup script to be run
		ready, err := init.IsHostReady(&h)
		if err != nil {
			evergreen.Logger.Logf(slogger.ERROR, "Error checking host %v for readiness: %v",
				h.Id, err)
			continue
		}

		// if the host isn't ready (for instance, it might not be up yet), skip it
		if !ready {
			evergreen.Logger.Logf(slogger.DEBUG, "Host %v not ready for setup", h.Id)
			continue
		}

		evergreen.Logger.Logf(slogger.INFO, "Running setup script for host %v", h.Id)

		// kick off the setup, in its own goroutine, so pending setups don't have
		// to wait for it to finish
		wg.Add(1)
		go func(h host.Host) {

			if err := init.ProvisionHost(&h); err != nil {
				evergreen.Logger.Logf(slogger.ERROR, "Error provisioning host %v: %v", h.Id, err)

				// notify the admins of the failure
				subject := fmt.Sprintf("%v Evergreen provisioning failure on %v",
					notify.ProvisionFailurePreface, h.Distro.Id)
				hostLink := fmt.Sprintf("%v/host/%v", init.Settings.Ui.Url, h.Id)
				message := fmt.Sprintf("Provisioning failed on %v host -- %v: see %v",
					h.Distro.Id, h.Id, hostLink)
				if err := notify.NotifyAdmins(subject, message, init.Settings); err != nil {
					evergreen.Logger.Errorf(slogger.ERROR, "Error sending email: %v", err)
				}
			}

			wg.Done()

		}(h)

	}

	// let all setup routines finish
	wg.Wait()

	return nil
}

// IsHostReady returns whether or not the specified host is ready for its setup script
// to be run.
func (init *HostInit) IsHostReady(host *host.Host) (bool, error) {

	// fetch the appropriate cloud provider for the host
	cloudMgr, err := providers.GetCloudManager(host.Distro.Provider, init.Settings)
	if err != nil {
		return false,
			fmt.Errorf("failed to get cloud manager for provider %v: %v", host.Distro.Provider, err)
	}

	// ask for the instance's status
	hostStatus, err := cloudMgr.GetInstanceStatus(host)
	if err != nil {
		return false, fmt.Errorf("error checking instance status of host %v: %v", host.Id, err)
	}

	// if the host isn't up yet, we can't do anything
	if hostStatus != cloud.StatusRunning {
		return false, nil
	}

	// set the host's dns name, if it is not set
	if host.Host == "" {

		// get the DNS name for the host
		hostDNS, err := cloudMgr.GetDNSName(host)
		if err != nil {
			return false, fmt.Errorf("error checking DNS name for host %v: %v", host.Id, err)
		}

		// sanity check for the host DNS name
		if hostDNS == "" {
			return false, fmt.Errorf("instance %v is running but not returning a DNS name",
				host.Id)
		}

		// update the host's DNS name
		if err := host.SetDNSName(hostDNS); err != nil {
			return false, fmt.Errorf("error setting DNS name for host %v: %v", host.Id, err)
		}

	}

	// check if the host is reachable via SSH
	cloudHost, err := providers.GetCloudHost(host, init.Settings)
	if err != nil {
		return false, fmt.Errorf("failed to get cloud host for %v: %v", host.Id, err)
	}
	reachable, err := cloudHost.IsSSHReachable()
	if err != nil {
		return false, fmt.Errorf("error checking if host %v is reachable: %v", host.Id, err)
	}

	// at this point, we can run the setup if the host is reachable
	return reachable, nil
}

// setupHost runs the specified setup script for an individual host. Returns
// the output from running the script remotely, as well as any error that
// occurs. If the script exits with a non-zero exit code, the error will be non-nil.
func (init *HostInit) setupHost(targetHost *host.Host) ([]byte, error) {

	// fetch the appropriate cloud provider for the host
	cloudMgr, err := providers.GetCloudManager(targetHost.Provider, init.Settings)
	if err != nil {
		return nil,
			fmt.Errorf("failed to get cloud manager for host %v with provider %v: %v",
				targetHost.Id, targetHost.Provider, err)
	}

	// mark the host as initializing
	if err := targetHost.SetInitializing(); err != nil {
		if err == mgo.ErrNotFound {
			return nil, ErrHostAlreadyInitializing
		} else {
			return nil, fmt.Errorf("database error: %v", err)
		}
	}

	// run the function scheduled for when the host is up
	err = cloudMgr.OnUp(targetHost)
	if err != nil {
		// if this fails it is probably due to an API hiccup, so we keep going.
		evergreen.Logger.Logf(slogger.WARN, "OnUp callback failed for host '%v': '%v'", targetHost.Id, err)
	}

	// run the remote setup script as sudo, if appropriate
	sudoStr := ""
	if targetHost.Distro.SetupAsSudo {
		sudoStr = "sudo "
	}

	// parse the hostname into the user, host and port
	hostInfo, err := util.ParseSSHInfo(targetHost.Host)
	if err != nil {
		return nil, err
	}
	user := targetHost.Distro.User
	if hostInfo.User != "" {
		user = hostInfo.User
	}

	// create a temp file for the setup script
	fileName := "setup.sh"
	file, err := ioutil.TempFile("", fileName)
	if err != nil {
		return nil, fmt.Errorf("error creating setup script: %v", err)
	}
	defer func() {
		file.Close()
		os.Remove(file.Name())
	}()

	// build the setup script
	setup, err := init.buildSetupScript(targetHost)
	if err != nil {
		return nil, fmt.Errorf("error building setup script for host %v: %v", targetHost.Id, err)
	}

	// write the setup script to the file
	if _, err := file.Write([]byte(setup)); err != nil {
		return nil, fmt.Errorf("error writing remote setup script: %v", err)
	}

	cloudHost, err := providers.GetCloudHost(targetHost, init.Settings)
	if err != nil {
		return nil, fmt.Errorf("Failed to get cloud host for %v: %v", targetHost.Id, err)
	}
	sshOptions, err := cloudHost.GetSSHOptions()
	if err != nil {
		return nil, fmt.Errorf("Error getting ssh options for host %v: %v", targetHost.Id, err)
	}

	// copy setup script over to the remote machine
	var scpSetupCmdStderr bytes.Buffer
	scpSetupCmd := &command.ScpCommand{
		Source:         file.Name(),
		Dest:           fileName,
		Stdout:         &scpSetupCmdStderr,
		Stderr:         &scpSetupCmdStderr,
		RemoteHostName: hostInfo.Hostname,
		User:           user,
		Options:        append([]string{"-P", hostInfo.Port}, sshOptions...),
	}

	// run the command to scp the setup script with a timeout
	err = util.RunFunctionWithTimeout(scpSetupCmd.Run, SCPTimeout)
	if err != nil {
		if err == util.ErrTimedOut {
			scpSetupCmd.Stop()
			return nil, fmt.Errorf("scp-ing setup script timed out")
		}
		return nil, fmt.Errorf("error (%v) copying setup script to remote "+
			"machine: %v", err, scpSetupCmdStderr.String())
	}

	// run command to ssh into remote machine and execute setup script
	var sshSetupCmdStderr bytes.Buffer
	runSetupCmd := &command.RemoteCommand{
		CmdString:      sudoStr + "sh " + fileName,
		Stdout:         &sshSetupCmdStderr,
		Stderr:         &sshSetupCmdStderr,
		RemoteHostName: hostInfo.Hostname,
		User:           user,
		Options:        []string{"-p", hostInfo.Port},
		Background:     false,
	}

	// only force creation of a tty if sudo
	if targetHost.Distro.SetupAsSudo {
		runSetupCmd.Options = []string{"-t", "-t", "-p", hostInfo.Port}
	}
	runSetupCmd.Options = append(runSetupCmd.Options, sshOptions...)

	// run the ssh command with given timeout
	err = util.RunFunctionWithTimeout(
		runSetupCmd.Run,
		time.Duration(SSHTimeoutSeconds)*time.Second,
	)

	return sshSetupCmdStderr.Bytes(), err
}

// Build the setup script that will need to be run on the specified host.
func (init *HostInit) buildSetupScript(h *host.Host) (string, error) {
	// replace expansions in the script
	exp := command.NewExpansions(init.Settings.Expansions)
	setupScript, err := exp.ExpandString(h.Distro.Setup)
	if err != nil {
		return "", fmt.Errorf("expansions error: %v", err)
	}
	return setupScript, err
}

// Provision the host, and update the database accordingly.
func (init *HostInit) ProvisionHost(h *host.Host) error {

	// run the setup script
	output, err := init.setupHost(h)

	// deal with any errors that occured while running the setup
	if err != nil {
		evergreen.Logger.Logf(slogger.ERROR, "Error running setup script: %v", err)

		// another hostinit process beat us there
		if err == ErrHostAlreadyInitializing {
			evergreen.Logger.Logf(slogger.DEBUG, "Attempted to initialize already initializing host %v", h.Id)
			return nil
		}

		// log the provisioning failure
		setupLog := ""
		if output != nil {
			setupLog = string(output)
		}
		alerts.RunHostProvisionFailTriggers(h)
		event.LogProvisionFailed(h.Id, setupLog)

		// setup script failed, mark the host's provisioning as failed
		if err := h.SetUnprovisioned(); err != nil {
			evergreen.Logger.Logf(slogger.ERROR, "unprovisioning host %v failed: %v", h.Id, err)
		}

		return fmt.Errorf("error initializing host %v: %v", h.Id, err)

	}

	// the setup was successful. update the host accordingly in the database
	if err := h.MarkAsProvisioned(); err != nil {
		return fmt.Errorf("error marking host %v as provisioned: %v", err)
	}

	evergreen.Logger.Logf(slogger.INFO, "Host %v successfully provisioned", h.Id)

	return nil
}

// LocateCLIBinary returns the (absolute) path to the CLI binary for the given architecture, based
// on the system settings. Returns an error if the file does not exist.
func LocateCLIBinary(settings *evergreen.Settings, architecture string) (string, error) {
	clientsSubDir := "clients"
	if settings.ClientBinariesDir != "" {
		clientsSubDir = settings.ClientBinariesDir
	}

	var path string
	if filepath.IsAbs(clientsSubDir) {
		path = filepath.Join(clientsSubDir, architecture, "main")
	} else {
		path = filepath.Join(evergreen.FindEvergreenHome(), clientsSubDir, architecture, "main")
	}
	_, err := os.Stat(path)
	if err != nil {
		return path, err
	}
	return filepath.Abs(path)
}

type LoadClientResult struct {
	BinaryPath string
	ConfigPath string
}

// LoadClient places the evergreen command line client on the host, places a copy of the user's
// settings onto the host, and makes the binary appear in the $PATH when the user logs in.
func (init *HostInit) LoadClient(target *host.Host, user *user.DBUser) (*LoadClientResult, error) {
	// Make sure we have the binary we want to upload - if it hasn't been built for the given
	// architecture, fail early
	cliBinaryPath, err := LocateCLIBinary(init.Settings, target.Distro.Arch)
	if err != nil {
		return nil, fmt.Errorf("Couldn't locate CLI binary for upload: %v", err)
	}

	// 1. mkdir the destination directory on the host,
	//    and modify ~/.profile so the target binary will be on the $PATH
	targetDir := "cli_bin"
	hostSSHInfo, err := util.ParseSSHInfo(target.Host)
	if err != nil {
		return nil, fmt.Errorf("error parsing ssh info %v: %v", target.Host, err)
	}

	cloudHost, err := providers.GetCloudHost(target, init.Settings)
	if err != nil {
		return nil, fmt.Errorf("Failed to get cloud host for %v: %v", target.Id, err)
	}
	sshOptions, err := cloudHost.GetSSHOptions()
	if err != nil {
		return nil, fmt.Errorf("Error getting ssh options for host %v: %v", target.Id, err)
	}
	sshOptions = append(sshOptions, "-o", "UserKnownHostsFile=/dev/null")

	mkdirOutput := &util.CappedWriter{&bytes.Buffer{}, 1024 * 1024}

	// Create the directory for the binary to be uploaded into.
	// Also, make a best effort to add the binary's location to $PATH upon login. If we can't do
	// this successfully, the command will still succeed, it just means that the user will have to
	// use an absolute path (or manually set $PATH in their shell) to execute it.
	makeShellCmd := &command.RemoteCommand{
		CmdString:      fmt.Sprintf("mkdir -m 777 -p ~/%s && (echo 'PATH=$PATH:~/%s' >> ~/.profile; echo 'PATH=$PATH:~/%s' >> ~/.bash_profile || true)", targetDir, targetDir, targetDir),
		Stdout:         mkdirOutput,
		Stderr:         mkdirOutput,
		RemoteHostName: hostSSHInfo.Hostname,
		User:           target.User,
		Options:        append([]string{"-p", hostSSHInfo.Port}, sshOptions...),
	}

	// 2. scp the binary to that directory
	scpOut := &util.CappedWriter{&bytes.Buffer{}, 1024 * 1024}
	// run the make shell command with a timeout
	err = util.RunFunctionWithTimeout(makeShellCmd.Run, 30*time.Second)
	if err != nil {
		return nil, fmt.Errorf("error running setup command for cli, %v: '%v'", mkdirOutput.Buffer.String(), err)
	}
	// place the binary into the directory
	scpSetupCmd := &command.ScpCommand{
		Source:         cliBinaryPath,
		Dest:           fmt.Sprintf("~/%s/evergreen", targetDir),
		Stdout:         scpOut,
		Stderr:         scpOut,
		RemoteHostName: hostSSHInfo.Hostname,
		User:           target.User,
		Options:        append([]string{"-P", hostSSHInfo.Port}, sshOptions...),
	}

	// run the command to scp the setup script with a timeout
	err = util.RunFunctionWithTimeout(scpSetupCmd.Run, 2*time.Minute)
	if err != nil {
		return nil, fmt.Errorf("error running SCP command for cli, %v: '%v'", scpOut.Buffer.String(), err)
	}

	// 4. Write a settings file for the user that owns the host, and scp it to the directory
	outputStruct := struct {
		User    string `json:"user"`
		APIKey  string `json:"api_key"`
		APIHost string `json:"api_server_host"`
		UIHost  string `json:"ui_server_host"`
	}{user.Id, user.APIKey, init.Settings.ApiUrl + "/api", init.Settings.Ui.Url}
	outputJSON, err := json.Marshal(outputStruct)
	if err != nil {
		return nil, err
	}

	tempFileName, err := util.WriteTempFile("", outputJSON)
	if err != nil {
		return nil, err
	}

	err = util.RunFunctionWithTimeout(
		(&command.ScpCommand{
			Source:         tempFileName,
			Dest:           fmt.Sprintf("~/%s/.evergreen.yml", targetDir),
			Stdout:         scpOut,
			Stderr:         scpOut,
			RemoteHostName: hostSSHInfo.Hostname,
			User:           target.User,
			Options:        append([]string{"-P", hostSSHInfo.Port}, sshOptions...),
		}).Run, 30*time.Second)
	if err != nil {
		return nil, fmt.Errorf("error running SCP command for evergreen.yml, %v: '%v'", scpOut.Buffer.String(), err)
	}

	defer os.Remove(tempFileName)
	return &LoadClientResult{
		BinaryPath: fmt.Sprintf("~/%s/evergreen", targetDir),
		ConfigPath: fmt.Sprintf("~/%s/.evergreen.yml", targetDir),
	}, nil
}
