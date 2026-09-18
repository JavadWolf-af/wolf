cat << 'EOF' > /usr/local/bin/wolf-update
#!/bin/bash
set -e

# ANSI Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

echo -e "${CYAN}"
echo "  __          _  __   _____      _  __"
echo "  \ \        / / | |/ _| / ____|    | |/ _|"
echo "   \ \  /\  / /__ | | |_ | (___   ___| | |_ "
echo "    \ \/  \/ / _ \| |  _| \___ \ / _ \ |  _|"
echo "     \  /\  / (_) | | |   ____) |  __/ | |  "
echo "      \/  \/ \___/|_|_|  |_____/ \___|_|_|  "
echo "          >>> WOLF SELF-BOT FAST UPDATER <<<"
echo -e "${NC}"

cd /opt/wolf || exit 1

echo -e "${YELLOW}[ 25% ] Syncing repository with GitHub...${NC}"
git fetch --all --quiet
git reset --hard origin/main --quiet

echo -e "${YELLOW}[ 60% ] Downloading the latest binary...${NC}"
systemctl stop wolfbot || true
curl -sSL -L -o /opt/wolf/wolfbot https://github.com/JavadWolf-af/wolf/releases/download/latest/wolfbot
chmod +x /opt/wolf/wolfbot

echo -e "${YELLOW}[ 90% ] Starting WolfBot service...${NC}"
systemctl restart wolfbot

echo -e "${GREEN}[ 100% ] Update successfully completed! WolfBot is now online. 🐺🚀${NC}"
echo " "
EOF
chmod +x /usr/local/bin/wolf-update
