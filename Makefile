
ARM_BINARIES := check.arm7 firmware.arm7 provision.arm7

# The list of GPG keys to encrypt the secrets file to.
# If you are an admin of LoBuPaNet, add your key ID
# here and make sure everyone has your key.
#
# 26FD2554: Ross
SECRETS_KEYS := 26FD2554

all: $(ARM_BINARIES)

check.arm7: daemon/check.go
	env GOARM=7 GOOS=linux GOARCH=arm \
		go build -o check.arm7 ./daemon/check.go

firmware.arm7: firmware/firmware.go
	env GOARM=7 GOOS=linux GOARCH=arm \
		go build -o firmware.arm7 ./firmware/firmware.go

provision.arm7: provision/provision.go
	env GOARM=7 GOOS=linux GOARCH=arm \
		go build -o provision.arm7 ./provision/provision.go

deploy: $(ARM_BINARIES)
	scp check.arm7 lobupanet:bin/check
	scp firmware.arm7 lobupanet:bin/firmware
	scp provision.arm7 lobupanet:bin/provision

secrets:
	@gpg --decrypt < secrets.json.gpg

secrets.json.gpg: secrets.json
	gpg --encrypt -a -r $(SECRETS_KEYS) < secrets.json > secrets.json.gpg

clean:
	[ ! -e check.arm7 ] || rm check.arm7
	[ ! -e firmware.arm7 ] || rm firmware.arm7
	[ ! -e provision.arm7 ] || rm provision.arm7

.PHONY: clean deploy secrets