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

	_ "github.com/go-sql-driver/mysql"
)

type Config struct {
	Port  int        `json:"port"`
	DB    DBConfig   `json:"db"`
	Nodes []NodeItem `json:"nodes"`
}

type DBConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	Database string `json:"database"`
}

type NodeItem struct {
	ID           string                    `json:"id"`
	Name         string                    `json:"name"`
	Lat          float64                   `json:"lat"`
	Lng          float64                   `json:"lng"`
	StatusSource interface{}               `json:"status_source"`
	Metrics      map[string]MetricProperty `json:"metrics"`
}

type MetricProperty struct {
	Label  string `json:"label"`
	Unit   string `json:"unit"`
	Table  string `json:"table"`
	Column string `json:"column"`
}

var (
	appConfig   Config
	configMutex sync.RWMutex
	dbPool      *sql.DB
)

func main() {
	// 1. Baca file config.json
	configFile, err := os.ReadFile("config.json")
	if err != nil {
		log.Fatalf("[ERROR] Gagal membaca config.json: %v", err)
	}
	if err := json.Unmarshal(configFile, &appConfig); err != nil {
		log.Fatalf("[ERROR] Format config.json tidak valid: %v", err)
	}

	// 2. Format DSN MySQL
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?timeout=5s",
		appConfig.DB.User, appConfig.DB.Password,
		appConfig.DB.Host, appConfig.DB.Port, appConfig.DB.Database,
	)
	
	dbPool, err = sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("[ERROR] Driver DB Error: %v", err)
	}
	dbPool.SetMaxOpenConns(10)
	dbPool.SetMaxIdleConns(5)

	// 3. TES KONEKSI NYATA DENGAN PING
	fmt.Println("[INFO] Mencoba menghubungkan ke MySQL...")
	err = dbPool.Ping()
	if err != nil {
		log.Printf("\n❌ KONEKSI DATABASE GAGAL!\nDetail: %v\nPeriksa IP, Port, Username, Password, atau izin user di MySQL.\n\n", err)
	} else {
		fmt.Printf("✅ BERHASIL TERHUBUNG ke database MySQL: %s@%s:%d/%s\n\n", 
			appConfig.DB.User, appConfig.DB.Host, appConfig.DB.Port, appConfig.DB.Database)
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

func handleNodesAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	configMutex.RLock()
	nodesCopy := make([]NodeItem, len(appConfig.Nodes))
	copy(nodesCopy, appConfig.Nodes)
	configMutex.RUnlock()

	// Kumpulkan daftar tabel dan kolom
	tableCols := make(map[string]map[string]bool)
	for _, node := range nodesCopy {
		for _, m := range node.Metrics {
			if tableCols[m.Table] == nil {
				tableCols[m.Table] = make(map[string]bool)
			}
			tableCols[m.Table][m.Column] = true
		}
	}

	// Query data per tabel
	dbSnapshots := make(map[string]map[string]interface{})
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

	// Susun Output JSON
	type OutputMetric struct {
		Label  string                 `json:"label"`
		Value  interface{}            `json:"value"`
		Unit   string                 `json:"unit"`
		Source map[string]string      `json:"source"`
	}

	type OutputNode struct {
		ID          string                  `json:"id"`
		Name        string                  `json:"name"`
		Lat         float64                 `json:"lat"`
		Lng         float64                 `json:"lng"`
		Status      string                  `json:"status"`
		LastUpdated string                  `json:"last_updated"`
		JsonData    map[string]OutputMetric `json:"json_data"`
	}

	var outputNodes []OutputNode
	currentTime := time.Now().Format("02 Jan 2006 - 15:04:05 WIB")

	for _, node := range appConfig.Nodes {
		jsonData := make(map[string]OutputMetric)
		for k, m := range node.Metrics {
			val := dbSnapshots[m.Table][m.Column]
			jsonData[k] = OutputMetric{
				Label: m.Label,
				Value: val,
				Unit:  m.Unit,
				Source: map[string]string{
					"table":  m.Table,
					"column": m.Column,
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
			LastUpdated: currentTime,
			JsonData:    jsonData,
		})
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"nodes":   outputNodes,
	})
}