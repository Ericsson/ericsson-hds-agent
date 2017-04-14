package agent

import (
	"bytes"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/Ericsson/ericsson-hds-agent/agent/log"
)

const (
	syslogSeverityAlert  = 1
	syslogSeverityNotice = 5
	syslogFacilityUser   = 1
)

//processCommands returns true if all commands vere successfully executed
func (a *Agent) processCommands(in []byte) (allSuccess bool) {
	allSuccess = true
	cmds := []command{}
	if err := json.Unmarshal(in, &cmds); err != nil {
		log.Errorf("invalid command received on tcp connection: %s", err)
		return false
	}

	for _, cmd := range cmds {
		switch cmd.Name {
		case "HTTP":
			err := fmt.Errorf("This is opennpagnet there are no HTTP mode for it %d", len(cmd.RunArgs))
			log.Error(err.Error())
			a.sendCmdStatusSyslog(cmd, "error")
			allSuccess = false
			continue

		case "HTTPS":
			err := fmt.Errorf("This is opennpagnet there are no HTTPS mode for it %d", len(cmd.RunArgs))
			log.Error(err.Error())
			a.sendCmdStatusSyslog(cmd, "error")
			allSuccess = false
			continue

		case "ExecCommand":
			log.Infof("executing command: %v", cmd)

			// send syslog that command has been recieved
			a.sendCmdStatusSyslog(cmd, "received")

			// execute the command
			go func(cmd command) {
				if cmdOut, err := a.execCommand(cmd); err != nil {
					log.Errorf("error during execution of [%+v]: %v", cmd, err)
					allSuccess = false

					// send error syslog
					a.sendCmdStatusSyslog(cmd, "error")

					// send command output blob
					cmdOut.Status = "error"
					if cmdOut.Stderr == "" {
						cmdOut.Stderr = err.Error()
					}
					a.sendCmdOutputBlob(cmdOut)
				} else {
					// send success syslog
					a.sendCmdStatusSyslog(cmd, "success")

					// send command output blob
					cmdOut.Status = "success"
					a.sendCmdOutputBlob(cmdOut)
				}
			}(cmd)

		default:
			errStr := fmt.Sprintf("unrecognized command [%s]", cmd.Name)
			log.Error(errStr)
			allSuccess = false
			a.sendCmdStatusSyslog(cmd, "skipped")
			continue
		}
	}

	return
}

func (a *Agent) sendCmdStatusSyslog(cmd command, status string) {
	severity := syslogSeverityNotice
	if status == "error" {
		severity = syslogSeverityAlert
	}

	s := Syslog{
		Tag:       "hds-agent",
		Hostname:  a.hostname,
		Facility:  syslogFacilityUser,
		Severity:  severity,
		Timestamp: time.Now(),
		Message:   a.cmdResponse(cmd.Name, cmd.CmdID, status),
	}

	a.NonBlockingSend(s.formatBytes())
}

func (a *Agent) sendCmdOutputBlob(cmdOut commandOutput) {
	cmdBlob := Blob{Type: "execCommand", NodeID: a.Config.NodeID}
	jsonOut, err := json.Marshal(cmdOut)
	if err != nil {
		log.Errorf("Error marshalling JSON object: %v", cmdOut)
		jsonOut = []byte{}
	}
	h := sha1.New()
	h.Write(jsonOut)
	cmdBlob.Timestamp = fmt.Sprintf("%d", time.Now().Unix())
	cmdBlob.Digest = fmt.Sprintf("%x", h.Sum(nil))
	cmdBlob.Content = jsonOut
	a.NonBlockingSend(cmdBlob.Format())
}

func (a *Agent) cmdResponse(cmdName, cmdID, status string) string {
	return fmt.Sprintf("%s %s %s %s", cmdName, a.Config.NodeID, cmdID, status)
}

func (a *Agent) execCommand(cmd command) (commandOutput, error) {
	cmdOut := commandOutput{
		NodeID:  a.Config.NodeID,
		CmdID:   cmd.CmdID,
		FileURL: cmd.FileURL,
		RunCmd:  cmd.RunCmd,
		RunArgs: cmd.RunArgs,
	}
	cmdStdout := &bytes.Buffer{}
	cmdStderr := &bytes.Buffer{}
	var status string

	// Download file
	status = "downloading"
	a.sendCmdStatusSyslog(cmd, status)
	cmdOut.Status = status

	filename, err := getFilenameFromURL(cmd.FileURL)
	if err != nil {
		return cmdOut, err
	}

	tmpDir, err := ioutil.TempDir(os.TempDir(), "hds-agent")
	if err != nil {
		return cmdOut, fmt.Errorf("error generating temporary directory: %s", err)
	}
	filename = filepath.Join(tmpDir, filename)

	file, err := os.Create(filename)
	if err != nil {
		return cmdOut, fmt.Errorf("error creating local file: %s", err)
	}

	if err := httpGetFile(cmd.FileURL, file); err != nil {
		return cmdOut, fmt.Errorf("error downloading remote file: %s", err)
	}

	// Execute command
	status = "executing"
	a.sendCmdStatusSyslog(cmd, status)
	cmdOut.Status = status

	if isTar(file) || isTarGz(file) || isTgz(file) {
		if err := untar(file); err != nil {
			return cmdOut, fmt.Errorf("error untarring file: %s", err)
		}
	} else if isGz(file) {
		if err := unzip(file); err != nil {
			return cmdOut, fmt.Errorf("error unzipping file: %s", err)
		}
	}

	runFile := filepath.Join(tmpDir, cmd.RunCmd)
	if !strings.HasPrefix(runFile, tmpDir) {
		return cmdOut, fmt.Errorf("refusing request to run executable in a parent directory: %s", cmd.RunCmd)
	}

	// Make the file runnable
	if err := os.Chmod(runFile, 0700); err != nil {
		return cmdOut, fmt.Errorf("error making file executable: %s", err)
	}

	// Close before running:
	file.Close()

	execCmd := exec.Command(runFile, cmd.RunArgs...)
	execCmd.Stdout = cmdStdout
	execCmd.Stderr = cmdStderr
	err = execCmd.Run()
	cmdOut.Stderr = cmdStderr.String()
	cmdOut.Stdout = cmdStdout.String()
	if err != nil {
		// Check if error was caused by exit code
		if exiterr, ok := err.(*exec.ExitError); ok {
			// Try to grab the exit code
			if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
				return cmdOut, fmt.Errorf("error executing command: %s, exit code: %d", err, status)
			}
		}
		return cmdOut, fmt.Errorf("error executing command: %s", err)
	}
	log.Infof("downloaded command output: %s", cmdOut.Stdout)

	return cmdOut, nil
}

// httpGetFile grabs a file from a url and writes it to a provided io.Writer
func httpGetFile(url string, file io.Writer) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("error fetching file: %s", resp.Status)
	}
	defer resp.Body.Close()

	n, err := io.Copy(file, resp.Body)
	if err != nil {
		return err
	}

	log.Infof("received %d bytes from %s", n, url)
	return nil
}
