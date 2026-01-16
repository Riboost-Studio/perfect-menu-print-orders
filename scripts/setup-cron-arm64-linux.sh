#!/bin/bash

# Setup cron job for Perfect Menu Print Orders on Linux ARM64 (Raspberry Pi)
# Usage: ./setup-cron-arm64-linux.sh

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BINARY_NAME="perfect-menu_print_orders-arm64-linux"
BINARY_PATH="$SCRIPT_DIR/$BINARY_NAME"

# Check if binary exists
if [ ! -f "$BINARY_PATH" ]; then
    echo "Error: Binary not found at $BINARY_PATH"
    exit 1
fi

# Make binary executable if it isn't already
chmod +x "$BINARY_PATH"

echo "Perfect Menu Print Orders - Cron Setup for Linux ARM64"
echo "======================================================="
echo ""
echo "Binary location: $BINARY_PATH"
echo ""
echo "Setting up cron jobs:"
echo "  - Every 5 minutes"
echo "  - On system startup"
echo ""

# Create log directory
LOG_DIR="$SCRIPT_DIR/logs"
mkdir -p "$LOG_DIR"

# Create the cron entries
CRON_ENTRY_SCHEDULE="*/5 * * * * cd \"$SCRIPT_DIR\" && \"$BINARY_PATH\" >> \"$LOG_DIR/print-orders.log\" 2>&1"
CRON_ENTRY_REBOOT="@reboot cd \"$SCRIPT_DIR\" && \"$BINARY_PATH\" >> \"$LOG_DIR/print-orders.log\" 2>&1"

# Remove any existing cron job for this binary
(crontab -l 2>/dev/null | grep -v "$BINARY_PATH") | crontab -

# Add the new cron jobs (both schedule and on reboot)
(crontab -l 2>/dev/null; echo "$CRON_ENTRY_SCHEDULE"; echo "$CRON_ENTRY_REBOOT") | crontab -

echo ""
echo "✓ Cron jobs installed successfully!"
echo "  Schedule: Every 5 minutes (*/5 * * * *)"
echo "  On system startup: @reboot"
echo "  Logs: $LOG_DIR/print-orders.log"
echo ""
echo "To view your cron jobs, run: crontab -l"
echo "To remove these cron jobs, run: crontab -e and delete the lines"
