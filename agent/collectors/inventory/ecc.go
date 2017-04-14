package inventory

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/Ericsson/ericsson-hds-agent/agent/collectors/types"
	"github.com/Ericsson/ericsson-hds-agent/agent/log"
)

const (
	mcelogConfigurationFile = "/etc/mcelog/mcelog.conf"
	mcelogDefaultPath       = "/var/log/mcelog"
	edacHomeMemory          = "/sys/devices/system/edac/mc"

	errorMcelogCE     = "Corrected error"
	errorMcelogUE     = "Uncorrected error"
	errorMcelogHeader = "Hardware event"
)

func (mclog *mcelog) formatToEntity() *types.Entry {
	e := types.Entry{}
	e.Category = fmt.Sprintf("CPU: %s BANK: %s", mclog.cpu, mclog.bank)

	dCPU := types.Detail{Tag: "CPU", Value: mclog.cpu}
	e.Details = append(e.Details, dCPU)
	dBank := types.Detail{Tag: "BANK", Value: mclog.bank}
	e.Details = append(e.Details, dBank)
	dCe := types.Detail{Tag: "CE", Value: strconv.Itoa(mclog.ce)}
	e.Details = append(e.Details, dCe)
	dUe := types.Detail{Tag: "UE", Value: strconv.Itoa(mclog.ue)}
	e.Details = append(e.Details, dUe)
	return &e
}

// ECCPrecheck validates ECC dependencies mcelog and dmidecode
func ECCPrecheck() error {
	err := isMcelog()
	if err == nil {
		return nil //mcelog is exists
	}

	err = isEdacLog()
	if err == nil {
		return nil //edac is exists
	}
	return errors.New("Mcelog and EDAC module is not found")
}

//check is Mcelog support
func isMcelog() error {

	if _, err := exec.LookPath("mcelog"); err != nil {
		return err
	}

	if len(getMCELOGFilePath(mcelogConfigurationFile)) == 0 {
		return errors.New("MCELOG logfile place not found")
	}

	dmi, er := exec.LookPath("dmidecode")
	if er != nil {
		return er
	}

	output, _ := exec.Command(dmi, "-t processor").Output()

	result := string(output) //MCELOG is not support AMD and not support 32 bit sys
	if strings.Contains(result, "AMD") || strings.Contains(result, "32-bit") {
		return errors.New("MCELOG is not support AMD and 32 bits")
	}
	return nil

}

//check is edac support
func isEdacLog() error {
	if _, err := os.Stat(edacHomeMemory + "/mc0"); err != nil {
		return fmt.Errorf("EDAC is not found: %v", err)
	}

	return nil
}

// ECCRun returns inventory of ECC in []byte
func ECCRun() ([]byte, error) {
	var logs []mcelog

	if isMcelog() == nil {
		logs = parseMcelog(getMCELOGFilePath(mcelogConfigurationFile))
	} else {
		logs = parseEdacFolder(edacHomeMemory)
	}

	result := formatLogMCELOG(logs)
	return json.Marshal(result)
}

func parseEdacFolder(path string) []mcelog {
	var eccLogErr = make([]mcelog, 0)
	memoryControllers, err := filepath.Glob(edacHomeMemory + "/mc*")
	if err != nil {
		return eccLogErr
	}

	for _, mc := range memoryControllers {
		processMemoryController(mc, eccLogErr)
	}

	return eccLogErr
}

func processMemoryController(mcFolder string, eccLogErr []mcelog) []mcelog {
	csrows, _ := filepath.Glob(mcFolder + "/csrow*")
	cpuRaw := regexp.MustCompile("mc(\\d+)").FindStringSubmatch(mcFolder)

	cpuNumber := cpuRaw[1]
	for _, csrow := range csrows {
		processCsrows(cpuNumber, csrow, eccLogErr)
	}

	return eccLogErr
}

