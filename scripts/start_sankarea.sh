#!/bin/bash
# start_sankarea.sh - Script to start the Sankarea Discord bot

# Set script to exit on error
set -e

# Directory of this script
DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
cd "$DIR/.."

# Load environment variables if .env exists
if [ -f .env ]; then
    echo "Loading environment variables from .env file..."
    export $(grep -v '^#' .env | xargs)
fi

# Check if the bot is already compiled
if [ ! -f ./cmd/sankarea/sankarea ]; then
    echo "Building Sankarea bot..."
    cd cmd/sankarea
    go build -o sankarea
    cd ../..
fi

# Start the bot
echo "Starting Sankarea bot..."
./cmd/sankarea/sankarea

# Check exit status
if [ $? -ne 0 ]; then
    echo "Bot exited with an error. Check logs for details."
    exit 1
fi
