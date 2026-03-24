

<a name="readme-top"></a>
<div align="center">
    
<br>

# express-dmz-bot

<!-- SHIELD GROUP -->

<a name="readme-left"></a>
<div align="left">

Бот для нашего ДМЗ ключа который работает на express.ms

 <a href="https://github.com/oditynet/telegrambot_DMZkeyroom" >Аналог telegrambot_DMZkeyroom</a>

Возможности: отправка статуса ключа в личных сообщениях в групповом чате. 

Нюанс получения ответа от бота:

От express получаем webhook по адресу , указанному в настройках бота. У меня это на порту WEBHOOK_PORT

Перед запуском заполняем файл

#### Config

```
cat .env
EXPRESS_DOMAIN=https://express.domain.ru
BOT_ID=111-22-33-44-5555
SECRET_KEY=11111
CHAT_ID=11-22-33-44-55555
WEBHOOK_PORT=443
DB_PATH=dmzkeyroom.db
```

Создаем группу и туда добавляем бота (у нас так)


#### Build and run
```
go build -o main main.go
./main
```
#### Test

Проверяем доступ к боту:

```
curl -k https://dmz_key_bot.domain.ru:443/healt

```

Или так:

```
curl -k -X POST http://dmz_key_bot.domain.ru:443/ \
  -H "Content-Type: application/json" \
  -d '{
    "command": {
      "body": "/status",
      "data": {}
    },
    "from": {
      "user_huid": "test-user-001",
      "username": "tester",
      "name": "Test User"
    },
    "group_chat_id": "test-chat-001",
    "sync_id": "test-sync-001"
  }'
```

#### Сертификаты

Самоподписанные делаем и в  настрйоках бота снимает галки, либо подписывайте своим СА.

```
openssl req -x509 -newkey rsa:4096 -keyout key.pem -out cert.pem -days 365 -nodes -subj "/CN=dmz_key_bot.domain.ru"
```

<img src="https://github.com/oditynet/express-dmz-bot/blob/main/res1.png" title="example" width="800" />

#### Service systemd
vim /etc/systemd/system/express-bot.service
```
[Unit]
Description=Express DMZ Key Room Bot
After=network.target

[Service]
Type=simple
User=root
Group=root
WorkingDirectory=/home/user
ExecStart=/home/user/main
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
```
