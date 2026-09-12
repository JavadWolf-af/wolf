#!/bin/bash
set -e

echo "🚀 Preparing server and installing dependencies..."
apt update
apt install mariadb-server git curl wget build-essential -y

echo "🔍 Checking Go version..."
NEED_GO_INSTALL=1

if command -v go >/dev/null 2>&1; then
    CURRENT_GO_RAW=$(go version | awk '{print $3}' | sed 's/go//')
    GO_MAJOR=$(echo "$CURRENT_GO_RAW" | cut -d. -f1)
    GO_MINOR=$(echo "$CURRENT_GO_RAW" | cut -d. -f2)

    if [ "$GO_MAJOR" -ge 1 ] && [ "$GO_MINOR" -ge 21 ]; then
        echo "✅ Go version $CURRENT_GO_RAW is already installed and supported (>= 1.21)."
        NEED_GO_INSTALL=0
    else
        echo "⚠️ Outdated Go version detected ($CURRENT_GO_RAW). Upgrading..."
    fi
fi

if [ "$NEED_GO_INSTALL" -eq 1 ]; then
    echo "🧹 Removing obsolete Go versions..."
    apt remove -y golang-go gccgo-go 2>/dev/null || true
    apt autoremove -y 2>/dev/null || true
    rm -rf /usr/local/go /usr/bin/go /usr/bin/gofmt

    echo "📦 Downloading and installing latest official Go..."
    LATEST_GO=$(curl -sSL "https://go.dev/VERSION?m=text" | head -n1)
    if [ -z "$LATEST_GO" ]; then
        LATEST_GO="go1.22.5"
    fi

    wget -4 -q --show-progress "https://go.dev/dl/${LATEST_GO}.linux-amd64.tar.gz" -O /tmp/go.tar.gz
    tar -C /usr/local -xzf /tmp/go.tar.gz
    rm -f /tmp/go.tar.gz

    ln -sf /usr/local/go/bin/go /usr/bin/go
    ln -sf /usr/local/go/bin/gofmt /usr/bin/gofmt
    
    if ! grep -q "/usr/local/go/bin" ~/.bashrc; then
        echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
    fi
    export PATH=$PATH:/usr/local/go/bin
fi

# تنظیم پروکسی تحریم‌شکن سراسری Go
go env -w GOPROXY=https://goproxy.io,direct

if ! grep -q "GOPROXY" ~/.bashrc; then
    echo 'export GOPROXY=https://goproxy.io,direct' >> ~/.bashrc
fi
export GOPROXY=https://goproxy.io,direct

echo "✅ Active Go version: $(go version)"

echo "🗄️ Configuring MySQL database..."
systemctl start mariadb || systemctl start mysql || true
mysql -e "CREATE DATABASE IF NOT EXISTS wolf_db CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;"
mysql -e "CREATE USER IF NOT EXISTS 'wolf_user'@'localhost' IDENTIFIED BY 'wolf_password';"
mysql -e "GRANT ALL PRIVILEGES ON wolf_db.* TO 'wolf_user'@'localhost';"
mysql -e "FLUSH PRIVILEGES;"

echo "📥 Cloning Wolf repository..."
systemctl stop wolfbot 2>/dev/null || true
rm -rf /opt/wolf
git clone https://github.com/JavadWolf-af/wolf /opt/wolf
mkdir -p /opt/wolf/sessions
cd /opt/wolf || exit 1

if [ ! -f .env ]; then
    echo "⚙️ Configuration file (.env) not found. Please enter parameters:"
    read -p "Enter BOT_TOKEN: " bot_token < /dev/tty
    read -p "Enter ADMIN_ID: " admin_id < /dev/tty
    read -p "Enter API_ID: " api_id < /dev/tty
    read -p "Enter API_HASH: " api_hash < /dev/tty

    cat << EOF > .env
