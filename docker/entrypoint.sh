#!/bin/sh
# PicoClaw Docker Entrypoint
# Handles initialization and runs the specified command

set -e

# Default paths
export PICOCLAW_CONFIG=${PICOCLAW_CONFIG:-/config/config.json}
export PICOCLAW_HOME=${PICOCLAW_HOME:-/data}

# Ensure directories exist
mkdir -p "$(dirname "$PICOCLAW_CONFIG")" "$PICOCLAW_HOME"

# Initialize empty config if it doesn't exist
if [ ! -f "$PICOCLAW_CONFIG" ]; then
    echo "Creating default config at $PICOCLAW_CONFIG"
    echo '{}' > "$PICOCLAW_CONFIG"
fi

# Log configuration
echo "PicoClaw Configuration:"
echo "  Config: $PICOCLAW_CONFIG"
echo "  Home:   $PICOCLAW_HOME"
echo "  User:   $(id)"
echo "  Args:   $@"

# Execute the command
exec "$@"
