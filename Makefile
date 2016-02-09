
ARM_BINARIES := speedcheck.arm7 firmware.arm7 provision.arm7

# The list of GPG keys to encrypt the secrets file to.
# If you are an admin of LoBuPaNet, add your key ID
# here and make sure everyone has your key.
#
# 26FD2554: Ross
SECRETS_KEYS := 26FD2554

all: $(ARM_BINARIES)

speedcheck.arm7: speedcheck/speedcheck.go
	env GOARM=7 GOOS=linux GOARCH=arm \
		go build -o speedcheck.arm7 ./speedcheck/speedcheck.go

firmware.arm7: firmware/firmware.go
	env GOARM=7 GOOS=linux GOARCH=arm \
		go build -o firmware.arm7 ./firmware/firmware.go

provision.arm7: provision/provision.go
	env GOARM=7 GOOS=linux GOARCH=arm \
		go build -o provision.arm7 ./provision/provision.go

deploy: $(ARM_BINARIES)
	scp speedcheck.arm7 lobupanet:bin/speedcheck
	scp firmware.arm7 lobupanet:bin/firmware
	scp provision.arm7 lobupanet:bin/provision

secrets:
	@gpg --decrypt < secrets.json.gpg

secrets.json.gpg: secrets.json
	gpg --encrypt -a -r $(SECRETS_KEYS) < secrets.json > secrets.json.gpg

clean:
	[ ! -e speedcheck.arm7 ] || rm speedcheck.arm7
	[ ! -e firmware.arm7 ] || rm firmware.arm7
	[ ! -e provision.arm7 ] || rm provision.arm7

.PHONY: clean deploy secrets