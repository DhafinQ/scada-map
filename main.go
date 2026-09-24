package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	_ "github.com/go-sql-driver/mysql"
)

type Config struct {
	Port  int        `json:"port"`
	DB    DBConfig   `json:"db"`
	MQTT  MQTTConfig `json:"mqtt"`
	Nodes []NodeItem `json:"nodes"`
}

type DBConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	Database string `json:"database"`
}

type MQTTConfig struct {
	Enabled  bool   `json:"enabled"`
	Broker   string `json:"broker"`   // Contoh: "tcp://broker.emqx.io" atau "tcp://host.docker.internal"
	Port     int    `json:"port"`     // Default 1883
	ClientID string `json:"client_id"`
	User     string `json:"user"`
	Password string `json:"password"`
	Topic    string `json:"topic"`    // e.g. "scada/nodes/#" atau topic agregat
}

type NodeItem struct {
	ID           string                    `json:"id"`
	Name         string                    `json:"name"`
	Lat          float64                   `json:"lat"`
	Lng          float64                   `json:"lng"`
	StatusSource interface{}               `json:"status_source"`
	DataSource   string                    `json:"data_source"` // "db" atau "mqtt"
	MQTTTopic    string                    `json:"mqtt_topic"`  // Topic spesifik jika pakai MQTT
	Metrics      map[string]MetricProperty `json:"metrics"`
}

type MetricProperty struct {
	Label  string `json:"label"`
	Unit   string `json:"unit"`
	Table  string `json:"table"`  // Untuk sumber DB
	Column string `json:"column"` // Untuk sumber DB, atau Key JSON jika sumber MQTT
}

var (
	appConfig   Config
	configMutex sync.RWMutex
	dbPool      *sql.DB
	mqttClient  mqtt.Client

	// Cache thread-safe untuk menampung telemetri terakhir dari MQTT
	// Key: Node ID atau Topic -> Value: Map key-value telemetri
	mqttDataStore = make(map[string]map[string]interface{})
	mqttMutex     sync.RWMutex
)

type MQTTRawPayload struct {
	Records   []MQTTRecord `json:"Records"`
	Timestamp string       `json:"Timestamp"`
}

type MQTTRecord struct {
	TagName string      `json:"TagName"`
	Value   interface{} `json:"Value"`
}

func main() {
	// 1. Baca konfigurasi
	if err := loadConfig(); err != nil {
		log.Fatalf("[ERROR] Gagal membaca config.json: %v", err)
	}

	// 2. Inisialisasi Database MySQL
	initDB()

	// 3. Inisialisasi MQTT Client (jika diaktifkan)
	if appConfig.MQTT.Enabled {
		initMQTT()
	}

	// 4. Routing
	http.HandleFunc("/api/nodes", handleNodesAPI)
	http.HandleFunc("/api/config", handleConfigAPI)
	http.Handle("/", http.FileServer(http.Dir(".")))

	port := 8080
	if appConfig.Port != 0 {
		port = appConfig.Port
	}

	fmt.Printf("[SERVER] Siap berjalan di http://localhost:%d\n", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
}

func loadConfig() error {
	configMutex.Lock()
	defer configMutex.Unlock()

	configFile, err := os.ReadFile("config.json")
	if err != nil {
		return err
	}
	return json.Unmarshal(configFile, &appConfig)
}

func initDB() {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?timeout=5s",
		appConfig.DB.User, appConfig.DB.Password,
		appConfig.DB.Host, appConfig.DB.Port, appConfig.DB.Database,
	)

	var err error
	dbPool, err = sql.Open("mysql", dsn)
	if err != nil {
		log.Printf("[DB] Driver Error: %v", err)
		return
	}
	dbPool.SetMaxOpenConns(10)
	dbPool.SetMaxIdleConns(5)

	fmt.Println("[INFO] Mencoba menghubungkan ke MySQL...")
	if err = dbPool.Ping(); err != nil {
		log.Printf("\n❌ KONEKSI DATABASE GAGAL: %v\n", err)
	} else {
		fmt.Printf("✅ BERHASIL TERHUBUNG ke database MySQL: %s@%s:%d/%s\n\n",
			appConfig.DB.User, appConfig.DB.Host, appConfig.DB.Port, appConfig.DB.Database)
	}
}

