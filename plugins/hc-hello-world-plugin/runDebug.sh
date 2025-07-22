#!/bin/bash

# DEBUG HELPER SCRIPT
# This script helps attach delve to the running plugin process
#
# Usage: ./runDebug.sh [PID]
# If no PID provided, it will try to find the plugin process

PLUGIN_NAME="hc-hello-world-plugin"
PORT=40000

# Function to check if port is in use and kill the process
check_and_kill_port() {
    local port=$1
    echo "🔍 Checking if port $port is already in use..."
    
    # Find process using the port
    local existing_pid=$(lsof -ti :$port 2>/dev/null)
    
    if [ -n "$existing_pid" ]; then
        echo "⚠️  Port $port is already in use by process $existing_pid"
        echo "🔪 Killing existing process on port $port..."
        kill -9 $existing_pid 2>/dev/null
        
        # Wait a moment for the process to die
        sleep 1
        
        # Check if port is now free
        local check_pid=$(lsof -ti :$port 2>/dev/null)
        if [ -n "$check_pid" ]; then
            echo "❌ Failed to free port $port. Process may have restarted."
            exit 1
        else
            echo "✅ Port $port is now free"
        fi
    else
        echo "✅ Port $port is available"
    fi
}

if [ $# -eq 1 ]; then
    PID=$1
else
    # Try to find the plugin process
    PID=$(pgrep -f "$PLUGIN_NAME" | head -1)
    if [ -z "$PID" ]; then
        echo "❌ Plugin process not found. Make sure the plugin is running."
        echo "Usage: $0 [PID]"
        exit 1
    fi
fi

# Cleanup function for when script exits
cleanup() {
    echo ""
    echo "🧹 Cleaning up delve process on port $PORT..."
    local delve_pid=$(lsof -ti :$PORT 2>/dev/null)
    if [ -n "$delve_pid" ]; then
        kill -TERM $delve_pid 2>/dev/null
        sleep 1
        # Force kill if still running
        kill -9 $delve_pid 2>/dev/null
        echo "✅ Delve process cleaned up"
    fi
    exit 0
}

# Set up cleanup trap for Ctrl+C and normal exit
trap cleanup INT TERM EXIT

# Check and kill any existing delve process on port 40000
check_and_kill_port $PORT

echo "🐛 Attaching delve to plugin process PID: $PID"
echo "🔌 Delve will listen on port $PORT"
echo "📝 Use 'Attach to HashiCorp Plugin (Remote Debug)' in VSCode"
echo "❌ Press Ctrl+C to stop debugging and cleanup"

# Attach delve to the running process
dlv attach $PID --listen=:$PORT --headless=true --api-version=2 --accept-multiclient --log-dest=dlv.log 