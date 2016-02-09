package main

// This program provisions a new device. When devices come from the factory
// they are configured to have the address 192.168.1.20. This program watches
// for the device on that IP or on the IP you specify. It modifies the running
// configuration with our settings, saves them to the flash memory and reboots
// the device into the new configuration.
//
// In general it is safe to reprovision an existing device as this program
// changes or if the device configuration drifts.
//
// This program requires the following third party programs: sshpass, ssh, ping
//
// TODO(ross): this doesn't change the password, and it should.
// TODO(ross): configure syslog on devices
// TODO(ross): configure ntp on devices
// TODO(ross): a dry-run mode that shows what configuration would change would be nice.

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"time"
)

const defaultAddress = "192.168.1.20" // the default address for new devices
const defaultPassword = "ubnt"
const defaultUsername = "ubnt"

func main() {
	name := flag.String("name", "", "device name")
	ip := flag.String("ip", "", "device IP address")
	flag.Parse()

	if *name == "" || *ip == "" {
		fmt.Fprintf(os.Stderr, "you must specify --name and --ip\n")
		os.Exit(1)
	}

	if err := Provision(*name, *ip); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
}

// Provision configures a unifi device
func Provision(name, ip string) error {
	// search for the device, either at the target IP or at the default IP
	deviceAddress := ""
	for i := 0; true; i++ {
		if err := exec.Command("ping", "-c1", ip).Run(); err == nil {
			deviceAddress = ip
			break
		}
		if err := exec.Command("ping", "-c1", defaultAddress).Run(); err == nil {
			deviceAddress = defaultAddress
			break
		}
		time.Sleep(time.Second)
		continue
	}
	log.Printf("found device on %s (icmp)", deviceAddress)

	// wait for ssh to work
	for i := 0; true; i++ {
		cmd := exec.Command("sshpass", "-p", defaultPassword,
			"ssh", "-oStrictHostKeyChecking=no", "-oUserKnownHostsFile=/dev/null",
			fmt.Sprintf("%s@%s", defaultUsername, deviceAddress),
			"true")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err == nil {
			log.Printf("found device on %s (ssh)", deviceAddress)
			break
		}
		time.Sleep(time.Second)
		continue
	}

	// grab the current config
	cmd := exec.Command("sshpass", "-p", defaultPassword,
		"ssh", "-oStrictHostKeyChecking=no", "-oUserKnownHostsFile=/dev/null",
		fmt.Sprintf("%s@%s", defaultUsername, deviceAddress),
		"cat", "/tmp/system.cfg")
	output := bytes.NewBuffer(nil)
	cmd.Stdout = output
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("get current config: %s", err)
	}
	config, err := ParseConfig(output.Bytes())
	if err != nil {
		return fmt.Errorf("cannot parse default config: %s", err)
	}

	// apply our configuration changes
	config["resolv.host.1.name"] = name
	config["resolv.host.1.status"] = "enabled"
	config["wireless.1.ssid"] = "LoBuPaNet"
	config["sshd.auth.key.1.comment"] = "root@lobupanet"
	config["sshd.auth.key.1.value"] = "AAAAB3NzaC1yc2EAAAADAQABAAABAQDHKdGw4zj5AJlRkDipXfae31aeEmxixIyzaVZShuS7LzM72rTshPlSym3poIGEjtSZEyEziURvaKMNKIWWEhiZBE2hPmHMuZ7Kle8r7mAn1TquxJALgNj7/yVAE27DJ+y3VF9kmiqsfjXtpCBYTYC83onVxLq1iGmeqCZCw5L4g0pQLOQPmUgV0qkDoR7VzGJfZ/vsWvZwtnNV4r6FMpVbtgJA3PrWaAUZmf3zHqq2oobgo2MbKehBs4L8SBltqLnL7am5v8CS3mgOw+LZKXgR7yNsF2mfkA1GgwYeh4V4NjOvhyfZ4RqVfAfjxxcfWhpDcLwwgyJ3uVuwCjoneRRH"
	config["sshd.auth.key.1.type"] = "ssh-rsa"
	config["sshd.auth.key.1.status"] = "enabled"
	//config["sshd.auth.passwd"] = "disabled"
	//config["ntpclient.status"] = "enabled"
	config["netconf.3.ip"] = ip

	log.Printf("copying new configuration to device")
	cmd = exec.Command("sshpass", "-p", defaultPassword,
		"ssh", "-oStrictHostKeyChecking=no", "-oUserKnownHostsFile=/dev/null",
		fmt.Sprintf("%s@%s", defaultUsername, deviceAddress),
		"/bin/sh", "-c", ""+
			"cat > /tmp/system.cfg && "+
			"cfgmtd -w -p /etc/ && "+
			"reboot")
	cmd.Stdin = bytes.NewReader(FormatConfig(config))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("write new config: %s", err)
	}

	// give reboot above some time to happen
	time.Sleep(3 * time.Second)
	deviceAddress = config["netconf.3.ip"]

	log.Printf("waiting for device to come online at %s", deviceAddress)
	for i := 0; true; i++ {
		err := exec.Command("ping", "-c", "1", deviceAddress).Run()
		if err == nil {
			log.Printf("found device on %s (icmp)", deviceAddress)
			break
		}
		time.Sleep(time.Second)
	}
	for i := 0; true; i++ {
		cmd := exec.Command("sshpass", "-p", defaultPassword,
			"ssh", "-oStrictHostKeyChecking=no", "-oUserKnownHostsFile=/dev/null",
			fmt.Sprintf("%s@%s", defaultUsername, deviceAddress),
			"true")
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err == nil {
			log.Printf("found device on %s (ssh)", deviceAddress)
			break
		}
		time.Sleep(time.Second)
		continue
	}

	// check that the config was applied
	cmd = exec.Command("sshpass", "-p", defaultPassword,
		"ssh", "-oStrictHostKeyChecking=no", "-oUserKnownHostsFile=/dev/null",
		fmt.Sprintf("%s@%s", defaultUsername, deviceAddress),
		"cat", "/tmp/system.cfg")
	output = bytes.NewBuffer(nil)
	cmd.Stdout = output
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("get default config: %s", err)
	}
	newConfig, err := ParseConfig(output.Bytes())
	if err != nil {
		return fmt.Errorf("cannot parse default config: %s", err)
	}
	if !reflect.DeepEqual(config, newConfig) {
		for k, v := range config {
			newV, _ := newConfig[k]
			if newV != v {
				log.Printf("%s is %s expected %s", k, newV, v)
			}
		}

		return fmt.Errorf("new configuration was not applied")
	}
	log.Printf("new configuration applied successfully")

	return nil
}

// ParseConfig reads the config and returns a map of the settings
func ParseConfig(buf []byte) (map[string]string, error) {
	rv := map[string]string{}
	scanner := bufio.NewScanner(bytes.NewReader(buf))
	for scanner.Scan() {
		parts := strings.SplitN(scanner.Text(), "=", 2)
		if len(parts) == 1 && strings.TrimSpace(parts[0]) == "" {
			continue
		}
		rv[parts[0]] = parts[1]
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return rv, nil
}

// FormatConfig returns the config formatted as it will be in the text file
func FormatConfig(c map[string]string) []byte {
	rv := bytes.NewBuffer(nil)
	for k, v := range c {
		fmt.Fprintf(rv, "%s=%s\n", k, v)
	}
	return rv.Bytes()
}
