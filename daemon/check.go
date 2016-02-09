package main

// env GOARM=7 GOOS=linux GOARCH=arm go build -o check.arm7 ./daemon/check.go && scp check.arm7 lobupanet:bin/check

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"time"
)

type StationInfo struct {
	Mac        string            `json:"mac"`
	Name       string            `json:"name"`
	LastIP     string            `json:"lastip"`
	AssocID    int               `json:"associd"`
	APRepeater int               `json:"aprepeater"`
	Tx         float64           `json:"tx"`
	Rx         float64           `json:"rx"`
	Signal     int               `json:"signal"`
	CCQ        int               `json:"ccq"`
	Idle       int               `json:"idle"`
	Uptime     int               `json:"uptime"`
	Ack        int               `json:"ack"`
	Distance   int               `json:"distance"`
	TxPower    int               `json:"txpower"`
	NoiseFloor int               `json:"noisefloor"`
	AirMax     AirMaxStationInfo `json:"airmax"`
	Stats      StationStats      `json:"stats"`
	Rates      []string          `json:"raters"`
	Signals    []int             `json:"signals"`
}

type AirMaxStationInfo struct {
	Priority int `json:"priority"`
	Quality  int `json:"quality"`
	Beam     int `json:"beam"`
	Signal   int `json:"signal"`
	Capacity int `json:"capacity"`
}

type StationStats struct {
	RxData  int `json:"rx_data"`
	RxBytes int `json:"rx_bytes"`
	RxPPS   int `json:"rx_pps"`
	TxData  int `json:"tx_data"`
	TxBytes int `json:"tx_bytes"`
	TxPPS   int `json:"tx_pps"`
}

type SpeedTestResult struct {
	RxRate float64
	TxRate float64
}

// GetStationInfo connects to the specified access point and invokes the
// wstalist command which returns JSON formatted data about each connected
// station.
func GetStationInfo(accessPoint string) ([]StationInfo, error) {
	cmd := exec.Command("ssh", accessPoint, "wstalist")
	stdoutReader, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	cmd.Stderr = os.Stderr

	stations := []StationInfo{}

	errCh := make(chan error)
	go func() {
		errCh <- json.NewDecoder(stdoutReader).Decode(&stations)
	}()

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("wstalist: %s", err)
	}

	if err := <-errCh; err != nil {
		return nil, fmt.Errorf("cannot parse wstalist output: %s", err)
	}
	return stations, nil
}

// DoSpeedTest connects to the specified access point and station and
// performs a speed test.
func DoSpeedTest(accessPoint, station, stationIP string) (*SpeedTestResult, error) {
	serverCmd := exec.Command("ssh", station, "ubntbox", "speedsrv", "-t", "90")
	serverCmd.Stderr = os.Stderr
	serverCmd.Start()
	defer func() {
		if p := serverCmd.Process; p != nil {
			p.Kill()
		}
	}()

	// TODO(ross): this is the time it will take to spin up the speedsrv and
	//   have it start listening. Sleep is evil, make this suck less.
	time.Sleep(3 * time.Second)

	// connect to the local end and run the speedtest
	testCmd := exec.Command("ssh", accessPoint, "ubntbox", "speedtest", "-d", "both", "-q", stationIP)
	output, err := testCmd.CombinedOutput()
	if err != nil {
		return nil, err
	}

	// parse the stdout
	expr := regexp.MustCompile("RX: ([0-9\\.]+) Mbps\\nTX: ([0-9\\.]+) Mbps\\n")
	matches := expr.FindAllSubmatch(output, 1)
	if len(matches) != 1 || len(matches[0]) != 3 {
		return nil, fmt.Errorf("cannot parse speedtest output: %q", string(output))
	}
	rxRateMbps, err := strconv.ParseFloat(string(matches[0][1]), 64)
	if err != nil {
		return nil, fmt.Errorf("cannot parse rx rate in speedtest output: %q", string(output))
	}
	txRateMbps, err := strconv.ParseFloat(string(matches[0][2]), 64)
	if err != nil {
		return nil, fmt.Errorf("cannot parse rx rate in speedtest output: %q", string(output))
	}
	return &SpeedTestResult{
		RxRate: rxRateMbps * 1000 * 1000,
		TxRate: txRateMbps * 1000 * 1000,
	}, nil
}

