#!/bin/sh
set -e

remove_logs() {
  if [ -n "${XDG_DATA_HOME:-}" ]; then
    LOG_DIR="$XDG_DATA_HOME/PrintDot/logs"
  elif [ -n "${HOME:-}" ]; then
    LOG_DIR="$HOME/.local/share/PrintDot/logs"
  else
    LOG_DIR=""
  fi

  if [ -n "$LOG_DIR" ]; then
    rm -rf "$LOG_DIR"
  fi
}

remove_logs_macos() {
  if [ -n "${HOME:-}" ]; then
    rm -rf "$HOME/Library/Application Support/PrintDot/logs"
  fi
}

case "$(uname -s)" in
  Darwin)
    remove_logs_macos
    ;;
  Linux)
    remove_logs
    ;;
  *)
    echo "Unsupported OS: $(uname -s)"
    exit 1
    ;;
esac

echo "PrintDot logs removed."
