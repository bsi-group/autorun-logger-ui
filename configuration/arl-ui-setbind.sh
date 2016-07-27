#!/bin/sh
chmod +x /opt/arl-ui/arl-ui
sudo setcap cap_net_bind_service+ep /opt/arl-ui/arl-ui
