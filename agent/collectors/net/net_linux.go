package net

import (
	"io/ioutil"
	"strings"

	"github.com/Ericsson/ericsson-hds-agent/agent/collectors"
	"github.com/Ericsson/ericsson-hds-agent/agent/log"
)

const (
	netBytesRX      = "bytesRX"
	netPacketsRX    = "packetsRX"
	netErrsRX       = "errsRX"
	netDropRX       = "dropRX"
	netFifoRX       = "fifoRX"
	netFrameRX      = "frameRX"
	netCompressedRX = "compressedRX"
	netMulticastRX  = "multicastRX"
	netBytesTX      = "bytesTX"
	netPacketsTX    = "packetsTX"
	netErrsTX       = "errsTX"
	netDropTX       = "dropTX"
	netFifoTX       = "fifoTX"
	netCollsTX      = "collsTX"
	netCarrierTX    = "carrierTX"
	netCompressedTX = "compressedTX"
)

func loader() ([]byte, error) {
	return ioutil.ReadFile("/proc/net/dev")
}

func preformatter(data []byte) ([]*collectors.MetricResult, error) {
	netColumns := []string{netBytesRX, netPacketsRX, netErrsRX, netDropRX, netFifoRX, netFrameRX, netCompressedRX, netMulticastRX, netBytesTX, netPacketsTX, netErrsTX, netDropTX, netFifoTX, netCollsTX, netCarrierTX, netCompressedTX}
	netMetadataProto := map[string]string{
		netBytesRX:      "int The total number of bytes of data received",
		netPacketsRX:    "int The total number of packets of data received",
		netErrsRX:       "int The total number of receive errors detected by the device driver.",
		netDropRX:       "int The total number of received packets dropped by the device driver.",
		netFifoRX:       "int The number of received FIFO buffer errors.",
		netFrameRX:      "int The number of packet framing errors.",
		netCompressedRX: "int The number of compressed packets received by the device driver",
		netMulticastRX:  "int The number of multicast frames received by the device driver",
		netBytesTX:      "int The total number of bytes of data transmitted",
		netPacketsTX:    "int The total number of packets of data transmitted",
		netErrsTX:       "int The total number of transmit errors detected by the device driver.",
		netDropTX:       "int The total number of transmited packets dropped by the device driver.",
		netFifoTX:       "int The number of transmited FIFO buffer errors",
		netCollsTX:      "int The number of collisions detected on the interface.",
		netCarrierTX:    "int The number of carrier losses detected by the device driver.",
		netCompressedTX: "int The number of compressed packets transmitted by the device driver",
	}
	metadata := map[string]string{}
	lines := strings.Split(string(data), "\n")
	headers := make([]string, 0)
	metrics := make([]string, 0)
	// Could we handle all of the file's columns?
	allCols := true
	for _, line := range lines[2:] {
		columns := strings.Fields(strings.Replace(line, ":", " ", -1))
		var end int

		if len(columns) < 2 {
			continue
		}

		for i, value := range columns[1:] {
			if i >= len(netColumns) {
				log.Infof("more columns than expected while collecting from /proc/net/dev")
				break
			}
			header := strings.Split(columns[0], ":")[0] + "." + netColumns[i]
			headers = append(headers, header)
			metrics = append(metrics, value)
			if v, ok := netMetadataProto[netColumns[i]]; ok {
				metadata[header] = v
			}

			end = i
		}

		if end+1 < len(netColumns) {
			allCols = false
		}
	}

	if !allCols {
		log.Infof("fewer columns than expected while collecting from /proc/net/dev")
	}

	result := collectors.BuildMetricResult(strings.Join(headers, " "), strings.Join(metrics, " "), "", metadata)
	return []*collectors.MetricResult{result}, nil
}
