
# Naming and Addressing

Access points start with `Ap`, stations start with `St`. Then an identifier for the location of the device (street names work) i.e. `Henry`. Then a number starting from `1`. Ex: `ApHenry1`, `StBrooklyn1`.

Record the device name and IP address in devices.json

# Provisioning a Station

Plug in the device and run:

    provision --name StLocation1 --ip 192.168.1.31

Note: until the tool is updated, you'll have to manually select the 
wireless mode for the -ac devices. Select `Station PTMP` (point-to-multipoint)

# Provisioning an Access Point

Plug in the device and run:

    provision --name ApLocation1 --ip 192.168.1.31

Navigate to the web interface, i.e. https://192.168.1.31 log in as ubnt/ubnt.

In the wireless tab, change "Wireless Mode" to "Access Point".

You'll have to change the password in order to apply the configuration update. Note the password in secrets.md.

