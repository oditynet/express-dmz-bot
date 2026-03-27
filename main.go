//version 0.6 beta

package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"database/sql"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
	"crypto/x509"
        "encoding/pem"
        "encoding/base64"

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
	CertFile      string
	KeyFile       string
}

var config Config
var db *sql.DB

// ============= СТРУКТУРЫ =============

type Button struct {
	Command string                 `json:"command"`
	Label   string                 `json:"label"`
	Data    map[string]interface{} `json:"data,omitempty"`
	HSize   int                    `json:"h_size,omitempty"`
        Opts    map[string]interface{} `json:"opts,omitempty"` 
}

type Notification struct {
	Status            string     `json:"status"`
	Body              string     `json:"body"`
	Bubble            [][]Button `json:"bubble,omitempty"`
	ButtonsAutoAdjust bool       `json:"buttons_auto_adjust,omitempty"`
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
	Phone    string   `json:"phone"`
	Mobile   string   `json:"mobile"`
	Active   bool     `json:"active"`
}

type ChatMember struct {
	HUID     string `json:"user_huid"`
	Name     string `json:"name"`
	Admin    bool   `json:"admin"`
	Owner    bool   `json:"owner"`
	UserKind string `json:"user_kind"`
}

type ChatInfo struct {
	GroupChatID string       `json:"group_chat_id"`
	Name        string       `json:"name"`
	ChatType    string       `json:"chat_type"`
	Members     []ChatMember `json:"members"`
}

type WebhookRequest struct {
    Command struct {
	Body     string                 `json:"body"`
	Data     map[string]interface{} `json:"data"`
	Metadata map[string]interface{} `json:"metadata"`
    } `json:"command"`
    From struct {
	UserHUID    string `json:"user_huid"`
	Username    string `json:"username"`
	Name        string `json:"name"`
	GroupChatID string `json:"group_chat_id"`
	ChatType    string `json:"chat_type"`
	AdLogin     string `json:"ad_login"`
	AdDomain    string `json:"ad_domain"`
	IsAdmin     bool   `json:"is_admin"`
	IsCreator   bool   `json:"is_creator"`
    } `json:"from"`
    BotID        string                   `json:"bot_id"`
    SyncID       string                   `json:"sync_id"`
    ProtoVersion int                      `json:"proto_version"`
    SourceSyncID interface{}              `json:"source_sync_id"`
    Entities     []interface{}            `json:"entities,omitempty"`
    Attachments  []map[string]interface{} `json:"attachments,omitempty"`
    AsyncFiles   []interface{}            `json:"async_files,omitempty"`
}

// ============= ТОКЕН =============
type TokenManager struct {
	token     string
	expiresAt time.Time
	mu        sync.RWMutex
}

var tokenManager = &TokenManager{}


