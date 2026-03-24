//version 0.1 alpha
package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3"
)

// ============= КОНФИГУРАЦИЯ =============
type Config struct {
	ExpressDomain string
	BotID         string
	SecretKey     string
	ChatID        string
	WebhookPort   string
	DBPath        string
}

var config Config
var db *sql.DB

// ============= СТРУКТУРЫ =============

type Button struct {
	Command string                 `json:"command"`
	Label   string                 `json:"label"`
	Data    map[string]interface{} `json:"data,omitempty"`
	HSize   int                    `json:"h_size,omitempty"` // 1-12, где 12 = вся ширина
}

type Notification struct {
	Status           string     `json:"status"`
	Body             string     `json:"body"`
	Bubble           [][]Button `json:"bubble,omitempty"`
	ButtonsAutoAdjust bool      `json:"buttons_auto_adjust,omitempty"`
}

type SendRequest struct {
	GroupChatID  string       `json:"group_chat_id"`
	Recipients   []string     `json:"recipients,omitempty"`
	Notification Notification `json:"notification"`
}

type UserInfo struct {
	UserHUID string   `json:"user_huid"`
	Name     string   `json:"name"`
	AdLogin  string   `json:"ad_login"`
	AdDomain string   `json:"ad_domain"`
	Emails   []string `json:"emails"`
	Active   bool     `json:"active"`
}

type ChatMember struct {
	HUID  string `json:"huid"`
	Name  string `json:"name"`
	Admin bool   `json:"admin"`
	Owner bool   `json:"owner"`
}

type ChatInfo struct {
	GroupChatID string       `json:"group_chat_id"`
	Name        string       `json:"name"`
	Members     []ChatMember `json:"members"`
}

type WebhookRequest struct {
	Command struct {
		Body string                 `json:"body"`
		Data map[string]interface{} `json:"data"`
	} `json:"command"`
	From struct {
		UserHUID string `json:"user_huid"`
		Username string `json:"username"`
		Name     string `json:"name"`
	} `json:"from"`
	GroupChatID string `json:"group_chat_id"`
	SyncID      string `json:"sync_id"`
}

// ============= ТОКЕН =============
type TokenManager struct {
	token     string
	expiresAt time.Time
	mu        sync.RWMutex
}

var tokenManager = &TokenManager{}

func generateSignature(botID, secretKey string) string {
	h := hmac.New(sha256.New, []byte(secretKey))
	h.Write([]byte(botID))
	return strings.ToUpper(hex.EncodeToString(h.Sum(nil)))
}

