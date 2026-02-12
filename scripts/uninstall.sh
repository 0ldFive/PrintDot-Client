#!/bin/sh
set -e

remove_data_linux() {
  if [ -n "${XDG_DATA_HOME:-}" ]; then
    BASE_DIR="$XDG_DATA_HOME/PrintDot"
  elif [ -n "${HOME:-}" ]; then
    BASE_DIR="$HOME/.local/share/PrintDot"
  else
    BASE_DIR=""
  fi

  if [ -n "$BASE_DIR" ]; then
    rm -rf "$BASE_DIR"
  fi
}

remove_data_macos() {
  if [ -n "${HOME:-}" ]; then
    rm -rf "$HOME/Library/Application Support/PrintDot"
  fi
}

case "$(uname -s)" in
  Darwin)
    remove_data_macos
    ;;
  Linux)
    remove_data_linux
    ;;
  *)
    echo "Unsupported OS: $(uname -s)"
    exit 1
    ;;
esac

echo "PrintDot data removed."
