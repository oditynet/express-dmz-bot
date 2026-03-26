<a name="readme-top"></a>
<div align="center">
    
<br>

# express-dmz-bot

** Уникальный в своем роде продукт, т.к. мало кто пишет их еще да еще осили TLS
<!-- SHIELD GROUP -->

<a name="readme-left"></a>
<div align="left">

Бот для нашего ДМЗ ключа который работает на express.ms по TLS

 <a href="https://github.com/oditynet/telegrambot_DMZkeyroom" >Аналог telegrambot_DMZkeyroom</a>

#### Возможности: 
 - Отправка статуса ключа в личных сообщениях в групповом чате (Вы не увидите кто нажимает кнопки)
 - Бот видит кто добавился в чат и отсылает ему лично кнопки
 - Бот ведет историю ключа
 - Бот отсылает уведомление о 152 ФЗ и ведет в базе кто согласился
 - Бот следит за сроком действия сертификата и за 14 дней до истечения отсылает всем админам уведомление раз в день
   

Нюанс получения ответа от бота:

От express получаем webhook по адресу , указанному в настройках бота. У меня это на порту WEBHOOK_PORT

 <a href="https://docs.express.ms/smartapps/developer-guide/development-debugging/" >Документация backend и frontend читаем</a>

#### Полезные ссылки для разработки

Техподдержка express отсутствует, разработчики молчат. Документация их малоинформативная. Радуемся что есть
```
https://nc.express.ms/s/c58XWQB7GyzQdqj
https://docs.express.ms/chatbots/developer-guide/api/botx-api/notifications-api/
https://docs.express.ms/chatbots/developer-guide/development-and-debugging/examples/
https://docs.express.ms/smartapps/developer-guide/smartapp-api/#%D0%BF%D0%BE%D0%BB%D1%83%D1%87%D0%B5%D0%BD%D0%B8%D0%B5-%D1%81%D0%BF%D0%B8%D1%81%D0%BA%D0%B0-smartapp-%D0%BD%D0%B0-cts
https://docs.express.ms/chatbots/developer-guide/api/botx-api/
```

### Профит которого нет в интернете (СА сертификаты)

Мы долго бились, чтоб подружить express сервер c ботом по 443 порту доверял сертификатам. Рабочий вариант: Ваш СА центр заверяет сертификат бота и на сервере express СА закидывать надо не в docker, ни в /etc, а через GUI. В дебаге нам поможет opessl утилита и curl.
<a href="https://github.com/oditynet/express-dmz-bot/blob/main/res3.png/" >Фото места в GUI</a>

#### Config

Перед запуском заполняем файл
```
cat .env
EXPRESS_DOMAIN=https://express.domain.ru
BOT_ID=111-22-33-44-5555
SECRET_KEY=11111
CHAT_ID=11-22-33-44-55555
WEBHOOK_PORT=443
DB_PATH=dmzkeyroom.db
CERT_FILE=...
KEY_FILE=...
```

Создаем группу и туда добавляем бота админом (у нас так)

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


<img src="https://github.com/oditynet/express-dmz-bot/blob/main/res1.png" title="example" width="800" />
<img src="https://github.com/oditynet/express-dmz-bot/blob/main/res2.png" title="example" width="800" />
<img src="https://github.com/oditynet/express-dmz-bot/blob/main/res3.png" title="example" width="800" />
