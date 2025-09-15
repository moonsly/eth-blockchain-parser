#!/bin/sh

# Load environment variables from file if it exists
if [ -f /app/cron_env ]; then
    . /app/cron_env
fi

# Change to app directory
cd /app

# Create log file if it doesn't exist
mkdir -p /var/log/eth_parser
touch /var/log/eth_parser/eth_parser.log

# Run the parser with logging
echo "$(date): Starting eth parser..." >> /var/log/eth_parser/eth_parser.log
./infura-parser 2>&1 >> /var/log/eth_parser/eth_parser.log
