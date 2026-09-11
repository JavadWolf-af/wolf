#!/bin/bash
echo "🚀 در حال آماده‌سازی سرور و نصب پیش‌نیازها..."
apt update
apt install mariadb-server golang-go git -y

echo "🗄️ در حال کانفیگ دیتابیس MySQL..."
mysql -e "CREATE DATABASE IF NOT EXISTS wolf_db CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;"
mysql -e "CREATE USER IF NOT EXISTS 'wolf_user'@'localhost' IDENTIFIED BY 'wolf_password';"
mysql -e "GRANT ALL PRIVILEGES ON wolf_db.* TO 'wolf_user'@'localhost';"
mysql -e "FLUSH PRIVILEGES;"

echo "📥 در حال دانلود سورس ربات از گیت‌هاب..."
systemctl stop wolfbot 2>/dev/null
rm -rf /opt/wolf
git clone https://github.com/JavadWolf-af/wolf /opt/wolf
cd /opt/wolf || exit

if [ ! -f .env ]; then
    echo "⚙️ فایل تنظیمات (.env) یافت نشد. لطفاً اطلاعات زیر را وارد کنید:"
    read -p "لطفا توکن ربات (BOT_TOKEN) را وارد کنید: " bot_token < /dev/tty
    read -p "لطفا آیدی عددی ادمین (ADMIN_ID) را وارد کنید: " admin_id < /dev/tty
    read -p "لطفا API_ID تلگرام خود را وارد کنید: " api_id < /dev/tty
    read -p "لطفا API_HASH تلگرام خود را وارد کنید: " api_hash < /dev/tty

    cat << EOF > .env
BOT_TOKEN=$bot_token
ADMIN_ID=$admin_id
API_ID=$api_id
API_HASH=$api_hash
DB_USER=wolf_user
DB_PASS=wolf_password
DB_NAME=wolf_db
EOF
    echo "✅ فایل .env با موفقیت ساخته شد."
fi

echo "🛠️ در حال ساخت دستور آپدیت خودکار (wolf-update)..."
cat << 'EOF' > /usr/local/bin/wolf-update
#!/bin/bash
cd /opt/wolf || exit
echo "🔄 در حال دریافت تغییرات از گیت‌هاب..."
git fetch --all
git reset --hard origin/main

echo "📦 در حال ساخت مجدد پکیج‌ها و دانلود پیش‌نیازها..."
export CGO_ENABLED=0
go mod tidy

echo "🔨 در حال کامپایل ربات..."
go build -o wolfbot .
systemctl restart wolfbot
echo "✅ ربات با موفقیت آپدیت و راه‌اندازی شد!"
EOF
chmod +x /usr/local/bin/wolf-update

echo "🤖 در حال ساخت سرویس 24 ساعته سیستم..."
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

echo "⚡ در حال کامپایل و اجرای اولیه ربات..."
wolf-update