func fetchNewToken() (string, error) {
	signature := generateSignature(config.BotID, config.SecretKey)
	url := fmt.Sprintf("%s/api/v2/botx/bots/%s/token?signature=%s",
		config.ExpressDomain, config.BotID, signature)

	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		Status string `json:"status"`
		Result string `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if result.Status != "ok" {
		return "", fmt.Errorf("failed to get token: %s", result.Status)
	}

	return result.Result, nil
}

func GetToken() (string, error) {
	tokenManager.mu.RLock()
	if tokenManager.token != "" && time.Now().Before(tokenManager.expiresAt) {
		token := tokenManager.token
		tokenManager.mu.RUnlock()
		return token, nil
	}
	tokenManager.mu.RUnlock()

	tokenManager.mu.Lock()
	defer tokenManager.mu.Unlock()

	if tokenManager.token != "" && time.Now().Before(tokenManager.expiresAt) {
		return tokenManager.token, nil
	}

	log.Println("🔄 Обновление токена...")
	newToken, err := fetchNewToken()
	if err != nil {
		return "", err
	}

	tokenManager.token = newToken
	tokenManager.expiresAt = time.Now().Add(14 * time.Minute)
	log.Printf("✅ Токен обновлен! Действует до: %s", tokenManager.expiresAt.Format("15:04:05"))
	return newToken, nil
}

func startTokenRefresher() {
	ticker := time.NewTicker(14 * time.Minute)
	go func() {
		for range ticker.C {
			log.Println("🔄 Плановое обновление токена...")
			newToken, err := fetchNewToken()
			if err != nil {
				log.Printf("❌ Ошибка обновления: %v", err)
				continue
			}
			tokenManager.mu.Lock()
			tokenManager.token = newToken
			tokenManager.expiresAt = time.Now().Add(14 * time.Minute)
			tokenManager.mu.Unlock()
			log.Println("✅ Токен обновлен")
		}
	}()
}

// ============= БАЗА ДАННЫХ =============

func initDB() error {
	var err error
	db, err = sql.Open("sqlite3", config.DBPath)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS key_status (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			status INTEGER NOT NULL,
			datechange TIMESTAMP NOT NULL
		)
	`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS history (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			timestamp TIMESTAMP NOT NULL,
			status INTEGER NOT NULL,
			user_huid TEXT NOT NULL,
			user_name TEXT NOT NULL,
			phone TEXT,
			message TEXT NOT NULL
		)
	`)
	if err != nil {
		return err
	}

	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM key_status").Scan(&count)
	if err != nil {
		return err
	}
	if count == 0 {
		_, err = db.Exec(`
			INSERT INTO key_status (id, status, datechange) 
			VALUES (1, 0, datetime('now'))
		`)
		if err != nil {
			return err
		}
		log.Println("✅ Создана запись статуса по умолчанию (status=0)")
	}

	log.Println("✅ База данных инициализирована")
	return nil
}

func GetStatus() (int, time.Time) {
	var status int
	var datechangeStr string
	err := db.QueryRow("SELECT status, datechange FROM key_status WHERE id = 1").Scan(&status, &datechangeStr)
	if err != nil {
		return 0, time.Now()
	}
	datechange, _ := time.Parse("2006-01-02 15:04:05", datechangeStr)
	return status, datechange
}

func SetStatus(status int, userHUID, userName, phone string) {
	now := time.Now().Format("2006-01-02 15:04:05")
	_, err := db.Exec("UPDATE key_status SET status = ?, datechange = ? WHERE id = 1", status, now)
	if err != nil {
		log.Printf("Ошибка обновления статуса: %v", err)
	}
}

func AddHistory(status int, userHUID, userName, phone, message string) {
	now := time.Now().Format("2006-01-02 15:04:05")
	_, err := db.Exec(`
		INSERT INTO history (timestamp, status, user_huid, user_name, phone, message)
		VALUES (?, ?, ?, ?, ?, ?)
	`, now, status, userHUID, userName, phone, message)
	if err != nil {
		log.Printf("Ошибка добавления истории: %v", err)
	}

	_, err = db.Exec(`
		DELETE FROM history WHERE id NOT IN (
			SELECT id FROM history ORDER BY timestamp DESC LIMIT 8
		)
	`)
	if err != nil {
		log.Printf("Ошибка очистки истории: %v", err)
	}
}

func GetHistory() ([]map[string]string, error) {
	rows, err := db.Query(`
		SELECT timestamp, status, user_name, phone, message 
		FROM history 
		ORDER BY timestamp DESC 
		LIMIT 8
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var history []map[string]string
	for rows.Next() {
		var timestamp, userName, phone, message string
		var status int
		err := rows.Scan(&timestamp, &status, &userName, &phone, &message)
		if err != nil {
			continue
		}
		history = append(history, map[string]string{
			"timestamp": timestamp,
			"status":    fmt.Sprintf("%d", status),
			"user_name": userName,
			"phone":     phone,
			"message":   message,
		})
	}
	return history, nil
}

// ============= ОТПРАВКА СООБЩЕНИЙ =============

func sendRequest(token string, payload SendRequest) error {
	jsonData, _ := json.Marshal(payload)
	url := fmt.Sprintf("%s/api/v4/botx/notifications/direct", config.ExpressDomain)
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if status, ok := result["status"]; ok && status == "ok" {
		return nil
	}
	return fmt.Errorf("send failed: %v", result)
}

func SendToUser(chatID, userHUID, text string) error {
	token, err := GetToken()
	if err != nil {
		return err
	}

	payload := SendRequest{
		GroupChatID: chatID,
		Recipients:  []string{userHUID},
		Notification: Notification{
			Status: "ok",
			Body:   text,
		},
	}

	return sendRequest(token, payload)
}