func initMQTT() {
	brokerURI := fmt.Sprintf("%s:%d", appConfig.MQTT.Broker, appConfig.MQTT.Port)
	opts := mqtt.NewClientOptions().AddBroker(brokerURI)
	clientID := appConfig.MQTT.ClientID
	if clientID == "" {
		clientID = fmt.Sprintf("scada-map-%d", time.Now().Unix())
	}
	opts.SetClientID(clientID)
	if appConfig.MQTT.User != "" {
		opts.SetUsername(appConfig.MQTT.User)
		opts.SetPassword(appConfig.MQTT.Password)
	}
	opts.SetAutoReconnect(true)
	opts.SetKeepAlive(60 * time.Second)

	// Callback penanganan pesan masuk
	opts.SetDefaultPublishHandler(func(client mqtt.Client, msg mqtt.Message) {
	var raw MQTTRawPayload
	if err := json.Unmarshal(msg.Payload(), &raw); err != nil {
		log.Printf("[MQTT] Gagal parse payload dari topic %s: %v", msg.Topic(), err)
		return
	}

	// Flattening records: TagName -> Value
	flatData := make(map[string]interface{})
	for _, rec := range raw.Records {
		flatData[rec.TagName] = rec.Value
	}

	mqttMutex.Lock()
	// Simpan ke memory berdasarkan nama topic
	mqttDataStore[msg.Topic()] = flatData
	mqttMutex.Unlock()
})

	opts.OnConnect = func(c mqtt.Client) {
		fmt.Printf("✅ BERHASIL TERHUBUNG ke MQTT Broker: %s\n", brokerURI)
		// Subscribe ke topic yang ditentukan di config
		topic := appConfig.MQTT.Topic
		if topic == "" {
			topic = "scada/nodes/#"
		}
		c.Subscribe(topic, 1, nil)
		fmt.Printf("[MQTT] Subscribed to topic: %s\n\n", topic)
	}

	opts.OnConnectionLost = func(c mqtt.Client, err error) {
		log.Printf("⚠️ KONEKSI MQTT TERPUTUS: %v. Mencoba rekoneksi...", err)
	}

	mqttClient = mqtt.NewClient(opts)
	if token := mqttClient.Connect(); token.Wait() && token.Error() != nil {
		log.Printf("❌ GAGAL TERHUBUNG KE MQTT BROKER: %v", token.Error())
	}
}

func handleNodesAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	configMutex.RLock()
	nodesCopy := make([]NodeItem, len(appConfig.Nodes))
	copy(nodesCopy, appConfig.Nodes)
	configMutex.RUnlock()

	// 1. Kumpulkan kebutuhan query MySQL untuk node yang bertipe "db"
	tableCols := make(map[string]map[string]bool)
	for _, node := range nodesCopy {
		source := strings.ToLower(node.DataSource)
		if source == "db" || source == "" { // Default ke DB
			for _, m := range node.Metrics {
				if m.Table != "" && m.Column != "" {
					if tableCols[m.Table] == nil {
						tableCols[m.Table] = make(map[string]bool)
					}
					tableCols[m.Table][m.Column] = true
				}
			}
		}
	}

	// 2. Query MySQL (hanya jika ada tabel yang perlu di-query & pool tersedia)
	dbSnapshots := make(map[string]map[string]interface{})
	if dbPool != nil && len(tableCols) > 0 {
		for tbl, cols := range tableCols {
			var colNames []string
			for c := range cols {
				colNames = append(colNames, fmt.Sprintf("`%s`", c))
			}
			query := fmt.Sprintf("SELECT %s FROM `%s` ORDER BY `id` DESC LIMIT 1", strings.Join(colNames, ", "), tbl)

			rows, err := dbPool.Query(query)
			if err != nil {
				log.Printf("[QUERY ERROR] Tabel '%s': %v", tbl, err)
				continue
			}

			columns, _ := rows.Columns()
			if rows.Next() {
				values := make([]interface{}, len(columns))
				valuePtrs := make([]interface{}, len(columns))
				for i := range columns {
					valuePtrs[i] = &values[i]
				}
				if err := rows.Scan(valuePtrs...); err == nil {
					rowMap := make(map[string]interface{})
					for i, col := range columns {
						val := values[i]
						if b, ok := val.([]byte); ok {
							rowMap[col] = string(b)
						} else {
							rowMap[col] = val
						}
					}
					dbSnapshots[tbl] = rowMap
				}
			}
			rows.Close()
		}
	}

	// 3. Susun Output JSON
	type OutputMetric struct {
		Label  string            `json:"label"`
		Value  interface{}       `json:"value"`
		Unit   string            `json:"unit"`
		Source map[string]string `json:"source"`
	}

	type OutputNode struct {
		ID          string                  `json:"id"`
		Name        string                  `json:"name"`
		Lat         float64                 `json:"lat"`
		Lng         float64                 `json:"lng"`
		Status      string                  `json:"status"`
		DataSource  string                  `json:"data_source"`
		LastUpdated string                  `json:"last_updated"`
		JsonData    map[string]OutputMetric `json:"json_data"`
	}

	var outputNodes []OutputNode
	currentTime := time.Now().Format("02 Jan 2006 - 15:04:05 WIB")

	mqttMutex.RLock()
	defer mqttMutex.RUnlock()

	for _, node := range nodesCopy {
		jsonData := make(map[string]OutputMetric)
		sourceMode := strings.ToLower(node.DataSource)
		if sourceMode == "" {
			sourceMode = "db"
		}

		for k, m := range node.Metrics {
			var val interface{}

			if sourceMode == "mqtt" {
				// Ambil langsung via TagName yang didefinisikan pada m.Column
				if payload, found := mqttDataStore[node.MQTTTopic]; found {
					val = payload[m.Column]
				}
			} else {
				// Query dari database MySQL
				if snapshot, found := dbSnapshots[m.Table]; found {
					val = snapshot[m.Column]
				}
			}

		// Jika sensor mengirim null, Anda bisa tetapkan fallback default jika diinginkan
		if val == nil {
			val = "-"
		}

		jsonData[k] = OutputMetric{
			Label: m.Label,
			Value: val,
			Unit:  m.Unit,
			Source: map[string]string{
				"type":   sourceMode,
				"table":  m.Table,
				"column": m.Column, // Berisi nama TagName jika sumber MQTT
		},
	}
}

		status := "ON"
		if strStatus, ok := node.StatusSource.(string); ok {
			status = strStatus
		}

		outputNodes = append(outputNodes, OutputNode{
			ID:          node.ID,
			Name:        node.Name,
			Lat:         node.Lat,
			Lng:         node.Lng,
			Status:      status,
			DataSource:  sourceMode,
			LastUpdated: currentTime,
			JsonData:    jsonData,
		})
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"nodes":   outputNodes,
	})
}

func handleConfigAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method == http.MethodGet {
		configMutex.RLock()
		defer configMutex.RUnlock()
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"config":  appConfig,
		})
		return
	}

	if r.Method == http.MethodPost {
		var newConfig Config
		if err := json.NewDecoder(r.Body).Decode(&newConfig); err != nil {
			http.Error(w, `{"success":false,"message":"Payload JSON tidak valid"}`, http.StatusBadRequest)
			return
		}

		rawBytes, err := json.MarshalIndent(newConfig, "", "  ")
		if err != nil {
			http.Error(w, `{"success":false,"message":"Gagal memformat JSON"}`, http.StatusInternalServerError)
			return
		}

		if err := os.WriteFile("config.json", rawBytes, 0644); err != nil {
			http.Error(w, `{"success":false,"message":"Gagal menulis file config.json"}`, http.StatusInternalServerError)
			return
		}

		configMutex.Lock()
		appConfig = newConfig
		configMutex.Unlock()

		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": "Konfigurasi berhasil disimpan",
		})
		return
	}

	http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
}