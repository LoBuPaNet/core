
AirOS devices ship with a default IP of 192.168.1.20. 

apt-get install sshpass
ifconfig eth0:1 192.168.1.1 up
for {
    if !ping 192.168.1.20; then
        sleep 10
    fi
    echo "found new device to provision"

    name=APHenry1
    ip=192.168.1.30

    sshpass -p ubnt ssh -oStrictHostKeyChecking=no ubnt@192.168.1.20 \
        cat /etc/default.cfg > default.cfg

    (
        cat default.cfg
        
        echo "resolv.host.1.name=$name"
        echo "wireless.1.ssid=LoBuPaNet"
        echo "sshd.auth.key.1.comment=root@lobupanet"
        echo "sshd.auth.key.1.value=AAAAB3NzaC1yc2EAAAADAQABAAABAQDHKdGw4zj5AJlRkDipXfae31aeEmxixIyzaVZShuS7LzM72rTshPlSym3poIGEjtSZEyEziURvaKMNKIWWEhiZBE2hPmHMuZ7Kle8r7mAn1TquxJALgNj7/yVAE27DJ+y3VF9kmiqsfjXtpCBYTYC83onVxLq1iGmeqCZCw5L4g0pQLOQPmUgV0qkDoR7VzGJfZ/vsWvZwtnNV4r6FMpVbtgJA3PrWaAUZmf3zHqq2oobgo2MbKehBs4L8SBltqLnL7am5v8CS3mgOw+LZKXgR7yNsF2mfkA1GgwYeh4V4NjOvhyfZ4RqVfAfjxxcfWhpDcLwwgyJ3uVuwCjoneRRH"
        echo "sshd.auth.key.1.type=ssh-rsa"
        echo "sshd.auth.key.1.status=enabled"
        echo "sshd.auth.passwd=disabled"

        echo "ntpclient.status=enabled"

        echo "netconf.3.ip=$ip"
    ) > config

    sshpass -p ubnt ssh -oStrictHostKeyChecking=no ubnt@192.168.1.20 \
        /bin/sh -c 'cat > /tmp/system.cfg && /usr/etc/rc.d/rc.softrestart save' \
        < config

    ssh -oStrictHostKeyChecking=no ubnt@192.168.1.30

}


# firmware update

version=$(cat /usr/lib/version)
sysid=$(cat /etc/board.inc | grep 'board_id=' | cut -d" -f2)  # '$board_id="0xe009";'
    
wget -O fw.json http://www.ubnt.com/update/check.php\?sysid\=$sysid\&fwver\=$version

    response: {"url": "http://dl.ubnt.com/firmwares/XN-fw/v5.6.3/XM.v5.6.3.28591.151130.1749.bin", "checksum": "26be1e137bd1991c570a70ae7beee19f", "update": "true", "version": "v5.6.3", "date": "151130", "security": ""}

or:

    {"update": "false"}

wget -O fw.bin $(jq -r .url < fw.json)
scp fw.bin ubnt@192.168.1.30:/tmp/fwupdate.bin

ssh ubnt@192.168.1.30 /sbin/fwupdate -m


