Notes on configuring a the Raspberry PI that is controlling this stuff.

### initial install

- Download Raspbian Jessie Lite 2015-11-21 from https://www.raspberrypi.org/downloads/raspbian/
- Flash to SD Card per https://www.raspberrypi.org/documentation/installation/installing-images/README.md
- `sudo apt-get update && apt-get dist-upgrade`
- `sudo raspi-config` -> expand file system
- `sudo apt-get install git vim tcpdump`

### basic network

- observe internal IP is 10.1.10.128, available external static IP is 75.144.86.92
- configure external interface - 1:1 nat from 75.144.86.92 <-> 10.1.10.128
- TODO(ross): "hairpin" access via the my local net doesn't work. (this affects only me)
- `sudo echo "lobupanet" >> /etc/hostname`
- `sudo hostname lobupanet`
- `/etc/ssh/sshd_config` -> `PasswordAuthentication no`

- `/etc/iptables.up.rules` (nothing but tcp/22):
   
        *filter
        -A INPUT -i lo -j ACCEPT
        -A INPUT ! -i lo -d 127.0.0.0/8 -j REJECT
        -A INPUT -m state --state ESTABLISHED,RELATED -j ACCEPT
        -A INPUT -p tcp -m state --state NEW --dport 22 -j ACCEPT
        -A INPUT -p icmp -m icmp --icmp-type 8 -j ACCEPT
        -A INPUT -j REJECT
        -A FORWARD -j REJECT
        COMMIT

- `/etc/network/if-pre-up.d/iptables` (chmod +x)

        #!/bin/sh
        /sbin/iptables-restore < /etc/iptables.up.rules

### security updates

[https://wiki.debian.org/UnattendedUpgrades](ref)

        apt-get install unattended-upgrades

In ``/etc/apt/apt.conf.d/50unattended-upgrades`` uncomment 

        Unattended-Upgrade::Origins-Pattern {
                // Codename based matching:
                // This will follow the migration of a release through different
                // archives (e.g. from testing to stable and later oldstable).
                "o=Raspbian,n=jessie";
        }

Make ` /etc/apt/apt.conf.d/20auto-upgrades` look like:

        APT::Periodic::Update-Package-Lists "1";
        APT::Periodic::Unattended-Upgrade "1";

Make `/etc/apt/apt.conf.d/02periodic` look like:

        APT::Periodic::Enable "1";
        APT::Periodic::Update-Package-Lists "1";
        APT::Periodic::Download-Upgradeable-Packages "1";
        APT::Periodic::Unattended-Upgrade "1";
        APT::Periodic::AutocleanInterval "21";
        APT::Periodic::Verbose "2";

### users

- setup sudo to allow nopasswd sudo to root:

        %sudo ALL=(ALL) NOPASSWD: ALL

- to add a new user

        adduser --disabled-password $N
        usermod -G sudo $N
        mkdir $N/.ssh
        echo '' > ~$N/.ssh/authorized_keys
        chown -R $N ~$N/.ssh

- once at least one non-root user is working, disable the pi account:  `rmuser pi`

### slackcat / opsbot

To keep everyone up to speed on what is going on, it is handy to be able to emit stuff to slack
from time to time. `slackcat` is a nice too for that, and can be used in a variety of ways and in
other scripts.

- build slackcat for arm

        cd ~/go/src/github.com/crewjam/slackcat
        env GOARM=7 GOOS=linux GOARCH=arm go build -v -o slackcat.linux.arm7 .
        scp slackcat.linux.arm7 $IP:slackcat
        ssh pi@$IP sudo cp slackcat /usr/bin/slackcat

- navigate to lobupa.slack.com -> custom integrations -> bots and created a new bot called `opsbot`. Make a note of the token
- created a channel called `#operations` and invited `opsbot`
- on rpi: ``echo "Hello, World!" | slackcat -token $TOKEN -channel operations``
- make `/etc/profile.d/opscat.sh` contain `alias opscat='slackcat -token=$SLACK_TOKEN -channel operations'`

Logs to slack (`/etc/systemd/system/journal2slack.service`):

        [Unit]
        Description=forward journal to slack
        After=network.target

        [Service]
        Environment=TOKEN=xoxb-REDACTED
        Environment=CHANNEL=logs-lobupanet
        ExecStart=/bin/sh -c '/bin/journalctl -f | /usr/bin/slackcat -token ${TOKEN} -channel ${CHANNEL} -write'
        Restart=on-failure

        [Install]
        WantedBy=multi-user.target

Install & start:

        sudo systemctl enable /etc/systemd/system/journal2slack.service
        sudo systemctl start journal2slack.service


### golang

    # ref http://dave.cheney.net/unofficial-arm-tarballs
    wget http://dave.cheney.net/paste/go1.5.3.linux-arm.tar.gz
    sudo tar -xvz -C /usr/local -f go1.5.3.linux-arm.tar.gz
    echo 'export PATH=$PATH:/usr/local/go/bin' > /etc/profile.d/golang.sh

set up personal ~/go dir:

        mkdir $HOME/go
        export GOPATH=$HOME/go
        export PATH=$PATH:$GOPATH/bin

### influxdb

    wget https://s3.amazonaws.com/influxdb/influxdb-0.10.0-1_linux_arm.tar.gz
    sudo tar -C / --strip-components=2 -vxzf influxdb-0.10.0-1_linux_arm.tar.gz

    sudo groupadd --system influxdb
    sudo adduser --system influxdb
    sudo usermod -g influxdb influxdb
    sudo chown -R influxdb:influxdb /var/log/influxdb /var/lib/influxdb

    sudo systemctl enable /usr/lib/influxdb/scripts/influxdb.service
    sudo systemctl start influxdb.service

Grafana:

[http://docs.grafana.org/project/building_from_source/](ref)

build the golang binary from source:

        mkdir -p $GOPATH/src/github.com/grafana
        cd $GOPATH/src/github.com/grafana
        wget https://github.com/grafana/grafana/archive/v2.6.0.tar.gz
        tar vxzf v2.6.0.tar.gz
        mv grafana-2.6.0 grafana
        cd grafana
        go run build.go setup
        godep restore
        go run build.go build

but the node/npm frontend is too hard to build, so just fetch the AMD64 binary 
distribution and us it, replacing the AMD64 binaries with ARM ones:

        wget https://grafanarel.s3.amazonaws.com/builds/grafana-2.6.0.linux-x64.tar.gz
        sudo tar -C /opt -zxf grafana-2.6.0.linux-x64.tar.gz
        sudo cp bin/grafana-server{,.md5} /opt/grafana-2.6.0/bin
        wget https://github.com/piksel/phantomjs-raspberrypi/raw/master/bin/phantomjs
        chmod +x phantomjs
        sudo cp phantomjs /opt/grafana-2.6.0/vendor/phantomjs/phantomjs

Here is a systemd service file for grafana:

        [Unit]
        Description=Graphana
        After=network.target

        [Service]
        User=grafana
        Group=grafana
        LimitNOFILE=65536
        ExecStart=/opt/grafana-2.6.0/bin/grafana-server -homepath /opt/grafana-2.6.0
        Restart=on-failure

        [Install]
        WantedBy=multi-user.target

Install & start:

        sudo groupadd --system grafana
        sudo adduser --system grafana
        sudo usermod -g grafana grafana
        sudo chown -R grafana:grafana /opt/grafana-2.6.0/data
        sudo systemctl enable /opt/grafana-2.6.0/grafana.service
        sudo systemctl start grafana.service

