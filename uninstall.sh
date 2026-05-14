#!/data/data/com.termux/files/usr/bin/bash
# Slimr uninstaller

set -e

echo "Slimr uninstaller"
echo

# Stop running instance
killall slimr 2>/dev/null && echo "  Stopped running instance" || true

# Remove boot script
rm -f ~/.termux/boot/slimr && echo "  Removed boot script" || true

# Remove binary
rm -f ~/slimr && echo "  Removed binary ~/slimr" || true

# Remove data directory
rm -rf ~/slimr-data && echo "  Removed data ~/slimr-data/" || true

echo
echo "Slimr uninstalled. Dependencies (ffmpeg, exiftool) were kept."
echo "To remove them: pkg uninstall ffmpeg exiftool"
