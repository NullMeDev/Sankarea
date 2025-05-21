#!/bin/bash
# run_forever.sh - Script to run Sankarea bot continuously with auto-restart

# Directory of this script
DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
cd "$DIR/.."

# Set up logging
LOG_FILE="sankarea_runner.log"
echo "$(date): Starting Sankarea runner script" >> $LOG_FILE

# Load environment variables if .env exists
if [ -f .env ]; then
    echo "Loading environment variables from .env file..."
    export $(grep -v '^#' .env | xargs)
fi

# Ensure the bot is compiled
echo "Building Sankarea bot..."
cd cmd/sankarea
go build -o sankarea
cd ../..

# Run the bot in a continuous loop
while true; do
    echo "$(date): Starting Sankarea bot..." | tee -a $LOG_FILE
    ./cmd/sankarea/sankarea
    
    EXIT_CODE=$?
    
    # Check if this was an intentional shutdown (exit code 0)
    if [ $EXIT_CODE -eq 0 ]; then
        echo "$(date): Bot was shut down normally. Exiting runner." | tee -a $LOG_FILE
        exit 0
    # Check if this was an intentional restart (exit code 42)
    elif [ $EXIT_CODE -eq 42 ]; then
        echo "$(date): Bot requested restart. Restarting immediately..." | tee -a $LOG_FILE
    # Any other exit code is treated as a crash
    else
        echo "$(date): Bot crashed with exit code $EXIT_CODE. Restarting in 5 seconds..." | tee -a $LOG_FILE
        sleep 5
    fi
done
