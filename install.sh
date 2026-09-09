#!/bin/bash
set -e

GREEN='\033[0;32m'
CYAN='\033[0;36m'
NC='\033[0m'

echo -e "${CYAN}🐺 در حال نصب و راه‌اندازی ربات ولف سلف...${NC}"

if [ "$EUID" -ne 0 ]; then
    echo "❌ لطفاً این اسکریپت را با دسترسی root اجرا کنید (sudo bash install.sh)"
    exit 1
fi

echo -e "${CYAN}📦 در حال بررسی و نصب پیش‌نیازها...${NC}"
apt-get update -y
apt-get install -y git curl build-essential

if ! command -v go &> /dev/null; then
    echo -e "${CYAN}⚡ در حال نصب زبان Go...${NC}"
    wget https://go.dev/dl/go1.22.1.linux-amd64.tar.gz
    rm -rf /usr/local/go && tar -C /usr/local -xzf go1.22.1.linux-amd64.tar.gz
    export PATH=$PATH:/usr/local/go/bin
    echo "export PATH=\$PATH:/usr/local/go/bin" >> ~/.bashrc
    rm go1.22.1.linux-amd64.tar.gz
fi

export PATH=$PATH:/usr/local/go/bin
PROJECT_DIR=$(pwd)

if [ ! -f .env ]; then
    echo -e "${CYAN}⚙️ تنظیمات .env...${NC}"
    read -p "لطفا توکن ربات (BOT_TOKEN) را وارد کنید: " bot_token
    read -p "لطفا آیدی عددی ادمین (ADMIN_ID) را وارد کنید: " admin_id
    
    cat <<EOF > .env
BOT_TOKEN=$bot_token
ADMIN_ID=$admin_id
EOF
    echo "✅ فایل .env ساخته شد."
fi

echo -e "${CYAN}🔨 در حال کامپایل پروژه...${NC}"
go mod tidy
go build -o wolfbot .

echo -e "${CYAN}🚀 در حال ساخت سرویس همیشه آنلاین (Systemd)...${NC}"
cat <<EOF > /etc/systemd/system/wolfbot.service
[Unit]
Description=Wolf Self Bot Service
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=$PROJECT_DIR
ExecStart=$PROJECT_DIR/wolfbot
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
cd $PROJECT_DIR
git pull origin main
export PATH=\$PATH:/usr/local/go/bin
go mod tidy
go build -o wolfbot .
systemctl restart wolfbot
echo "✅ ربات با موفقیت آپدیت شد و بدون دستکاری دیتابیس مجدداً راه‌اندازی گردید!"
EOF

chmod +x /usr/local/bin/wolf-update

echo -e "${GREEN}🎉 نصب ربات ولف سلف با موفقیت انجام شد!${NC}"
echo -e "${GREEN}🔹 ربات به صورت ۲۴/۷ آنلاین شد.${NC}"
echo -e "${GREEN}🔹 جهت آپدیت ربات در آینده فقط دستور زیر را وارد کنید:${NC}"
echo -e "${CYAN}wolf-update${NC}"
