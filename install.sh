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
apt-get install -y git curl build-essential

# مسیر استاندارد نصب پروژه روی سرور
TARGET_DIR="/opt/wolf"

# بررسی دانلود پروژه از گیت‌هاب در صورت اجرا با دستور یک‌خطی
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

# نصب مطمئن و هوشمند زبان Go
if ! command -v go &> /dev/null; then
    echo -e "${CYAN}⚡ در حال نصب زبان Go...${NC}"
    
    # تلاش اول: نصب از طریق apt (سریع و بدون خطای لینک)
    if apt-get install -y golang-go &>/dev/null; then
        echo -e "${GREEN}✅ زبان Go از مخازن سیستم‌عامل نصب شد.${NC}"
    else
        # تلاش دوم: دریافت مستقیم تاربال رسمی
        GO_TAR=$(curl -s https://go.dev/dl/?mode=json | grep -o 'go[0-9.]*\.linux-amd64\.tar\.gz' | head -n 1)
        if [ -n "$GO_TAR" ]; then
            wget "https://go.dev/dl/${GO_TAR}" -O go.tar.gz
            rm -rf /usr/local/go && tar -C /usr/local -xzf go.tar.gz
            export PATH=$PATH:/usr/local/go/bin
            echo "export PATH=\$PATH:/usr/local/go/bin" >> ~/.bashrc
            rm go.tar.gz
        fi
    fi
fi

export PATH=$PATH:/usr/local/go/bin

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
export PATH=\$PATH:/usr/local/go/bin
go mod tidy
go build -o wolfbot .
systemctl restart wolfbot
echo "✅ ربات با موفقیت آپدیت شد و بدون دستکاری دیتابیس مجدداً راه‌اندازی گردید!"
EOF

chmod +x /usr/local/bin/wolf-update

echo -e "${GREEN}🎉 نصب ربات ولف سلف با موفقیت انجام شد!${NC}"
echo -e "${GREEN}🔹 ربات به صورت ۲۴/۷ آنلاین شد.${NC}"
echo -e "${GREEN}🔹 جهت آپدیت ربات در آینده فقط دستور زیر را در سرور وارد کنید:${NC}"
echo -e "${CYAN}wolf-update${NC}"
