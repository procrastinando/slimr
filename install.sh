#!/data/data/com.termux/files/usr/bin/bash
# Slimr installer — one-line: curl -L https://raw.githubusercontent.com/procrastinando/slimr/master/install.sh | bash

set -e

BOLD="\033[1m"
GREEN="\033[32m"
NC="\033[0m"

echo -e "${BOLD}Slimr installer${NC}"
echo

# ---- dependencies ----
echo "Installing dependencies..."
pkg update -y -qq
pkg install -y -qq ffmpeg exiftool
echo "  ffmpeg  : $(ffmpeg -version 2>&1 | head -1)"
echo "  exiftool: $(exiftool -ver)"
echo

# ---- data directory ----
mkdir -p ~/slimr-data

# ---- download binary ----
URL="https://github.com/procrastinando/slimr/releases/latest/download/slimr_arm64"
echo "Downloading slimr..."
curl -L --progress-bar -o ~/slimr "$URL"
chmod +x ~/slimr
echo "  binary: ~/slimr"
echo

# ---- Termux:Boot auto-start ----
if [ -d ~/.termux/boot ]; then
    cat > ~/.termux/boot/slimr << 'BOOTSCRIPT'
#!/data/data/com.termux/files/usr/bin/bash
termux-wake-lock
cd $(dirname $0)/..
nohup ./slimr > ~/slimr-data/slimr.log 2>&1 &
BOOTSCRIPT
    chmod +x ~/.termux/boot/slimr
    echo "Auto-start: enabled (Termux:Boot detected)"
else
    echo "Auto-start: install Termux:Boot from F-Droid to enable"
fi
echo

echo -e "${GREEN}Slimr installed.${NC}"
echo
echo "  Start:  ~/slimr"
echo "  Open:   http://127.0.0.1:8880"
echo "  Data:   ~/slimr-data/"
echo "  Logs:   ~/slimr-data/slimr.log"
echo "  Config: ~/slimr-data/config.json"