func SendButtons(chatID string) error {
    token, err := GetToken()
    if err != nil {
	return err
    }

    buttons := [][]Button{
	{
	    {Command: "0", Label: "0", HSize: 3},
	    {Command: "1", Label: "1", HSize: 3},
	    {Command: "2", Label: "2", HSize: 3},
	    {Command: "3", Label: "3", HSize: 3},
	},
	{
	    {Command: "/status", Label: "Статус", HSize: 4},
	    {Command: "/history", Label: "История", HSize: 4},
	    {Command: "/help", Label: "Помощь", HSize: 4},
	},
    }

    payload := SendRequest{
	GroupChatID: chatID,
	Notification: Notification{
	    Status:           "ok",
	    Body:             "DMZ Key Room - управление ключом",
	    Bubble:           buttons,
	    ButtonsAutoAdjust: true,
	},
    }

    return sendRequest(token, payload)
}


func DeleteMessage(chatID, syncID string) error {
	token, err := GetToken()
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/api/v4/botx/chats/%s/messages/%s",
		config.ExpressDomain, chatID, syncID)

	req, _ := http.NewRequest("DELETE", url, nil)
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

func AddUserToGroup(chatID, userHUID string) error {
	token, err := GetToken()
	if err != nil {
		return err
	}

	payload := map[string]interface{}{
		"group_chat_id": chatID,
		"users":         []string{userHUID},
	}

	jsonData, _ := json.Marshal(payload)
	url := fmt.Sprintf("%s/api/v4/botx/chats/add_users", config.ExpressDomain)
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

func IsUserInGroup(chatID, userHUID string) (bool, error) {
	chatInfo, err := GetChatInfo(chatID)
	if err != nil {
		return false, err
	}

	for _, member := range chatInfo.Members {
		if member.HUID == userHUID {
			return true, nil
		}
	}
	return false, nil
}

// ============= ПОЛУЧЕНИЕ ИНФОРМАЦИИ =============

func GetChatInfo(chatID string) (*ChatInfo, error) {
	token, err := GetToken()
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/api/v3/botx/chats/%s", config.ExpressDomain, chatID)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Status string   `json:"status"`
		Result ChatInfo `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if result.Status != "ok" {
		return nil, fmt.Errorf("failed to get chat info")
	}

	return &result.Result, nil
}

func IsChatAdmin(chatID, userHUID string) (bool, error) {
	chatInfo, err := GetChatInfo(chatID)
	if err != nil {
		return false, err
	}

	for _, member := range chatInfo.Members {
		if member.HUID == userHUID {
			return member.Admin || member.Owner, nil
		}
	}
	return false, nil
}

func GetUserInfo(userHUID string) (*UserInfo, error) {
	token, err := GetToken()
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/api/v3/botx/users/by_huid?user_huid=%s", config.ExpressDomain, userHUID)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Status string   `json:"status"`
		Result UserInfo `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if result.Status != "ok" {
		return nil, fmt.Errorf("user not found")
	}

	return &result.Result, nil
}

func SearchUserByName(fullName string) ([]UserInfo, error) {
	token, err := GetToken()
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/api/v3/botx/users/users_as_csv?cts_user=true&unregistered=false",
		config.ExpressDomain)

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	reader := csv.NewReader(resp.Body)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	var users []UserInfo
	var foundUsers []UserInfo

	for i, record := range records {
		if i == 0 {
			continue
		}
		if len(record) >= 5 {
			user := UserInfo{
				UserHUID: record[0],
				Name:     record[4],
				AdLogin:  record[1],
				AdDomain: record[2],
				Active:   len(record) > 6 && record[6] == "true",
			}
			users = append(users, user)
		}
	}

	searchName := normalizeName(fullName)
	for _, user := range users {
		if normalizeName(user.Name) == searchName {
			foundUsers = append(foundUsers, user)
		}
	}

	return foundUsers, nil
}

func normalizeName(name string) string {
	name = strings.Join(strings.Fields(name), " ")
	return strings.ToLower(name)
}

