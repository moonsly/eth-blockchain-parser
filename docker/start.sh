#!/bin/sh
set -e

# Check if INFURA_API_KEY is set
if [ -z "$INFURA_API_KEY" ]; then
    echo "Error: INFURA_API_KEY environment variable is required"
    exit 1
fi

# Set default values
export INFURA_API_KEY=${INFURA_API_KEY:-af0eb78c2b874cb2a2df3515af9182fa}
export INFURA_NETWORK=${INFURA_NETWORK:-mainnet}
export DB_PATH=${DB_PATH:-/app/data/blockchain.db}
export CSV_PATH=${CSV_PATH:-/app/data/whale_txns.csv}
export LAST_BLOCK_PATH=${LAST_BLOCK_PATH:-/app/data/last_block.dat}
export SERVER_PORT=${SERVER_PORT:-8015}
export SERVER_HOST=${SERVER_HOST:-0.0.0.0}
export SERVER_USERNAME=${SERVER_USERNAME:-admin}
export SERVER_PASSWORD=${SERVER_PASSWORD:-password123}

# Ensure directories exist and have correct permissions
mkdir -p /app/data /var/log/eth_parser
chown -R appuser:appgroup /app/data /var/log/eth_parser
chmod 755 /app/data

# Initialize database (SQLite will create file automatically)
echo "Initializing database bash..."
touch $DB_PATH
su-exec appuser ./infura-parser -initw || echo "Database already initialized"

# Create environment file for cron
cat > /app/cron_env << EOF
export INFURA_API_KEY="$INFURA_API_KEY"
export INFURA_NETWORK="$INFURA_NETWORK"
export DB_PATH="$DB_PATH"
export CSV_PATH="$CSV_PATH"
export LAST_BLOCK_PATH="$LAST_BLOCK_PATH"
EOF
chown appuser:appgroup /app/cron_env

# Create the cron job for appuser (standard cron format: m h dom mon dow command)
su-exec appuser echo "*/2 * * * * /app/run_parser.sh" | su-exec appuser crontab -

# Start cron daemon in background
/usr/sbin/crond -f &
echo "Cron daemon started"

# Start HTTP API server in background as appuser
echo "Starting HTTP API server..."
su-exec appuser ./server-run \
    -db="$DB_PATH" \
    -port="$SERVER_PORT" \
    -host="$SERVER_HOST" \
    -username="$SERVER_USERNAME" \
    -password="$SERVER_PASSWORD" &

echo "API server started on port $SERVER_PORT"

# Create log file and set permissions
touch /var/log/eth_parser/eth_parser.log
chown appuser:appgroup /var/log/eth_parser/eth_parser.log

# Keep container running and show logs
echo "ETH Blockchain Parser started. Monitoring logs..."
echo "$(date): Container started" >> /var/log/eth_parser/eth_parser.log
tail -f /var/log/eth_parser/eth_parser.log
