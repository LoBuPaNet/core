Notes on configuring a the Raspberry PI that is controlling this stuff.

### initial install

- Download Raspbian Jessie Lite 2015-11-21 from https://www.raspberrypi.org/downloads/raspbian/
- Flash to SD Card per https://www.raspberrypi.org/documentation/installation/installing-images/README.md
- `sudo apt-get update && apt-get dist-upgrade`

### basic network

- observe internal IP is 10.1.10.128, available external static IP is 75.144.86.92
- configure external interface - 1:1 nat from 75.144.86.92 <-> 10.1.10.128
- TODO(ross): "hairpin" access via the my local net doesn't work. (this affects only me)
- `sudo echo "lobupanet" >> /etc/hostname`
- `sudo hostname lobupanet`

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
- make `/etc/profile.d/opscat.sh` contain `alias opscat='slackcat -token=xoxb-20537584544-otzISg9k9Um5b8gBWn7hSKXJ -channel operations'`
