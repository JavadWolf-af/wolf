#!/bin/bash
set -e

GREEN='\033[0;32m'
CYAN='\033[0;36m'
NC='\033[0m'

echo -e "${CYAN}🐺 در حال نصب و راه‌اندازی ربات ولف سلف...${NC}"

if [ "$EUID" -ne 0 ]; then
    echo "❌ لطفاً این اسکریپت را با دسترسی root اجرا کنید (sudo bash ...)"
    exit 1
fi

echo -e "${CYAN}📦 در حال بررسی و نصب پیش‌نیازها...${NC}"
apt-get update -y
apt-get install -y git curl build-essential wget software-properties-common gcc sqlite3 libsqlite3-dev

TARGET_DIR="/opt/wolf"

if [ ! -f "main.go" ]; then
    echo -e "${CYAN}📥 در حال دریافت پروژه از گیت‌هاب...${NC}"
    if [ -d "$TARGET_DIR" ]; then
        rm -rf "$TARGET_DIR"
    fi
    git clone https://github.com/JavadWolf-af/wolf.git "$TARGET_DIR"
    cd "$TARGET_DIR"
else
    TARGET_DIR=$(pwd)
fi

# نصب Go از مخازن رسمی
if ! command -v go &> /dev/null || [ "$(go version | grep -oE 'go1\.[0-9]+' | cut -d. -f2)" -lt 22 ]; then
    echo -e "${CYAN}⚡ در حال نصب آخرین نسخه Go...${NC}"
    add-apt-repository ppa:longsleep/golang-backports -y
    apt-get update -y
    apt-get install -y golang-go
fi

# 🛑 درخواست اطلاعات .env پیش از کامپایل و ران شدن ربات
if [ ! -f .env ]; then
    echo -e "${CYAN}⚙️ فایل تنظیمات .env یافت نشد. لطفاً اطلاعات زیر را وارد کنید:${NC}"
    read -p "لطفا توکن ربات (BOT_TOKEN) را وارد کنید: " bot_token
    read -p "لطفا آیدی عددی ادمین (ADMIN_ID) را وارد کنید: " admin_id
    
    cat <<EOF > .env
BOT_TOKEN=$bot_token
ADMIN_ID=$admin_id
EOF
    echo "✅ فایل .env با موفقیت ساخته شد."
else
    echo "✅ فایل .env از قبل موجود است."
fi

echo -e "${CYAN}🔨 در حال کامپایل پروژه (بسیار سریع)...${NC}"
export GOPROXY=direct
export CGO_ENABLED=1
go mod tidy
go build -o wolfbot .
chmod +x wolfbot

echo -e "${CYAN}🚀 در حال ساخت سرویس همیشه آنلاین (Systemd)...${NC}"
cat <<EOF > /etc/systemd/system/wolfbot.service
[Unit]
Description=Wolf Self Bot Service
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=$TARGET_DIR
ExecStart=$TARGET_DIR/wolfbot
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable wolfbot
systemctl restart wolfbot

echo -e "${CYAN}🛠️ در حال ساخت دستور آپدیت (wolf-update)...${NC}"
cat <<EOF > /usr/local/bin/wolf-update
#!/bin/bash
echo "🔄 در حال دریافت آخرین تغییرات از گیت‌هاب..."
cd $TARGET_DIR
git pull origin main
export GOPROXY=direct
export CGO_ENABLED=1
go mod tidy
go build -o wolfbot .
chmod +x wolfbot
systemctl restart wolfbot
echo "✅ ربات با موفقیت آپدیت شد و بدون دستکاری دیتابیس مجدداً راه‌اندازی گردید!"
EOF

chmod +x /usr/local/bin/wolf-update

echo -e "${GREEN}🎉 نصب ربات ولف سلف با موفقیت انجام شد و ربات در حال اجراست!${GREEN}"
