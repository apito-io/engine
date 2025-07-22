#!/bin/bash

# KILL DEBUG UTILITY
# This script kills any delve debugger processes running on port 40000

PORT=40000

echo "🔍 Looking for delve processes on port $PORT..."

# Find process using the port
PID=$(lsof -ti :$PORT 2>/dev/null)

if [ -n "$PID" ]; then
    echo "🔪 Found delve process $PID on port $PORT, killing it..."
    kill -TERM $PID 2>/dev/null
    sleep 1
    
    # Check if still running and force kill
    if kill -0 $PID 2>/dev/null; then
        echo "🔨 Force killing stubborn process..."
        kill -9 $PID 2>/dev/null
    fi
    
    echo "✅ Delve process killed successfully"
else
    echo "✅ No delve process found on port $PORT"
fi

# Also kill any other dlv processes that might be hanging around
DLV_PIDS=$(pgrep dlv 2>/dev/null)
if [ -n "$DLV_PIDS" ]; then
    echo "🧹 Found additional dlv processes: $DLV_PIDS"
    echo "🔪 Killing all dlv processes..."
    pkill -9 dlv 2>/dev/null
    echo "✅ All dlv processes cleaned up"
fi

echo "🎯 Debug cleanup complete!" 