func main() {
	accessPoint := flag.String("ap", "", "the address of the local (access point) end of the connection")
	station := flag.String("station", "", "the address of theremote end of the connection")
	stationIP := flag.String("station-ip", "", "the address of the remote end from the perspective of the access point (default: --station)")
	doStats := flag.Bool("collect-stats", true, "collect link statistics")
	doSpeedTest := flag.Bool("speed-test", true, "do link speed test")
	influxURL := flag.String("influx-url", "http://localhost:8086", "The base URL to influx DB")
	influxDBName := flag.String("influx-db", "mydb", "The base URL to influx DB")
	flag.Parse()

	if *stationIP == "" {
		*stationIP = *station
	}
	now := time.Now().UnixNano()
	stats := bytes.NewBuffer(nil)

	if *doStats {
		stationsInfo, err := GetStationInfo(*accessPoint)
		if err != nil {
			log.Fatalf("%s", err)
		}

		for _, stationInfo := range stationsInfo {
			fmt.Fprintf(stats, "TX,ap=%s,station=%s value=%f %d\n",
				*accessPoint, stationInfo.Name, stationInfo.Tx, now)
			fmt.Fprintf(stats, "RX,ap=%s,station=%s value=%f %d\n",
				*accessPoint, stationInfo.Name, stationInfo.Rx, now)
			fmt.Fprintf(stats, "Signal,ap=%s,station=%s value=%d %d\n",
				*accessPoint, stationInfo.Name, stationInfo.Signal, now)
			fmt.Fprintf(stats, "CCQ,ap=%s,station=%s value=%d %d\n",
				*accessPoint, stationInfo.Name, stationInfo.CCQ, now)
			fmt.Fprintf(stats, "Distance,ap=%s,station=%s value=%d %d\n",
				*accessPoint, stationInfo.Name, stationInfo.Distance, now)
			fmt.Fprintf(stats, "TxPower,ap=%s,station=%s value=%d %d\n",
				*accessPoint, stationInfo.Name, stationInfo.TxPower, now)
			fmt.Fprintf(stats, "NoiseFloor,ap=%s,station=%s value=%d %d\n",
				*accessPoint, stationInfo.Name, stationInfo.NoiseFloor, now)
			fmt.Fprintf(stats, "AirMaxPriority,ap=%s,station=%s value=%d %d\n",
				*accessPoint, stationInfo.Name, stationInfo.AirMax.Priority, now)
			fmt.Fprintf(stats, "AirMaxQuality,ap=%s,station=%s value=%d %d\n",
				*accessPoint, stationInfo.Name, stationInfo.AirMax.Quality, now)
			fmt.Fprintf(stats, "AirMaxBeam,ap=%s,station=%s value=%d %d\n",
				*accessPoint, stationInfo.Name, stationInfo.AirMax.Beam, now)
			fmt.Fprintf(stats, "AirMaxSignal,ap=%s,station=%s value=%d %d\n",
				*accessPoint, stationInfo.Name, stationInfo.AirMax.Signal, now)
			fmt.Fprintf(stats, "AirMaxCapacity,ap=%s,station=%s value=%d %d\n",
				*accessPoint, stationInfo.Name, stationInfo.AirMax.Capacity, now)
			for i, rate := range stationInfo.Rates {
				signal := stationInfo.Signals[i]
				fmt.Fprintf(stats, "Signal%s,ap=%s,station=%s value=%d %d\n",
					rate, *accessPoint, stationInfo.Name, signal, now)
			}
		}
	}

	if *doSpeedTest {
		speedTestResult, err := DoSpeedTest(*accessPoint, *station, *stationIP)
		if err != nil {
			log.Fatalf("%s", err)
		}
		fmt.Fprintf(stats, "RxSpeedTest,ap=%s,station=%s value=%f %d\n",
			*accessPoint, *station, speedTestResult.RxRate, now)
		fmt.Fprintf(stats, "TxSpeedTest,ap=%s,station=%s value=%f %d\n",
			*accessPoint, *station, speedTestResult.TxRate, now)
	}

	req, err := http.NewRequest("POST",
		fmt.Sprintf("%s/write?db=%s", *influxURL, *influxDBName),
		bytes.NewReader(stats.Bytes()))
	if err != nil {
		log.Fatalf("%s", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatalf("%s", err)
	}
	if resp.StatusCode >= 400 {
		log.Fatalf("influx: %s", resp.Status)
	}
}
