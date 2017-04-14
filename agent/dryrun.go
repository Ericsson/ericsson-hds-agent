package agent

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/Ericsson/ericsson-hds-agent/agent/log"
)

func prepareCaptureStdout() (r, w, old *os.File) {
	old = os.Stdout // keep backup of the real stdout
	r, w, _ = os.Pipe()
	os.Stdout = w
	return r, w, old
}

func captureStdout(r, w, old *os.File) string {
	outC := make(chan string)
	// copy the output in a separate goroutine so printing can't block indefinitely
	go func() {
		var buf bytes.Buffer
		io.Copy(&buf, r)
		outC <- buf.String()
	}()

	// back to normal state
	w.Close()
	os.Stdout = old // restoring the real stdout
	return <-outC
}

func prepareCaptureStderr() (r, w, old *os.File) {
	old = os.Stderr // keep backup of the real stderr
	r, w, _ = os.Pipe()
	os.Stderr = w
	return r, w, old
}

func captureStderr(r, w, old *os.File) string {
	outC := make(chan string)
	// copy the output in a separate goroutine so printing can't block indefinitely
	go func() {
		var buf bytes.Buffer
		io.Copy(&buf, r)
		outC <- buf.String()
	}()

	// back to normal state
	w.Close()
	os.Stderr = old // restoring the real stderr
	return <-outC
}

func findErrors(stderr string) (errorLines []string) {
	lines := strings.Split(stderr, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "E") || strings.Contains(line, " error") {
			errorLines = append(errorLines, line)
		}
	}
	return errorLines
}

func dryRun(context *Agent, initialFreq int) {
	const dryRunSeconds = 10
	log.Consolef("running in 'dry run' mode, all flags will be ignored. It will take %d seconds...", dryRunSeconds)

	r, w, old := prepareCaptureStdout()
	r2, w2, old2 := prepareCaptureStderr()

	go func() {
		<-time.After(dryRunSeconds * time.Second)

		stdout := captureStdout(r, w, old)
		stderr := captureStderr(r2, w2, old2)

		// sleep a little to let agent last logs be printed before printing dry run results
		time.Sleep(100 * time.Millisecond)
		err := dryRunResults(stdout, stderr, initialFreq)
		if err != nil {
			log.Errorf("Exit because of error: %s", err.Error())
			os.Exit(1)
		}
		context.WaitGroup.Done()
		os.Exit(1)
	}()
}
func dryRunResults(stdout, stderr string, frequency int) (err error) {
	log.Consolef("-------- dry run results --------\n")
	fmt.Println("metric header columns and values that were collected")
	displayMetrics(stdout)
	fmt.Println("------------")
	errorLines := findErrors(stderr)

	log.Consolef("dry-run has %d errors\n", len(errorLines))
	if len(errorLines) > 0 {
		for _, line := range errorLines {
			fmt.Println(line)
		}
		fmt.Println()
		fmt.Println()
	}
	pCollectors := passedCollectors(log.ReadLogFile())
	log.Consolef("Collectors report:")
	fmt.Printf("%d successfully running collectors: %s\n", len(pCollectors), strings.Join(pCollectors, ", "))
	fCollectors := failedCollectors(stderr)
	fmt.Printf("%d failed collectors: %s\n", len(fCollectors), strings.Join(fCollectors, ", "))
	bytesSum, itemsSum, err := sentDataStats(log.ReadLogFile())
	if err != nil {
		log.Error(err.Error())
		return err
	}
	fmt.Printf("%d items (%d bytes) will be sent during one iteration\n",
		itemsSum, bytesSum)

	estimatedBytes, estimatedItems := projectDataSize(bytesSum, itemsSum, frequency)
	fmt.Printf("%d items (%d bytes) will be sent over one hour since we have -frequency %d seconds\n",
		estimatedItems, estimatedBytes, frequency)
	return nil
}

func passedCollectors(output string) (pCollectors []string) {
	//Done processing metric: cpu
	var regex = regexp.MustCompile("Done processing metric: (.+)")

	matches := regex.FindAllStringSubmatch(output, -1)
	for _, match := range matches {
		pCollectors = append(pCollectors, match[1])
	}
	return pCollectors
}

func failedCollectors(output string) (fCollectors []string) {
	//Collector sysinfo.bmc.bmc-info will not run because
	var regex = regexp.MustCompile("Collector (.+) will not run because")

	matches := regex.FindAllStringSubmatch(output, -1)
	for _, match := range matches {
		fCollectors = append(fCollectors, match[1])
	}
	return fCollectors
}

func sentDataStats(output string) (bytesSum, itemsSum int, err error) {
	var sentDataRegex = regexp.MustCompile("suppressed output:(.+)")
	matches := sentDataRegex.FindAllStringSubmatch(output, -1)
	for _, match := range matches {
		bytesSum += len(match[1])
		itemsSum++
	}
	return bytesSum, itemsSum, err
}

func projectDataSize(bytesPerIteration, itemsPerIteration, frequency int) (estimatedBytes, estimatedIterations int) {
	if frequency <= 0 {
		return bytesPerIteration, itemsPerIteration
	}
	// how many times agent will send data over one hour:
	timesSent := 3600 / frequency
	return timesSent * bytesPerIteration, timesSent * itemsPerIteration
}

func collectorData(stdout, collectorName string) (headerLine, valuesLine string) {
	lines := strings.Split(stdout, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, ":=:header "+collectorName+" ") {
			headerLine = line
			continue
		}
		if strings.HasPrefix(line, ":=:"+collectorName+" ") {
			valuesLine = line
			continue
		}
	}
	return headerLine, valuesLine
}

func displayMetrics(stdout string) {
	var metricHeaders = regexp.MustCompile(`:=:header (\w+) `)

	matches := metricHeaders.FindAllStringSubmatch(stdout, -1)
	for _, match := range matches {
		mhv := metricsCollector{
			Collector: match[1],
		}
		mhv.Header, mhv.Values = collectorData(stdout, mhv.Collector)
		fmt.Println("--------" + mhv.Collector)
		fmt.Println(mhv.Header)
		fmt.Println("----")
		fmt.Println(mhv.Values)
	}
}
