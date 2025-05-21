# Sankarea Extreme

This repo hosts the "extreme" Sankarea Discord bot with production-grade features:
- Environment-based config (`.env`)
- Docker + Docker Compose
- PostgreSQL persistence + migrations (`migrations/`)
- Prometheus metrics & webhook listener
- Legacy update scripts merged (`Sankareaupdate.sh`, `update.sh`)
- Terraform & Ansible for IaC
- Nginx reverse proxy with Let's Encrypt
- Systemd services & timers
- Logrotate & Fail2Ban config
- Cron-based backups to S3
- Canary deploy strategy with Docker Compose