func processCsrows(cpuNumber, csrow string, eccLogErr []mcelog) {
	ceTotal, _ := ioutil.ReadFile(csrow + "/ce_count") //Corrected error
	ueTotal, _ := ioutil.ReadFile(csrow + "/ue_count") //Uncorrected error

	if string(ceTotal) == "0" && string(ueTotal) == "0" {
		return
	}
	chanCount := 0
	for true {

		bank, err := ioutil.ReadFile(csrow + fmt.Sprintf("/ch%d_dimm_label", chanCount))
		if err != nil {
			break // no new chan
		}

		ceRaw, _ := ioutil.ReadFile(csrow + fmt.Sprintf("/ch%d_ce_count", chanCount))
		ueRaw, _ := ioutil.ReadFile(csrow + fmt.Sprintf("/ch%d_ue_count", chanCount))
		ce, _ := strconv.Atoi(string(ceRaw))
		ue, _ := strconv.Atoi(string(ueRaw))
		logErr := mcelog{bank: string(bank), ce: ce, ue: ue, cpu: cpuNumber}

		if logErr.ue != 0 || logErr.ce != 0 {
			if exists, pos := containsLog(eccLogErr, logErr); exists {
				eccLogErr[pos].ce += logErr.ce
				eccLogErr[pos].ue += logErr.ue
			} else {
				eccLogErr = append(eccLogErr, logErr)
			}

		}
		chanCount++
	}
}

func formatLogMCELOG(logs []mcelog) *types.GenericInfo {
	g := types.GenericInfo{}
	g.Entries = make([]types.Entry, 0)

	for _, l := range logs {
		log.Infof("Log: CPU: %s, BANK: %s", l.cpu, l.bank)

		e := l.formatToEntity()
		g.Entries = append(g.Entries, *e)
	}
	return &g
}

//move all data from mcelog log to struct
func parseMcelog(mcelogFilepath string) []mcelog {

	output, err := exec.Command("grep", errorMcelogHeader, "-A", "15", mcelogFilepath).Output()
	if err != nil {
		return nil
	}
	filteredLog := strings.Split(string(output), errorMcelogHeader)
	var mceLogErr = make([]mcelog, 0)
	if len(filteredLog) == 0 {
		return mceLogErr
	}

	cpuReg := regexp.MustCompile("CPU (\\d+)")
	bankReg := regexp.MustCompile("BANK (\\d+)")
	for _, log := range filteredLog {
		cpuRaw := cpuReg.FindStringSubmatch(log)
		bakRaw := bankReg.FindStringSubmatch(log)
		var cpu, bank string
		var ce, ue int
		if len(cpuRaw) == 2 {
			cpu = cpuRaw[1]
		}
		if len(bakRaw) == 2 {
			bank = bakRaw[1]
		}
		if strings.Contains(log, errorMcelogCE) {
			ce = 1
		}
		if strings.Contains(log, errorMcelogUE) {
			ue = 1
		}
		if ue == 0 && ce == 0 { //Not recognized error
			continue
		}
		loggedError := mcelog{cpu: cpu, bank: bank, ce: ce, ue: ue}
		if exists, pos := containsLog(mceLogErr, loggedError); exists {

			mceLogErr[pos].ce += loggedError.ce
			mceLogErr[pos].ue += loggedError.ue
		} else {
			mceLogErr = append(mceLogErr, loggedError)
		}
	}
	return mceLogErr
}

func containsLog(logs []mcelog, log mcelog) (bool, int) {
	for i, a := range logs {
		if a.bank == log.bank && a.cpu == log.cpu {
			return true, i
		}
	}
	return false, -1
}

//Get path to mcelog logfile from mcelog config
func getMCELOGFilePath(confPath string) string {
	if _, err := os.Stat(confPath); err != nil {
		return "" //mcelog config file can't be read (miss or no accsess)
	}
	output, _ := exec.Command("grep", "^logfile", confPath).Output()

	conf := string(output)

	if len(conf) < 2 {
		return mcelogDefaultPath
	}

	for _, line := range strings.Split(conf, "\n") {
		pos := strings.Index(line, "=")
		if pos > 0 {
			path := line[pos+1:]
			return strings.TrimSpace(path)
		}
	}
	return "" //not found
}
