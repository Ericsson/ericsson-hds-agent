package agent

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/Ericsson/ericsson-hds-agent/agent/log"
)

// parseDest returns struct Destination from given destination string after validation
func parseDest(dst string) (*Destination, error) {
	d := Destination{}

	d.dst = strings.ToLower(strings.TrimSpace(dst))
	// safe set
	d.proto = ""

	//don't allow more than one destination in the dst parameter (assuming they are comma separated)
	if len(strings.Split(dst, ",")) > 1 {
		return nil, fmt.Errorf("invalid address: %s", dst)
	}

	// if set to "", just ignore
	if dst == "" {
		return &d, nil
	}

	switch {
	case strings.HasPrefix(dst, protoTCP+":"):
		d.proto = protoTCP
		dst = strings.TrimPrefix(dst, protoTCP+":")
		err := isValidAddress(dst)
		if err != nil {
			return nil, fmt.Errorf("given an invalid remote address (requires valid host and port): %v, %v", dst, err)
		}
	default:
		return nil, fmt.Errorf("invalid destination provided: %s", dst)
	}

	d.dst = dst
	return &d, nil
}

// Agent is running without any flags
func getHostPort(addr string) (string, int, error) {
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return "", 0, err
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return "", 0, err
	}
	return host, port, nil
}

func (a *Agent) connect() {
	destination := a.Destination

	log.Infof("agent connectToDestination, dest %s, protocol %s", destination.dst, destination.proto)
	if destination.proto == protoTCP {
		go a.connectTCP(a.SendCh)
	}

	if a.Config.DryRun == true {
		log.Info("destination set to null")
		go a.connectDryRun(a.SendCh)
	}
}

func (a *Agent) connectDryRun(source <-chan []byte) {
	log.Infof("attempt sending to null")

	a.sendInventory()
	for data := range source {
		log.Infof("suppressed output: %s", data)
	}
}

func (a *Agent) connectTCP(source <-chan []byte) {
	var (
		conn    net.Conn
		errc    chan error
		msgc    <-chan []byte
		pending []byte
	)

	ds := a.Destination
	reconnectTimer := time.After(0)
	sendToServer := func(msg []byte) error {
		log.Infof("attempt sending to servaddr '%s'", ds.dst)
		if _, err := conn.Write(msg); err != nil {
			log.Errorf("can't send to server, %s", err)
			return err
		}
		log.Infof("sent %d bytes", len(msg))
		return nil
	}

	connectToServer := func() {
		log.Infof("attempt connecting to servaddr %s", ds.dst)
		reconnectTimer = nil
		errc = make(chan error, 2)
		var err error
		conn, err = a.dialServer(ds)
		if err != nil {
			errc <- err
			return
		}
		log.Infof("successfully connected to %s", ds.dst)

		//send !nodeID and headers message
		nodeIDAndHeaders := a.initialSendData()
		attachListener(conn, a.processCommands, errc)
		if err := sendToServer(nodeIDAndHeaders); err != nil {
			errc <- err
			return
		}
		// Send the inventory
		a.sendInventory()

		// Send pending message
		if len(pending) != 0 {
			if err := sendToServer(pending); err != nil {
				errc <- err
				return
			}
			pending = []byte{}
		}

		msgc = source
	}

	for {
		select {
		case <-reconnectTimer:
			connectToServer()

		case err := <-errc:
			if conn != nil {
				log.Errorf("connection error with server, %s", err)
				log.Infof("closing connection to %s", ds.dst)
				if err := conn.Close(); err != nil {
					log.Errorf("error closing connection: %v", err)
				}
				errc = nil
				msgc = nil
				conn = nil
			} else {
				log.Errorf("connection attempt to %s failed: %s", ds.dst, err)
			}
			log.Errorf("attempting to reconnect in %0.f seconds", a.WaitTime.Seconds())
			reconnectTimer = time.After(a.WaitTime)

		case data := <-msgc:
			if err := sendToServer(append(data, '\n')); err != nil {
				pending = append(data, '\n')
				msgc = nil //here we ensure, that next case of this select will not be on <-msgc
				errc <- err
			}
		}
	}
}

func (a *Agent) initialSendData() []byte {
	log.Info("Sending initial data")
	var initialData string

	metadata := Metadata{
		HostType: "hds-agent",
	}
	metadataBytes, err := json.Marshal(metadata)
	if err == nil {
		initialData = fmt.Sprintf("!nodeID %s\n!metadata %s\n", a.Config.NodeID, string(metadataBytes))
	} else {
		initialData = fmt.Sprintf("!nodeID %s\n", a.Config.NodeID)
		log.Errorf("Couldn't marshal !metadata message, %s", err)
	}
	//add metric headers
	a.metricHeaders.RLock()
	for _, header := range a.metricHeaders.Map {
		initialData += header + "\n"
	}
	a.metricHeaders.RUnlock()

	//add metadata lines
	a.metricMetadata.RLock()
	for metric, metadata := range a.metricMetadata.Map {
		for name, val := range metadata {
			data, err := a.metadataString(metric, name, val)
			if err != nil {
				log.Error(err.Error())
				continue
			}
			initialData = initialData + data + "\n"
		}
	}
	a.metricMetadata.RUnlock()

	return []byte(initialData)
}

// attachListener attaches a callback to a tcp connection, sending connection errors back on provided channel
func attachListener(conn net.Conn, cb func([]byte) bool, errc chan<- error) {
	log.Infof("attaching listener to connection with %s", conn.RemoteAddr().String())
	connection := bufio.NewReader(conn)
	var (
		line []byte
		err  error
	)
	go func() {
		for {
			if line, err = connection.ReadBytes(byte('\n')); err != nil {
				log.Infof("Error reading from %s. Terminating listener...", conn.RemoteAddr().String())
				errc <- err
				return
			}
			if len(line) == 0 {
				continue
			}
			log.Infof("received data: %s", string(line))
			if ok := cb(line); !ok {
				log.Error("error when processing listener callback")
			}
		}
	}()
}

func (a *Agent) dialServer(ds *Destination) (net.Conn, error) {
	var conn net.Conn
	var er error
	if _, _, err := getHostPort(ds.dst); err != nil {
		return nil, fmt.Errorf("can't determine server's address:port from: %s, %s", ds.dst, err)
	}

	conn, er = net.DialTimeout(protoTCP, ds.dst, a.dialTimeout)
	return conn, er
}

func isValidAddress(dst string) error {
	if len(dst) == 0 {
		return fmt.Errorf("empty address")
	}

	host, _, err := net.SplitHostPort(dst)
	if err != nil {
		return fmt.Errorf("address parse error '%s', %v", host, err)
	}
	ip := net.ParseIP(host)
	if ip != nil {
		log.Infof("valid IP %s", host)
		return nil
	}

	_, err = net.LookupHost(host)
	if err != nil {
		return fmt.Errorf("address lookup error '%s', %v", host, err)
	}

	return nil
}
