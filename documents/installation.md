# Installation

The following outlines the instructions for installing the User Interface (UI) server on Ubuntu linux.

The UI server uses the items installed with the analysis server and provides it's own HTTP server and therefore no other software is required.

## Logging
The server logs to it's own specific lfile, which requires the following steps:

- Make a directory for the log
```
sudo mkdir /var/log/arl-ui
```
- Change the directory owner to the user that will run the server
```
sudo chown YOURUSER /var/log/arl-ui
```

## Authentication
The HTTP server uses Basic Authentication as a simple method to enforce access control.

The user configuration is controlled by the **users.config** file located within the application directory. The initial file contains the **arl** user, which requires a password to be set as the applications validation will prevent the application from running.

## Systemd Service

To enable the AutoRun UI server to run on boot up, a Systemd service must be created. The following instructions detail the steps required. The instructions show the installation into the **/opt** directory.

- Create a directory in **/opt**
```
sudo mkdir /opt/arl-ui
```
- Change the directory owner to the user we want to run the application as
```
sudo chown USERNAME /opt/arl-ui
```
- Change the directory group to the user we want to run the application as
```
sudo chgrp USERNAME /opt/arl-ui
```
- Copy the required files to the directory as listed below:
```
arl-ui.config
arl-ui-setbind.sh
arl-ui
static
templates
```
- Set the appropriate configuration options in the config file (arl.config). The configuration values are detailed in the **configuration.pdf** document.
- If the server is to be run on the lower TCP ports such as 80 or 81, then edit the **arl-ui-setbind.sh** file  with the correct paths (if they differ). The file is used to set execute options on the **arl-ui** binary and to allow it to bind to lower TCP ports. The file should be run each time the binary is changed. The files default contents are:
```
#!/bin/sh
chmod +x /opt/arl/arl-ui
sudo setcap cap_net_bind_service+ep /opt/arl/arl-ui
```
- Next define the Systemd service config by creating a new service file:
```
sudo nano /etc/systemd/system/arl-ui.service
```
- Copy the following to the service file or use the file located in the **configuration** directory:
```
[Unit]
Description=ARL-UI

[Service]
ExecStart=/opt/arl/arl -c /opt/arl-ui/arl-ui.config

[Install]
WantedBy=multi-user.target
```
- To test the service, run the following command:
```
systemctl start arl-ui.service
```
- To check the service output, run the following command:
```
systemctl status arl-ui.service
```
- If there are problems, then the service can be stopped by running the command:
```
systemctl stop arl-ui.service
```
- Once the configuration is correct and the server is running, the service can be made to run at boot, by running the following command:
```
systemctl enable arl-ui.service
```