/*func downloadFile(fileID string) ([]byte, error) {
    token, err := GetToken()
    if err != nil {
	return nil, err
    }
    
    url := fmt.Sprintf("%s/api/v3/botx/files/%s", config.ExpressDomain, fileID)
    req, _ := http.NewRequest("GET", url, nil)
    req.Header.Set("Authorization", "Bearer "+token)
    
    client := &http.Client{Timeout: 30 * time.Second}
    resp, err := client.Do(req)
    if err != nil {
	return nil, err
    }
    defer resp.Body.Close()
    
    return io.ReadAll(resp.Body)
}
func processUserList(chatID, adminHUID string, fileData []byte) {
    lines := strings.Split(string(fileData), "\n")
    var successCount, failCount int
    var failedUsers []string
    
    for _, line := range lines {
	line = strings.TrimSpace(line)
	if line == "" {
	    continue
	}
	
	// Ищем пользователя по ФИО
	users, err := SearchUserByName(line)
	if err != nil || len(users) == 0 {
	    log.Printf("Пользователь не найден: %s", line)
	    failCount++
	    failedUsers = append(failedUsers, line)
	    continue
	}
	
	targetUser := users[0]
	
	// Проверяем, в группе ли уже
	isMember, _ := IsUserInGroup(chatID, targetUser.UserHUID)
	if isMember {
	    log.Printf("Пользователь уже в группе: %s", line)
	    successCount++
	    continue
	}
	
	// Добавляем в группу
	err = AddUserToGroup(chatID, targetUser.UserHUID)
	if err != nil {
	    log.Printf("Ошибка добавления %s: %v", line, err)
	    failCount++
	    failedUsers = append(failedUsers, fmt.Sprintf("%s (ошибка: %v)", line, err))
	    continue
	}
	
	// Отправляем уведомление пользователю
	SendToUser(chatID, targetUser.UserHUID, fmt.Sprintf("Вас добавили в группу DMZ Key Room!\n\nДобавил: %s", adminHUID))
	
	successCount++
	time.Sleep(500 * time.Millisecond)
    }
    
    // Отчет администратору
    report := fmt.Sprintf("✅ Обработка списка пользователей завершена\n\n✅ Успешно добавлено: %d\n❌ Не найдено/ошибок: %d", successCount, failCount)
    
    if len(failedUsers) > 0 && len(failedUsers) <= 10 {
	report += "\n\nНе добавлены:\n"
	for _, name := range failedUsers {
	    report += fmt.Sprintf("- %s\n", name)
	}
    } else if len(failedUsers) > 10 {
	report += fmt.Sprintf("\n\nИ еще %d пользователей не добавлено (список в логах)", len(failedUsers)-10)
    }
    
    SendToUser(chatID, adminHUID, report)
}*/
func handleFileUpload(chatID, userHUID, fileName, content string) {
    // Декодируем base64
    decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(content, "data:text/plain;base64,"))
    if err != nil {
        SendToUser(chatID, userHUID, fmt.Sprintf("Ошибка декодирования файла: %v", err))
        return
    }
    
    // Разбираем строки с ФИО
    lines := strings.Split(string(decoded), "\n")
    addedCount := 0
    notFoundCount := 0
    
    for _, line := range lines {
        line = strings.TrimSpace(line)
        if line == "" {
            continue
        }
        
        users, err := SearchUserByName(line)
        if err != nil || len(users) == 0 {
            notFoundCount++
            continue
        }
        
        // Берем первого найденного
        targetUser := users[0]
        
        // Проверяем, не в группе ли уже
        isMember, _ := IsUserInGroup(chatID, targetUser.UserHUID)
        if isMember {
            continue
        }
        
        if err := AddUserToGroup(chatID, targetUser.UserHUID); err == nil {
            addedCount++
            SendToUser(chatID, targetUser.UserHUID, fmt.Sprintf("Вас добавили в группу DMZ Key Room!"))
        }
    }
    
    msg := fmt.Sprintf("Обработка файла %s завершена:\nДобавлено: %d\nНе найдено: %d", fileName, addedCount, notFoundCount)
    SendToUser(chatID, userHUID, msg)
}

func generateSignature(botID, secretKey string) string {
	h := hmac.New(sha256.New, []byte(secretKey))
	h.Write([]byte(botID))
	return strings.ToUpper(hex.EncodeToString(h.Sum(nil)))
}

