#!/bin/bash
# Source this file to add Sankarea bot aliases

# Directory of this script
DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

# Define aliases
alias start-sankarea="$DIR/scripts/start_sankarea.sh"
alias run-sankarea="$DIR/scripts/run_forever.sh"
alias test-sankarea="$DIR/scripts/test_sankarea.sh"
alias status-sankarea="systemctl status sankarea 2>/dev/null || echo 'Sankarea is not running as a service'"
alias logs-sankarea="tail -f $DIR/logs/sankarea_*.log | grep -v 'PING'"

# Print available commands
echo "Sankarea bot aliases available:"
echo "  start-sankarea  - Start the bot in foreground mode"
echo "  run-sankarea   - Run the bot with auto-restart"
echo "  test-sankarea  - Run tests on the bot"
echo "  status-sankarea - Check bot service status"
echo "  logs-sankarea  - View bot logs"
