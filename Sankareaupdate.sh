#!/usr/bin/env bash
set -euxo pipefail

# 1) cd into this repo
cd "$(dirname "$0")"

# 2) pull down your latest commits
git pull origin main

# 3) rebuild if needed (Go example)
go build -o sankarea ./cmd/sankarea

# 4) restart the systemd service so it picks up the new binary
systemctl restart sankarea.service