// ============= ОБРАБОТЧИКИ =============

func webhookHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var data WebhookRequest
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		log.Printf("❌ Ошибка: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"status": "error"})
		return
	}

	command := data.Command.Body
	userHUID := data.From.UserHUID
	userName := data.From.Username
	if userName == "" {
		userName = data.From.Name
	}
	chatID := data.GroupChatID
	syncID := data.SyncID

	log.Printf("Команда: %s от %s (%s)", command, userName, userHUID)

	// Проверяем, является ли пользователь администратором чата
	isAdmin, err := IsChatAdmin(chatID, userHUID)
	if err != nil {
		log.Printf("⚠️ Ошибка проверки прав: %v", err)
		isAdmin = false
	}

	// Обработка кнопок 0,1,2,3 (статус)
	statusMap := map[string]int{
		"0": 0, "1": 1, "2": 2, "3": 3,
	}
	if status, ok := statusMap[command]; ok {
		userInfo, _ := GetUserInfo(userHUID)
		phone := ""
		if userInfo != nil && len(userInfo.Emails) > 0 {
			phone = userInfo.Emails[0]
		}

		SetStatus(status, userHUID, userName, phone)
		now := time.Now().Format("02/01/2006 15:04:05")

		var msg string
		switch status {
		case 0:
			msg = fmt.Sprintf("🔑 *Статус: 0 - Ключ на ресепшене, ДЗ на охране*\n\nУстановил: %s\nВремя: %s", userName, now)
			AddHistory(0, userHUID, userName, phone, fmt.Sprintf("%s: Ключ на ресепшене, ДЗ на охране. Установил %s", now, userName))
		case 1:
			msg = fmt.Sprintf("🔓 *Статус: 1 - Ключ на ресепшене, ДЗ не на охране*\n\nИнициатор: %s\nТелефон: %s\nВремя: %s", userName, phone, now)
			AddHistory(1, userHUID, userName, phone, fmt.Sprintf("%s: Ключ на ресепшене, ДЗ не на охране. Установил %s по телефону: %s", now, userName, phone))
		case 2:
			msg = fmt.Sprintf("🔒 *Статус: 2 - ДЗ закрыт на ключ*\n\nКлюч у: %s\nТелефон: %s\nВремя: %s", userName, phone, now)
			AddHistory(2, userHUID, userName, phone, fmt.Sprintf("%s: ДЗ закрыт на ключ, ключ у %s. Телефон: %s", now, userName, phone))
		case 3:
			msg = fmt.Sprintf("🚪 *Статус: 3 - ДЗ открыт, ключ у меня*\n\nКлюч у: %s\nТелефон: %s\nВремя: %s", userName, phone, now)
			AddHistory(3, userHUID, userName, phone, fmt.Sprintf("%s: ДЗ открыт, ключ у %s. Телефон: %s", now, userName, phone))
		}
		SendToUser(chatID, userHUID, msg)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		return
	}

	// Команды для администраторов чата
	if isAdmin {
		// /add ФИО
		if strings.HasPrefix(command, "/add ") {
			fullName := strings.TrimPrefix(command, "/add ")
			fullName = strings.TrimSpace(fullName)

			if fullName == "" {
				SendToUser(chatID, userHUID, "Формат: /add <Фамилия Имя Отчество>")
				DeleteMessage(chatID, syncID)
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
				return
			}

			users, err := SearchUserByName(fullName)
			if err != nil {
				SendToUser(chatID, userHUID, fmt.Sprintf("Ошибка поиска: %v", err))
				DeleteMessage(chatID, syncID)
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
				return
			}

			if len(users) == 0 {
				SendToUser(chatID, userHUID, fmt.Sprintf("❌ Пользователь \"%s\" не найден", fullName))
				DeleteMessage(chatID, syncID)
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
				return
			}

			if len(users) > 1 {
				msg := fmt.Sprintf("Найдено несколько пользователей с именем \"%s\":\n\n", fullName)
				for i, u := range users {
					msg += fmt.Sprintf("%d. %s (HUID: %s)\n", i+1, u.Name, u.UserHUID)
				}
				msg += "\nУточните: /add_by_huid <HUID>"
				SendToUser(chatID, userHUID, msg)
				DeleteMessage(chatID, syncID)
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
				return
			}

			targetUser := users[0]

			isMember, err := IsUserInGroup(chatID, targetUser.UserHUID)
			if err != nil {
				SendToUser(chatID, userHUID, fmt.Sprintf("❌ Ошибка проверки: %v", err))
				DeleteMessage(chatID, syncID)
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
				return
			}

			if isMember {
				SendToUser(chatID, userHUID, fmt.Sprintf("⚠️ %s уже в группе", targetUser.Name))
				DeleteMessage(chatID, syncID)
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
				return
			}

			err = AddUserToGroup(chatID, targetUser.UserHUID)
			if err != nil {
				SendToUser(chatID, userHUID, fmt.Sprintf("❌ Ошибка добавления: %v", err))
				DeleteMessage(chatID, syncID)
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
				return
			}

			SendToUser(chatID, userHUID, fmt.Sprintf("✅ %s добавлен в группу", targetUser.Name))
			SendToUser(chatID, targetUser.UserHUID, fmt.Sprintf("Вас добавили в группу DMZ Key Room!\n\n👤 Добавил: %s\n\nИспользуйте кнопки для управления статусом ключа.", userName))
			DeleteMessage(chatID, syncID)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
			return
		}

		// /add_by_huid <HUID>
		if strings.HasPrefix(command, "/add_by_huid ") {
			targetHUID := strings.TrimPrefix(command, "/add_by_huid ")
			targetHUID = strings.TrimSpace(targetHUID)

			_, err := uuid.Parse(targetHUID)
			if err != nil {
				SendToUser(chatID, userHUID, "❌ Неверный формат HUID")
				DeleteMessage(chatID, syncID)
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
				return
			}

			userInfo, err := GetUserInfo(targetHUID)
			if err != nil {
				SendToUser(chatID, userHUID, fmt.Sprintf("❌ Пользователь не найден: %v", err))
				DeleteMessage(chatID, syncID)
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
				return
			}

			isMember, err := IsUserInGroup(chatID, targetHUID)
			if err != nil {
				SendToUser(chatID, userHUID, fmt.Sprintf("❌ Ошибка: %v", err))
				DeleteMessage(chatID, syncID)
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
				return
			}

			if isMember {
				SendToUser(chatID, userHUID, fmt.Sprintf("⚠️ %s уже в группе", userInfo.Name))
				DeleteMessage(chatID, syncID)
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
				return
			}

			err = AddUserToGroup(chatID, targetHUID)
			if err != nil {
				SendToUser(chatID, userHUID, fmt.Sprintf("❌ Ошибка: %v", err))
				DeleteMessage(chatID, syncID)
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
				return
			}

			SendToUser(chatID, userHUID, fmt.Sprintf("✅ %s добавлен в группу", userInfo.Name))
			SendToUser(chatID, targetHUID, fmt.Sprintf("🎉 Вас добавили в группу DMZ Key Room!\n\n👤 Добавил: %s", userName))
			DeleteMessage(chatID, syncID)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
			return
		}
	}

	// Обычные команды
	switch command {
	case "/status":
		status, datechange := GetStatus()
		var statusText string
		switch status {
		case 0:
			statusText = "0 - Ключ на ресепшене, ДЗ на охране"
		case 1:
			statusText = "1 - Ключ на ресепшене, ДЗ не на охране"
		case 2:
			statusText = "2 - ДЗ закрыт на ключ"
		case 3:
			statusText = "3 - ДЗ открыт, ключ у меня"
		default:
			statusText = "не определен"
		}
		msg := fmt.Sprintf("*Текущий статус ключа:*\n%s\n\nПоследнее изменение: %s", statusText, datechange.Format("02/01/2006 15:04:05"))
		SendToUser(chatID, userHUID, msg)

	case "/history":
		history, err := GetHistory()
		if err != nil || len(history) == 0 {
			SendToUser(chatID, userHUID, "История пуста")
		} else {
			msg := "*История изменения статуса ключа:*\n\n"
			for _, h := range history {
				msg += fmt.Sprintf("%s\n", h["message"])
			}
			SendToUser(chatID, userHUID, msg)
		}

	case "/help":
		helpText := `0 - Ключ на ресепшене, ДЗ на охране
1 - Ключ на ресепшене, ДЗ не на охране
2 - ДЗ закрыт на ключ
3 - ДЗ открыт, ключ у меня
/status - показать статус
/history - история статуса ключа
/help - показать справку

+79262292310 - телефон для снятия с охраны демозала`
		SendToUser(chatID, userHUID, helpText)

	default:
		// Игнорируем неизвестные команды
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"service": "express-dmz-bot",
	})
}

