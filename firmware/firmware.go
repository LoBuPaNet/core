// This program updates the firmware on the specified device to the latest
// available firmware from the vendor.
package main

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

const defaultUsername = "ubnt"

// Status represents the response we get back from the ubnt
// firmware upgrade check service.
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

// UpgradeFirmware upgrades the firmware on the specified device to the
// latest available.
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
	boardID := ""
	scanner := bufio.NewScanner(bytes.NewReader(output.Bytes()))
	for scanner.Scan() {
		if strings.HasPrefix(scanner.Text(), "$board_id=") {
			boardID = strings.SplitN(scanner.Text(), "=", 2)[1]
			boardID = strings.Trim(boardID, "\";")
		}
	}
	log.Printf("device id is %q", boardID)

	log.Printf("checking if version %s is the latest for device %s", version, boardID)
	values := url.Values{}
	values.Set("sysid", boardID)
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