BOT_TOKEN=$bot_token
ADMIN_ID=$admin_id
API_ID=$api_id
API_HASH=$api_hash
DB_USER=wolf_user
DB_PASS=wolf_password
DB_NAME=wolf_db
EOF
    echo "✅ .env file created successfully."
fi

echo "🛠️ Creating stylish wolf-update script..."
cat << 'EOF' > /usr/local/bin/wolf-update
#!/bin/bash
set -e

# ANSI Color Codes
C_RESET='\033[0m'
C_BOLD='\033[1m'
C_RED='\033[0;31m'
C_GREEN='\033[0;32m'
C_YELLOW='\033[1;33m'
C_BLUE='\033[0;34m'
C_CYAN='\033[0;36m'
C_PURPLE='\033[0;35m'

echo -e "${C_CYAN}${C_BOLD}"
echo "  __          _  __   _____      _  __"
echo "  \ \        / / | |/ _| / ____|    | |/ _|"
echo "   \ \  /\  / /__ | | |_ | (___   ___| | |_ "
echo "    \ \/  \/ / _ \| |  _| \___ \ / _ \ |  _|"
echo "     \  /\  / (_) | | |   ____) |  __/ | |  "
echo "      \/  \/ \___/|_|_|  |_____/ \___|_|_|  "
echo -e "          ${C_YELLOW}>>> WOLF SELF-BOT UPDATE MANAGER <<<${C_RESET}\n"

cd /opt/wolf || exit 1

export PATH=$PATH:/usr/local/go/bin:/usr/bin
export GOPROXY=https://goproxy.io,direct

echo -e "${C_BLUE}[  5% ]${C_RESET} ${C_BOLD}Checking for remote updates...${C_RESET}"
git fetch --all --quiet

CURRENT_COMMIT=$(git rev-parse HEAD)
LATEST_COMMIT=$(git rev-parse origin/main)

if [ "$CURRENT_COMMIT" = "$LATEST_COMMIT" ]; then
    echo -e "${C_GREEN}[ OK ]${C_RESET} Local files are already up-to-date (${CURRENT_COMMIT:0:7})"
else
    echo -e "${C_PURPLE}[ 20% ]${C_RESET} ${C_BOLD}Changed files detected:${C_RESET}"
    DIFF_FILES=$(git diff --name-status "$CURRENT_COMMIT" "$LATEST_COMMIT")
    
    while IFS=$'\t' read -r status file; do
        case "$status" in
            M*) echo -e "       ${C_YELLOW}[MODIFIED]${C_RESET} $file" ;;
            A*) echo -e "       ${C_GREEN}[ADDED]   ${C_RESET} $file" ;;
            D*) echo -e "       ${C_RED}[DELETED] ${C_RESET} $file" ;;
            *)  echo -e "       ${C_CYAN}[CHANGED] ${C_RESET} $file" ;;
        esac
    done <<< "$DIFF_FILES"
fi

echo -e "\n${C_BLUE}[ 40% ]${C_RESET} Synchronizing repository tree..."
git reset --hard origin/main --quiet

echo -e "${C_BLUE}[ 65% ]${C_RESET} Tidying Go dependencies..."
export CGO_ENABLED=0
go mod tidy

echo -e "${C_BLUE}[ 85% ]${C_RESET} Compiling WolfBot binary..."
go build -ldflags="-s -w" -o wolfbot .

echo -e "${C_BLUE}[ 95% ]${C_RESET} Restarting background service..."
systemctl restart wolfbot

echo -e "${C_GREEN}${C_BOLD}[ 100% ] Update completed successfully! Service is active and running.${C_RESET}\n"
EOF
chmod +x /usr/local/bin/wolf-update

echo "🤖 Setting up systemd service..."
cat << 'EOF' > /etc/systemd/system/wolfbot.service
[Unit]
Description=Wolf Self Bot
After=network.target mysql.service mariadb.service

[Service]
Type=simple
User=root
WorkingDirectory=/opt/wolf
ExecStart=/opt/wolf/wolfbot
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable wolfbot

echo "⚡ Performing initial build..."
wolf-update
