package main

// compile:
// env GOARM=7 GOOS=linux GOARCH=arm go build -o firmware.arm7 ./firmware/firmware.go

import (
	"bufio"
	"bytes"
	"crypto/md5"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
)

/*
version=$(cat /usr/lib/version)
sysid=$(cat /etc/board.inc | grep 'board_id=' | cut -d" -f2)  # '$board_id="0xe009";'

wget -O fw.json http://www.ubnt.com/update/check.php\?sysid\=$sysid\&fwver\=$version

    response: {
    "url": "http://dl.ubnt.com/firmwares/XN-fw/v5.6.3/XM.v5.6.3.28591.151130.1749.bin",
    "checksum": "26be1e137bd1991c570a70ae7beee19f", "update": "true",
    "version": "v5.6.3", "date": "151130", "security": ""}

or:

    {"update": "false"}

wget -O fw.bin $(jq -r .url < fw.json)
scp fw.bin ubnt@192.168.1.30:/tmp/fwupdate.bin

ssh ubnt@192.168.1.30 /sbin/fwupdate -m
*/

const defaultUsername = "ubnt"

type Status struct {
	URL      string `json:"url"`
	Checksum string `json:"checksum"`
	Update   string `json:"update"`
	Version  string `json:"version"`
	Date     string `json:"date"`
	Security string `json:"security"`
}

func main() {
	deviceAddress := flag.String("addr", "", "device address")
	flag.Parse()
	if err := UpgradeFirmware(*deviceAddress); err != nil {
		log.Fatalf("%s", err)
	}
}

func UpgradeFirmware(deviceAddress string) error {
	log.Printf("getting current firmware version")
	output := bytes.NewBuffer(nil)
	cmd := exec.Command("ssh",
		fmt.Sprintf("%s@%s", defaultUsername, deviceAddress),
		"cat", "/usr/lib/version")
	cmd.Stdout = output
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	version := strings.TrimSpace(string(output.Bytes()))
	log.Printf("current firmware is %q", version)

	log.Printf("getting device id")
	output = bytes.NewBuffer(nil)
	cmd = exec.Command("ssh",
		fmt.Sprintf("%s@%s", defaultUsername, deviceAddress),
		"cat", "/etc/board.inc")
	cmd.Stdout = output
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	sysID := ""
	scanner := bufio.NewScanner(bytes.NewReader(output.Bytes()))
	for scanner.Scan() {
		if strings.HasPrefix(scanner.Text(), "$board_id=") {
			sysID = strings.SplitN(scanner.Text(), "=", 2)[1]
			sysID = strings.Trim(sysID, "\";")
		}
	}
	log.Printf("device id is %q", sysID)

	log.Printf("checking if version %s is the latest for device %s", version, sysID)
	values := url.Values{}
	values.Set("sysid", sysID)
	values.Set("fwver", version)
	resp, err := http.Get("http://www.ubnt.com/update/check.php?" + values.Encode())
	if err != nil {
		return err
	}

	status := Status{}
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return err
	}
	if status.Update == "false" {
		log.Printf("firmware is up to date")
		return nil
	}

	// fetch the new firmware
	log.Printf("fetching the firmware from %s (md5 %s)",
		status.URL, status.Checksum)
	resp, err = http.Get(status.URL)
	if err != nil {
		return err
	}
	firmwareBuf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	// verify the checksum
	h := md5.New()
	h.Write(firmwareBuf)
	if fmt.Sprintf("%x", h.Sum(nil)) != status.Checksum {
		return fmt.Errorf("%s: expected %s got %c",
			status.URL, status.Checksum, h.Sum(nil))
	}

	log.Printf("copying the update to the device")
	cmd = exec.Command("ssh",
		fmt.Sprintf("%s@%s", defaultUsername, deviceAddress),
		"cat", ">/tmp/fwupdate.bin")
	cmd.Stdin = bytes.NewReader(firmwareBuf)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}

	log.Printf("applying the update")
	cmd = exec.Command("ssh",
		fmt.Sprintf("%s@%s", defaultUsername, deviceAddress),
		"/sbin/fwupdate", "-m")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}

	return nil
}