func startStatusResetter() {
    go func() {
	for {
	    // Вычисляем время до следующего 23:59
	    now := time.Now()
	    next := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 0, 0, now.Location())
	    
	    // Если уже прошло 23:59 сегодня, ставим на завтра
	    if now.After(next) {
		next = next.Add(24 * time.Hour)
	    }
	    
	    waitDuration := next.Sub(now)
	    log.Printf("Следующий сброс статуса в: %s (через %s)", next.Format("15:04:05"), waitDuration)
	    
	    // Ждем до 23:59
	    time.Sleep(waitDuration)
	    
	    // Сбрасываем статус на 0
	    log.Println("Автоматический сброс статуса на 0 (23:59)")
	    
	    nowTime := time.Now().Format("02/01/2006 15:04:05")
	    SetStatus(0, "system", "Система", "")
	    AddHistory(0, "system", "Система", "", fmt.Sprintf("%s: Автоматический сброс статуса на 0 (ночной сброс)", nowTime))
	    
	    log.Println("Статус сброшен на 0")
	}
    }()
}

// ============= MAIN =============

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("⚠️ .env файл не найден")
	}

	config = Config{
		ExpressDomain: os.Getenv("EXPRESS_DOMAIN"),
		BotID:         os.Getenv("BOT_ID"),
		SecretKey:     os.Getenv("SECRET_KEY"),
		ChatID:        os.Getenv("CHAT_ID"),
		WebhookPort:   os.Getenv("WEBHOOK_PORT"),
		DBPath:        os.Getenv("DB_PATH"),
	}

	if config.WebhookPort == "" {
		config.WebhookPort = "8080"
	}
	if config.DBPath == "" {
		config.DBPath = "dmzkeyroom.db"
	}

	if config.ExpressDomain == "" || config.BotID == "" || config.SecretKey == "" || config.ChatID == "" {
		log.Fatal("❌ Проверьте .env файл")
	}

	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("DMZ Key Bot для Express")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("Express:   %s\n", config.ExpressDomain)
	fmt.Printf("Bot ID:    %s\n", config.BotID)
	fmt.Printf("Chat ID:   %s\n", config.ChatID)
	fmt.Printf("Порт:      %s\n", config.WebhookPort)
	fmt.Printf("База:      %s\n", config.DBPath)
	fmt.Println(strings.Repeat("=", 60) + "\n")

	if err := initDB(); err != nil {
		log.Fatalf("Ошибка инициализации БД: %v", err)
	}
	defer db.Close()

	startTokenRefresher()
	startStatusResetter()
	if _, err := GetToken(); err != nil {
		log.Fatalf("Не удалось получить токен: %v", err)
	}
	log.Println("Токен получен")

	log.Println("Отправка кнопок в чат...")
	if err := SendButtons(config.ChatID); err != nil {
		log.Printf("Ошибка: %v", err)
	} else {
		log.Println("Кнопки отправлены!")
	}

	http.HandleFunc("/", webhookHandler)
	http.HandleFunc("/health", healthHandler)

	log.Printf("Сервер запущен на порту %s", config.WebhookPort)
	log.Println("Ожидание команд...\n")

	if err := http.ListenAndServeTLS(":"+config.WebhookPort, "cert.pem", "key.pem", nil); err != nil {
        log.Fatal("Ошибка:", err)
    }
}
