#!/bin/bash
set -e

echo "🚀 آماده‌سازی سرور و نصب پیش‌نیازها..."
apt update -y
apt install mariadb-server curl git -y

echo "🗄️ پیکربندی دیتابیس MySQL..."
systemctl start mariadb || systemctl start mysql || true
mysql -e "CREATE DATABASE IF NOT EXISTS wolf_db CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;"
mysql -e "CREATE USER IF NOT EXISTS 'wolf_user'@'localhost' IDENTIFIED BY 'wolf_password';"
mysql -e "GRANT ALL PRIVILEGES ON wolf_db.* TO 'wolf_user'@'localhost';"
mysql -e "FLUSH PRIVILEGES;"

echo "📥 دریافت پروژه و ایجاد پوشه‌ها..."
systemctl stop wolfbot 2>/dev/null || true
mkdir -p /opt/wolf/sessions
cd /opt/wolf || exit 1

# کلون کردن فایل‌های کم‌حجم پروژه در صورت نیاز
if [ ! -d "/opt/wolf/.git" ]; then
    git clone https://github.com/JavadWolf-af/wolf /opt/wolf
fi

if [ ! -f .env ]; then
    echo "⚙️ فایل تنظیمات (.env) یافت نشد. لطفاً اطلاعات را وارد کنید:"
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
    echo "✅ فایل .env با موفقیت ایجاد شد."
fi

echo "📦 دریافت مستقیم فایل باینری و آماده سلف‌بات از گیت‌هاب..."
curl -sSL -L -o /opt/wolf/wolfbot https://github.com/JavadWolf-af/wolf/releases/download/latest/wolfbot
chmod +x /opt/wolf/wolfbot

echo "🛠️ تنظیم اسکریپت آپدیت سریع (wolf-update)..."
cat << 'EOF' > /usr/local/bin/wolf-update
#!/bin/bash
set -e

echo " "
echo "  __          _  __   _____      _  __"
echo "  \ \        / / | |/ _| / ____|    | |/ _|"
echo "   \ \  /\  / /__ | | |_ | (___   ___| | |_ "
echo "    \ \/  \/ / _ \| |  _| \___ \ / _ \ |  _|"
echo "     \  /\  / (_) | | |   ____) |  __/ | |  "
echo "      \/  \/ \___/|_|_|  |_____/ \___|_|_|  "
echo "          >>> WOLF SELF-BOT FAST UPDATER <<<"
echo " "

cd /opt/wolf || exit 1

echo "[ 25% ] همگام‌سازی ریپازیتوری..."
git fetch --all --quiet
git reset --hard origin/main --quiet

echo "[ 60% ] دانلود فایل اجرایی جدید..."
systemctl stop wolfbot || true
curl -sSL -L -o /opt/wolf/wolfbot https://github.com/JavadWolf-af/wolf/releases/download/latest/wolfbot
chmod +x /opt/wolf/wolfbot

echo "[ 90% ] راه‌اندازی سرویس سلف‌بات..."
systemctl restart wolfbot

echo "[ 100% ] آپدیت با موفقیت کامل شد و ربات آنلاین است! 🐺🚀"
echo " "
EOF
chmod +x /usr/local/bin/wolf-update

echo "🤖 ساخت سرویس سیستمی wolfbot..."
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
systemctl restart wolfbot

echo "🎉 نصب روی سرور خام با موفقیت و در چند ثانیه به پایان رسید!"