func fetchNewToken() (string, error) {
	signature := generateSignature(config.BotID, config.SecretKey)
	url := fmt.Sprintf("%s/api/v2/botx/bots/%s/token?signature=%s",
		config.ExpressDomain, config.BotID, signature)

	//log.Printf("Запрос токена: %s", url)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	//log.Printf("Ответ токена: %s", string(body))

	var result struct {
		Status string `json:"status"`
		Result string `json:"result"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
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

	//log.Println("Обновление токена...")
	newToken, err := fetchNewToken()
	if err != nil {
		return "", err
	}

	tokenManager.token = newToken
	tokenManager.expiresAt = time.Now().Add(14 * time.Minute)
	//log.Printf("Токен обновлен. Действует до: %s", tokenManager.expiresAt.Format("15:04:05"))
	return newToken, nil
}

func startTokenRefresher() {
	ticker := time.NewTicker(14 * time.Minute)
	go func() {
		for range ticker.C {
			log.Println("Плановое обновление токена...")
			newToken, err := fetchNewToken()
			if err != nil {
				log.Printf("Ошибка обновления: %v", err)
				continue
			}
			tokenManager.mu.Lock()
			tokenManager.token = newToken
			tokenManager.expiresAt = time.Now().Add(14 * time.Minute)
			tokenManager.mu.Unlock()
			log.Println("Токен обновлен")
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

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS user_consent (
			user_huid TEXT PRIMARY KEY,
			user_name TEXT NOT NULL,
			consent_status INTEGER NOT NULL DEFAULT 0,
			consent_date TIMESTAMP,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
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
		log.Println("Создана запись статуса по умолчанию")
	}

	log.Println("База данных инициализирована")
	return nil
}

func GetStatus() (int, time.Time) {
    var status int
    var datechangeStr string
    err := db.QueryRow("SELECT status, datechange FROM key_status WHERE id = 1").Scan(&status, &datechangeStr)
    if err != nil {
	if err == sql.ErrNoRows {
	    now := time.Now()
	    db.Exec("INSERT INTO key_status (id, status, datechange) VALUES (1, 0, ?)", now.Format("2006-01-02 15:04:05"))
	    return 0, now
	}
	log.Printf("Ошибка получения статуса: %v", err)
	return 0, time.Now()
    }
    
    // Парсинг даты
    datechange, err := time.Parse("2006-01-02 15:04:05", datechangeStr)
    if err != nil {
	log.Printf("Ошибка парсинга даты: %v, строка: %s", err, datechangeStr)
	return status, time.Now()
    }
    
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

// ============= УПРАВЛЕНИЕ СОГЛАСИЕМ =============

func GetUserConsent(userHUID string) (int, error) {
	var consentStatus int
	err := db.QueryRow("SELECT consent_status FROM user_consent WHERE user_huid = ?", userHUID).Scan(&consentStatus)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return consentStatus, nil
}

func SetUserConsent(userHUID, userName string, consentStatus int) error {
	now := time.Now().Format("2006-01-02 15:04:05")

	var consentDate interface{}
	if consentStatus == 1 {
		consentDate = now
	} else {
		consentDate = nil
	}

	_, err := db.Exec(`
		INSERT INTO user_consent (user_huid, user_name, consent_status, consent_date, updated_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(user_huid) DO UPDATE SET
			user_name = excluded.user_name,
			consent_status = excluded.consent_status,
			consent_date = excluded.consent_date,
			updated_at = excluded.updated_at
	`, userHUID, userName, consentStatus, consentDate, now)

	if err != nil {
		log.Printf("Ошибка сохранения согласия: %v", err)
		return err
	}

	log.Printf("Пользователь %s (%s) статус согласия: %d", userName, userHUID, consentStatus)
	return nil
}

// ============= ОТПРАВКА СООБЩЕНИЙ =============

func sendRequest(token string, payload SendRequest) error {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("ошибка маршалинга: %v", err)
	}

	log.Printf("Отправка запроса в Express:")
	log.Printf("   Payload: %s", string(jsonData))

	url := fmt.Sprintf("%s/api/v4/botx/notifications/direct", config.ExpressDomain)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	log.Printf("Ответ от Express: %s", string(body))

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("ошибка парсинга ответа: %v", err)
	}

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

	log.Printf("Отправка сообщения пользователю %s в чат %s", userHUID, chatID)

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

func SendButtonsToAll(chatID string) error {
	token, err := GetToken()
	if err != nil {
		return err
	}

	log.Printf("Отправка кнопок в чат: %s", chatID)

	buttons := [][]Button{
		{
			{Command: "0", Label: "0", HSize: 3, Opts: map[string]interface{}{"silent": true}},
			{Command: "1", Label: "1", HSize: 3, Opts: map[string]interface{}{"silent": true}},
			{Command: "2", Label: "2", HSize: 3, Opts: map[string]interface{}{"silent": true}},
			{Command: "3", Label: "3", HSize: 3, Opts: map[string]interface{}{"silent": true}},
		},
		{
			{Command: "/status", Label: "Статус", HSize: 4, Opts: map[string]interface{}{"silent": true}},
			{Command: "/history", Label: "История", HSize: 4, Opts: map[string]interface{}{"silent": true}},
			{Command: "/help", Label: "Помощь", HSize: 4, Opts: map[string]interface{}{"silent": true}},
		},
	}

	payload := SendRequest{
		GroupChatID: chatID,
		Notification: Notification{
			Status:            "ok",
			Body:              "DMZ Key Room - управление ключом",
			Bubble:            buttons,
			ButtonsAutoAdjust: true,
		},
	}

	return sendRequest(token, payload)
}

func SendButtonsToUser(chatID, userHUID string) error {
	consentStatus, _ := GetUserConsent(userHUID)
	if consentStatus != 1 {
		log.Printf("Пользователь %s не дал согласия, отправляем запрос", userHUID)
		return SendConsentRequest(chatID, userHUID)
	}

	token, err := GetToken()
	if err != nil {
		return err
	}

	log.Printf("Отправка кнопок пользователю %s в чат %s", userHUID, chatID)
        buttons := [][]Button{
    {
        {Command: "0", Label: "0", HSize: 3, Opts: map[string]interface{}{"silent": true}},
        {Command: "1", Label: "1", HSize: 3, Opts: map[string]interface{}{"silent": true}},
        {Command: "2", Label: "2", HSize: 3, Opts: map[string]interface{}{"silent": true}},
        {Command: "3", Label: "3", HSize: 3, Opts: map[string]interface{}{"silent": true}},
    },
    {
        {Command: "/status", Label: "Статус", HSize: 4, Opts: map[string]interface{}{"silent": true}},
        {Command: "/history", Label: "История", HSize: 4, Opts: map[string]interface{}{"silent": true}},
        {Command: "/help", Label: "Помощь", HSize: 4, Opts: map[string]interface{}{"silent": true}},
    },
}

	payload := SendRequest{
		GroupChatID: chatID,
		Recipients:  []string{userHUID},
		Notification: Notification{
			Status:            "ok",
			Body:              "DMZ Key Room - управление ключом",
			Bubble:            buttons,
			ButtonsAutoAdjust: true,
		},
	}

	return sendRequest(token, payload)
}

func SendConsentRequest(chatID, userHUID string) error {
	token, err := GetToken()
	if err != nil {
		return err
	}

	buttons := [][]Button{
		{
		{Command: "/consent_yes", Label: "Согласен", HSize: 6, Opts: map[string]interface{}{"silent": true}},
                {Command: "/consent_no", Label: "Не согласен", HSize: 6, Opts: map[string]interface{}{"silent": true}},
		},
	}

	message := `*Согласие на обработку персональных данных*

В соответствии с Федеральным законом № 152-ФЗ "О персональных данных" информируем вас, что:

Ваши персональные данные (ФИО, номер телефона, HUID) будут обрабатываться
Данные хранятся в базе данных
Цель обработки: ведение учета статуса ключа от демозала
Срок хранения: бессрочный

Для продолжения работы с ботом необходимо ваше согласие.

*Вы согласны на обработку ваших персональных данных?*`

	payload := SendRequest{
		GroupChatID: chatID,
		Recipients:  []string{userHUID},
		Notification: Notification{
			Status:            "ok",
			Body:              message,
			Bubble:            buttons,
			ButtonsAutoAdjust: true,
		},
	}

	log.Printf("Отправка запроса согласия пользователю %s", userHUID)
	return sendRequest(token, payload)
}


func AddUserToGroup(chatID, userHUID string) error {
	token, err := GetToken()
	if err != nil {
		return err
	}

	payload := map[string]interface{}{
		"group_chat_id": chatID,
		"user_huids":    []string{userHUID},
	}

	jsonData, _ := json.Marshal(payload)
	url := fmt.Sprintf("%s/api/v3/botx/chats/add_user", config.ExpressDomain)
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

	url := fmt.Sprintf("%s/api/v3/botx/chats/info?group_chat_id=%s", config.ExpressDomain, chatID)
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
		return nil, fmt.Errorf("failed to get chat info: %s", result.Status)
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
		return nil, fmt.Errorf("user not found: %s", result.Status)
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
			if len(record) > 3 && record[3] != "" {
				user.Emails = []string{record[3]}
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

func GetAllUserHUIDs(chatID string) ([]string, error) {
	chatInfo, err := GetChatInfo(chatID)
	if err != nil {
		return nil, err
	}

	huids := make([]string, len(chatInfo.Members))
	for i, member := range chatInfo.Members {
		huids[i] = member.HUID
		log.Printf("Участник %d: %s (%s)", i+1, member.Name, member.HUID)
	}
	return huids, nil
}

func checkCertificateExpiry() {
    certFile := config.CertFile
    if certFile == "" {
        certFile = "cert.pem"
    }
    
    // Читаем сертификат
    certData, err := os.ReadFile(certFile)
    if err != nil {
        log.Printf("Ошибка чтения сертификата: %v", err)
        return
    }
    
    // Парсим сертификат
    block, _ := pem.Decode(certData)
    if block == nil {
        log.Printf("Ошибка декодирования PEM")
        return
    }
    
    cert, err := x509.ParseCertificate(block.Bytes)
    if err != nil {
        log.Printf("Ошибка парсинга сертификата: %v", err)
        return
    }
    
    now := time.Now()
    daysLeft := int(cert.NotAfter.Sub(now).Hours() / 24)
    
    log.Printf("Сертификат действителен до: %s, осталось дней: %d", cert.NotAfter.Format("02.01.2006"), daysLeft)
    
    // Если осталось 14 дней или меньше
    if daysLeft <= 14 && daysLeft > 0 {
        // Получаем всех администраторов чата
        admins, err := getChatAdmins(config.ChatID)
        if err != nil {
            log.Printf("Ошибка получения администраторов: %v", err)
            return
        }
        
        msg := fmt.Sprintf(`⚠️ *Внимание!*

Сертификат бота DMZ Key Room истекает через %d дней.

Дата истечения: %s

Необходимо продлить сертификат.`, 
            daysLeft, cert.NotAfter.Format("02.01.2006 15:04:05"))
        
        // Отправляем уведомление каждому администратору
        for _, adminHUID := range admins {
            if err := SendToUser(config.ChatID, adminHUID, msg); err != nil {
                log.Printf("Ошибка отправки уведомления администратору %s: %v", adminHUID, err)
            } else {
                log.Printf("Уведомление отправлено администратору %s", adminHUID)
            }
        }
    }
}

// Получение списка администраторов чата
func getChatAdmins(chatID string) ([]string, error) {
    chatInfo, err := GetChatInfo(chatID)
    if err != nil {
        return nil, err
    }
    
    var admins []string
    for _, member := range chatInfo.Members {
        if member.Admin || member.Owner {
            admins = append(admins, member.HUID)
        }
    }
    return admins, nil
}

func startCertificateChecker() {
    go func() {
        // Проверяем сразу при запуске
        checkCertificateExpiry()
        
        // Затем проверяем раз в день
        ticker := time.NewTicker(24 * time.Hour)
        for range ticker.C {
            checkCertificateExpiry()
        }
    }()
}
/*func DeleteMessage(chatID, syncID string) error {
    token, err := GetToken()
    if err != nil {
	return err
    }

    // Правильный endpoint по документации
    url := fmt.Sprintf("%s/api/v4/botx/chats/%s/messages/%s", 
	config.ExpressDomain, chatID, syncID)
    
    log.Printf("Удаление сообщения: %s", url)
    
    req, err := http.NewRequest("DELETE", url, nil)
    if err != nil {
	return err
    }
    req.Header.Set("Authorization", "Bearer "+token)

    client := &http.Client{Timeout: 10 * time.Second}
    resp, err := client.Do(req)
    if err != nil {
	return err
    }
    defer resp.Body.Close()

    body, _ := io.ReadAll(resp.Body)
    log.Printf("Ответ на удаление: %s", string(body))
    log.Printf("Статус удаления: %d", resp.StatusCode)

    // Успешное удаление обычно возвращает 200 OK или 204 No Content
    if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent {
	log.Printf("Сообщение %s успешно удалено", syncID)
	return nil
    }

    return fmt.Errorf("failed to delete message, status: %d, body: %s", resp.StatusCode, string(body))
}
*/
func HideMessage(syncID string) error {
    token, err := GetToken()
    if err != nil {
        return err
    }

    payload := map[string]interface{}{
        "sync_id": syncID,
        "payload": map[string]interface{}{
            "body":        "Файл обработан и скрыт",
            "attachments": []interface{}{},
        },
    }

    jsonData, _ := json.Marshal(payload)
    url := fmt.Sprintf("%s/api/v3/botx/events/edit_event", config.ExpressDomain)
    
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

// ============= ОБРАБОТЧИКИ =============
func webhookHandler(w http.ResponseWriter, r *http.Request) {
    log.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
    log.Printf("Получен запрос: %s %s", r.Method, r.URL.Path)

    if r.Method == "GET" {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	return
    }

    if r.Method != "POST" {
	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	return
    }

    bodyBytes, err := io.ReadAll(r.Body)
    if err != nil {
	log.Printf("Ошибка чтения тела: %v", err)
	w.WriteHeader(http.StatusBadRequest)
	json.NewEncoder(w).Encode(map[string]string{"status": "error"})
	return
    }
    
//    log.Printf("ПОЛНЫЙ JSON ЗАПРОСА:")
//    log.Printf("%s", string(bodyBytes))
    
    r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

    var data WebhookRequest
    if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
	log.Printf("Ошибка декодирования: %v", err)
	w.WriteHeader(http.StatusBadRequest)
	json.NewEncoder(w).Encode(map[string]string{"status": "error"})
	return
    }
    
    // ========== ОБЪЯВЛЯЕМ ПЕРЕМЕННЫЕ ==========
    command := data.Command.Body
    userHUID := data.From.UserHUID
    userName := data.From.Username
    if userName == "" {
	userName = data.From.Name
    }
    chatID := data.From.GroupChatID
    // ========================================
    
    // Обработка события добавления в чат
    if command == "system:added_to_chat" {
	log.Printf("Событие: добавление пользователя в чат")
	
	var addedMembers []string
	if added, ok := data.Command.Data["added_members"].([]interface{}); ok {
	    for _, m := range added {
		if huid, ok := m.(string); ok {
		    addedMembers = append(addedMembers, huid)
		}
	    }
	}
	
//	log.Printf("Добавленные пользователи: %v", addedMembers)
	
	for _, userHUID := range addedMembers {
	    userInfo, err := GetUserInfo(userHUID)
	    userName := userHUID
	    if err == nil && userInfo != nil {
		userName = userInfo.Name
	    }
	    
	    log.Printf("Отправка согласия пользователю: %s (%s)", userName, userHUID)
	    SendToUser(chatID, userHUID, fmt.Sprintf("Добро пожаловать, %s!", userName))
	//    SendConsentRequest(chatID, userHUID)
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	return
    }
    
    // ========== ОБРАБОТКА ФАЙЛОВ (после объявления переменных) ==========
    if len(data.Attachments) > 0 {
	log.Printf("Получен файл от %s", userName)
	//syncID := data.SyncID 
	
	for _, att := range data.Attachments {
	    attType, _ := att["type"].(string)
	    if attType == "document" {
		attData, ok := att["data"].(map[string]interface{})
		if !ok {
		    continue
		}
		fileName, _ := attData["file_name"].(string)
		content, _ := attData["content"].(string)
		
		//log.Printf("Файл: %s, размер: %d", fileName, len(content))
		
		go handleFileUpload(chatID, userHUID, fileName, content)
		
		SendToUser(chatID, userHUID, fmt.Sprintf("Получен файл: %s, начата обработка", fileName))
		/*if err := DeleteMessage(chatID, syncID); err != nil {
	    	    log.Printf("Ошибка удаления сообщения с файлом: %v", err)
		} else {
		    log.Printf("Сообщение с файлом %s удалено из чата", fileName)
		}*/
		 // Скрываем сообщение с файлом
            /*if err := HideMessage(syncID); err != nil {
                log.Printf("Ошибка скрытия сообщения: %v", err)
            } else {
                log.Printf("Сообщение с файлом %s скрыто", fileName)
            }*/
	    }
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	return
    }
    // ====================================================================

    if command == "" || userHUID == "" {
	log.Printf("Пропускаем callback запрос")
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	return
    }	

	//log.Printf("Команда: '%s' от %s (%s) в чате %s", command, userName, userHUID, chatID)

	if command == "/consent_yes" {
		if err := SetUserConsent(userHUID, userName, 1); err != nil {
			log.Printf("Ошибка сохранения: %v", err)
			SendToUser(chatID, userHUID, "Произошла ошибка. Попробуйте позже.")
		} else {
			SendToUser(chatID, userHUID, "Спасибо! Теперь вы можете использовать бот.")
			SendButtonsToUser(chatID, userHUID)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		return
	}

	if command == "/consent_no" {
		SetUserConsent(userHUID, userName, 0)
		SendConsentRequest(chatID, userHUID)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		return
	}

	consentStatus, _ := GetUserConsent(userHUID)
	if consentStatus != 1 {
		SendConsentRequest(chatID, userHUID)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		return
	}

	isAdmin, _ := IsChatAdmin(chatID, userHUID)
	log.Printf("isAdmin: %v", isAdmin)

	statusMap := map[string]int{"0": 0, "1": 1, "2": 2, "3": 3}
	if status, ok := statusMap[command]; ok {
		userInfo, _ := GetUserInfo(userHUID)
		phone := ""
		if userInfo != nil {
			if userInfo.Phone != "" {
				phone = userInfo.Phone
			} else if userInfo.Mobile != "" {
				phone = userInfo.Mobile
			} else if len(userInfo.Emails) > 0 {
				phone = userInfo.Emails[0]
			}
		}

		SetStatus(status, userHUID, userName, phone)
		now := time.Now().Format("02/01/2006 15:04:05")

		var msg string
		switch status {
		case 0:
			msg = fmt.Sprintf("*Статус: 0 - Ключ на ресепшене, ДЗ на охране*\n\nУстановил: %s\nВремя: %s", userName, now)
			AddHistory(0, userHUID, userName, phone, fmt.Sprintf("%s: Ключ на ресепшене, ДЗ на охране. Установил %s", now, userName))
		case 1:
			msg = fmt.Sprintf("*Статус: 1 - Ключ на ресепшене, ДЗ не на охране*\n\nИнициатор: %s\nТелефон: %s\nВремя: %s", userName, phone, now)
			AddHistory(1, userHUID, userName, phone, fmt.Sprintf("%s: Ключ на ресепшене, ДЗ не на охране. Установил %s по телефону: %s", now, userName, phone))
		case 2:
			msg = fmt.Sprintf("*Статус: 2 - ДЗ закрыт на ключ*\n\nКлюч у: %s\nТелефон: %s\nВремя: %s", userName, phone, now)
			AddHistory(2, userHUID, userName, phone, fmt.Sprintf("%s: ДЗ закрыт на ключ, ключ у %s. Телефон: %s", now, userName, phone))
		case 3:
			msg = fmt.Sprintf("*Статус: 3 - ДЗ открыт, ключ у меня*\n\nКлюч у: %s\nТелефон: %s\nВремя: %s", userName, phone, now)
			AddHistory(3, userHUID, userName, phone, fmt.Sprintf("%s: ДЗ открыт, ключ у %s. Телефон: %s", now, userName, phone))
		}

		SendToUser(chatID, userHUID, msg)
		SendButtonsToUser(chatID, userHUID)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		return
	}

	if isAdmin {
		if strings.HasPrefix(command, "/add ") {
			fullName := strings.TrimPrefix(command, "/add ")
			fullName = strings.TrimSpace(fullName)

			if fullName == "" {
				SendToUser(chatID, userHUID, "Формат: /add <Фамилия Имя Отчество>")
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
				return
			}

			users, err := SearchUserByName(fullName)
			if err != nil {
				SendToUser(chatID, userHUID, fmt.Sprintf("Ошибка поиска: %v", err))
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
				return
			}

			if len(users) == 0 {
				SendToUser(chatID, userHUID, fmt.Sprintf("Пользователь \"%s\" не найден", fullName))
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
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
				return
			}

			targetUser := users[0]
			isMember, _ := IsUserInGroup(chatID, targetUser.UserHUID)
			if isMember {
				SendToUser(chatID, userHUID, fmt.Sprintf("%s уже в группе", targetUser.Name))
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
				return
			}

			if err := AddUserToGroup(chatID, targetUser.UserHUID); err != nil {
				SendToUser(chatID, userHUID, fmt.Sprintf("Ошибка добавления: %v", err))
			} else {
				SendToUser(chatID, userHUID, fmt.Sprintf("%s добавлен в группу", targetUser.Name))
				SendToUser(chatID, targetUser.UserHUID, fmt.Sprintf("Вас добавили в группу DMZ Key Room!\n\nДобавил: %s", userName))
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
			return
		}

		if strings.HasPrefix(command, "/add_by_huid ") {
			targetHUID := strings.TrimPrefix(command, "/add_by_huid ")
			targetHUID = strings.TrimSpace(targetHUID)

			if _, err := uuid.Parse(targetHUID); err != nil {
				SendToUser(chatID, userHUID, "Неверный формат HUID")
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
				return
			}

			userInfo, err := GetUserInfo(targetHUID)
			if err != nil {
				SendToUser(chatID, userHUID, fmt.Sprintf("Пользователь не найден: %v", err))
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
				return
			}

			isMember, _ := IsUserInGroup(chatID, targetHUID)
			if isMember {
				SendToUser(chatID, userHUID, fmt.Sprintf("%s уже в группе", userInfo.Name))
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
				return
			}

			if err := AddUserToGroup(chatID, targetHUID); err != nil {
				SendToUser(chatID, userHUID, fmt.Sprintf("Ошибка: %v", err))
			} else {
				SendToUser(chatID, userHUID, fmt.Sprintf("%s добавлен в группу", userInfo.Name))
				SendToUser(chatID, targetHUID, fmt.Sprintf("Вас добавили в группу DMZ Key Room!\n\nДобавил: %s", userName))
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
			return
		}
	}else if strings.HasPrefix(command, "/add ") || strings.HasPrefix(command, "/add_by_huid ") {
    // Если команда админская, но пользователь не админ
    SendToUser(chatID, userHUID, "У вас нет прав администратора для выполнения этой команды.")
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
    return
}

	switch command {
	case "/status":
	    status, datechange := GetStatus()
	    
	    // Получаем последнюю запись из истории
	    var lastUserName, lastPhone string
	    err := db.QueryRow(`
	        SELECT user_name, phone FROM history 
	        ORDER BY timestamp DESC LIMIT 1
	    `).Scan(&lastUserName, &lastPhone)
	    if err != nil {
	        lastUserName = "неизвестно"
	        lastPhone = "не указан"
	    }
        
	    var statusText string
	    switch status {
	    case 0:
	        statusText = fmt.Sprintf("0 - Ключ на ресепшене, ДЗ на охране\nУстановил: %s\nТелефон: %s", lastUserName, lastPhone)
	    case 1:
	        statusText = fmt.Sprintf("1 - Ключ на ресепшене, ДЗ не на охране\nУстановил: %s\nТелефон: %s", lastUserName, lastPhone)
	    case 2:
	        statusText = fmt.Sprintf("2 - ДЗ закрыт на ключ\nКлюч у: %s\nТелефон: %s", lastUserName, lastPhone)
	    case 3:
	        statusText = fmt.Sprintf("3 - ДЗ открыт\nКлюч у %s\nТелефон: %s", lastUserName, lastPhone)
	    default:
	        statusText = "не определен"
	    }
    
	    msg := fmt.Sprintf("*Текущий статус ключа:*\n%s\n\nПоследнее изменение: %s", 
	        statusText, datechange.Format("02/01/2006 15:04:05"))
	    SendToUser(chatID, userHUID, msg)
	    
	    SendButtonsToUser(chatID, userHUID)
	    w.Header().Set("Content-Type", "application/json")
	    json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	    return
	/*case "/status":
		status, datechange := GetStatus()
		statusText := map[int]string{
			0: "0 - Ключ на ресепшене, ДЗ на охране",
			1: "1 - Ключ на ресепшене, ДЗ не на охране",
			2: "2 - ДЗ закрыт на ключ",
			3: "3 - ДЗ открыт, ключ у меня",
		}[status]
		if statusText == "" {
			statusText = "не определен"
		}
		msg := fmt.Sprintf("*Текущий статус ключа:*\n%s\n\nПоследнее изменение: %s",
			statusText, datechange.Format("02/01/2006 15:04:05"))
		SendToUser(chatID, userHUID, msg)
		
		SendButtonsToUser(chatID, userHUID)
		w.Header().Set("Content-Type", "application/json")
	        json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	        return*/
	


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
		
		SendButtonsToUser(chatID, userHUID)
		w.Header().Set("Content-Type", "application/json")
     	        json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
     	        return


	case "/help":
		helpText := `Справка по управлению ключом демозала

Кнопки статуса:
0 - Ключ на ресепшене, ДЗ на охране
1 - Ключ на ресепшене, ДЗ не на охране
2 - ДЗ закрыт на ключ
3 - ДЗ открыт, ключ у меня

Команды:
/status - показать текущий статус
/history - история изменения статуса
/help - показать эту справку

Позвонить: +79262292310 - телефон для снятия с охраны демозала`
		SendToUser(chatID, userHUID, helpText)
	}
	
	
	SendButtonsToUser(chatID, userHUID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	return
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
			now := time.Now()
			next := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 0, 0, now.Location())

			if now.After(next) {
				next = next.Add(24 * time.Hour)
			}

			waitDuration := next.Sub(now)
			log.Printf("Следующий сброс статуса в: %s (через %s)", next.Format("15:04:05"), waitDuration)
			time.Sleep(waitDuration)

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
		log.Println("файл .env не найден, используем переменные окружения")
	}

	config = Config{
		ExpressDomain: os.Getenv("EXPRESS_DOMAIN"),
		BotID:         os.Getenv("BOT_ID"),
		SecretKey:     os.Getenv("SECRET_KEY"),
		ChatID:        os.Getenv("CHAT_ID"),
		WebhookPort:   os.Getenv("WEBHOOK_PORT"),
		DBPath:        os.Getenv("DB_PATH"),
		CertFile:      os.Getenv("CERT_FILE"),
		KeyFile:       os.Getenv("KEY_FILE"),
	}

	if config.WebhookPort == "" {
		config.WebhookPort = "443"
	}
	if config.DBPath == "" {
		config.DBPath = "dmzkeyroom.db"
	}
	if config.CertFile == "" {
		config.CertFile = "cert.pem"
	}
	if config.KeyFile == "" {
		config.KeyFile = "key.pem"
	}

	if config.ExpressDomain == "" || config.BotID == "" || config.SecretKey == "" || config.ChatID == "" {
		log.Fatal("Проверьте .env файл: EXPRESS_DOMAIN, BOT_ID, SECRET_KEY, CHAT_ID должны быть заданы")
	}

	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("DMZ Key Bot для Express")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("Express:   %s\n", config.ExpressDomain)
	fmt.Printf("Bot ID:    %s\n", config.BotID)
	fmt.Printf("Chat ID:   %s\n", config.ChatID)
	fmt.Printf("Порт:      %s (HTTPS)\n", config.WebhookPort)
	//fmt.Printf("Сертификат: %s\n", config.CertFile)
	fmt.Printf("Ключ:      %s\n", config.KeyFile)
	fmt.Printf("База:      %s\n", config.DBPath)
	fmt.Println(strings.Repeat("=", 60) + "\n")

	if err := initDB(); err != nil {
		log.Fatalf("Ошибка инициализации БД: %v", err)
	}
	defer db.Close()

	//startTokenRefresher()
	startCertificateChecker()
	//checkCertificateExpiry()
	startStatusResetter()

	if _, err := GetToken(); err != nil {
		log.Fatalf("Не удалось получить токен: %v", err)
	}
	//log.Println("Токен получен")

	//if huids, err := GetAllUserHUIDs(config.ChatID); err != nil {
	//	log.Printf("Ошибка получения участников: %v", err)
	//} else {
	//	log.Printf("Найдено участников в чате: %d", len(huids))
	//}

	log.Println("Отправка кнопок в чат...")
	if err := SendButtonsToAll(config.ChatID); err != nil {
		log.Printf("Ошибка отправки кнопок: %v", err)
	} else {
		log.Println("Кнопки отправлены")
	}

	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	http.HandleFunc("/", webhookHandler)
	http.HandleFunc("/health", healthHandler)

	server := &http.Server{
		Addr:      ":" + config.WebhookPort,
		Handler:   nil,
		TLSConfig: tlsConfig,
	}

	log.Printf("Сервер запущен на порту %s (HTTPS)", config.WebhookPort)
	log.Printf("TLS: минимальная версия TLS 1.2")
	log.Printf("Сертификат: %s", config.CertFile)
	log.Printf("Ключ: %s", config.KeyFile)
	log.Println("Ожидание вебхуков от Express...\n")

	if err := server.ListenAndServeTLS(config.CertFile, config.KeyFile); err != nil {
		log.Fatalf("Ошибка запуска HTTPS сервера: %v", err)
	}
}
