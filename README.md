# SCADA Map Monitoring System

Aplikasi pemantauan visual berbasis web interaktif untuk memetakan titik distribusi dan telemetri fasilitas (seperti WTP, Reservoir, dan DMA) secara _real-time_. Dibangun menggunakan **Go (Golang)** untuk sisi *backend* dan **Leaflet.js** untuk antarmuka *frontend*.

Sistem ini dirancang untuk dapat beroperasi secara **Full Offline** (Air-gapped Network) dan mendukung dua sumber data secara bersamaan: **MySQL Database** dan **MQTT Broker**.

---

## 🌟 Fitur Utama

- **Peta Spasial Full Offline**: Mendukung penggunaan *Local Tile Server* (MBTiles) sehingga peta satelit/layout tetap muncul tanpa koneksi internet.
- **Dual Data Source**: 
  - Mendukung pembacaan *snapshot* data terakhir dari tabel **MySQL**.
  - Mendukung *subscribe* data metrik _real-time_ dari banyak **MQTT Broker** sekaligus (*Multi-connection*).
- **Watchdog Timer (Status Timeout)**: Deteksi otomatis alat mati/koneksi terputus. Jika data sensor tidak masuk selama X detik, penanda di peta akan otomatis berubah menjadi "OFF" (merah).
- **Dynamic Status Parsing**: Mampu membaca status "ON/OFF" langsung dari nilai boolean (`true/false`) atau angka di payload MQTT.
- **Web-based Configuration UI**: Panel visual (`/config.html`) untuk menambah/mengubah lokasi node, metrik data, dan manajemen koneksi broker MQTT tanpa perlu *restart* server atau *coding*.

---

## 📂 Struktur Proyek

```text
scada-map-project/
|-- static/                 # Aset Frontend statis (Offline)
|   |-- css/
|   |   |-- leaflet.css
|   |-- images/             # Ikon marker bawaan Leaflet
|   |   |-- layers-2x.png
|   |   |-- layers.png
|   |   |-- marker-icon-2x.png
|   |   |-- marker-icon.png
|   |   |-- marker-shadow.png
|   |-- js/
|       |-- leaflet.js
|-- tiles/                  # Folder untuk meletakkan file peta offline (.mbtiles)
|   |-- .gitkeep
|-- .gitignore
|-- Dockerfile              # Multi-stage build untuk mengkompilasi Go
|-- README.md
|-- config.example.json     # Template konfigurasi awal
|-- config.html             # Antarmuka dengan panel kontrol
|-- docker-compose.yml      # Orkestrasi container (App & TileServer)
|-- go.mod                  # Dependensi Go module
|-- go.sum
|-- index.html              # Antarmuka hanya Peta SCADA (tanpa kontrol)
|-- main.go                 # Backend Server (HTTP, MySQL, MQTT logic)
```

---

## 💻 Persyaratan Sistem (Requirements)

- **Docker Engine** & **Docker Compose** terinstal.
- File Peta Offline format `.mbtiles` (Misal: hasil _export_ QGIS atau MOBAC) jika ingin menggunakan latar peta *offline*.
- MySQL Server atau MQTT Broker (lokal maupun *cloud*) sebagai sumber data telemetri.

---

## 🚀 Cara Deploy dengan Docker (Langkah-demi-Langkah)

### 1. Persiapkan Konfigurasi
Aplikasi membutuhkan file `config.json` agar dapat berjalan. Gandakan template yang sudah disediakan:
```bash
cp config.example.json config.json
```
*(Anda bisa mengedit file ini secara manual untuk mengatur koneksi database dan broker, atau mengaturnya nanti melalui antarmuka web).*

### 2. Persiapkan Peta Offline (Opsional tapi Direkomendasikan)
Jika Anda menggunakan Tile Server, letakkan file `.mbtiles` Anda ke dalam folder `tiles/`.
Misalnya: `tiles/semanan_map.mbtiles`. 
*(Pastikan nama file dan setting URL di `index.html` cocok).*

### 3. Build & Jalankan Container
Jalankan perintah Docker Compose berikut dari direktori root proyek:
```bash
docker compose up -d --build
```
Proses ini akan:
1. Mengunduh pustaka Go dan melakukan kompilasi `main.go`.
2. Menjalankan *backend* Go dan menyajikan file statis di Port `8080`.
3. Menjalankan layanan `mbtileserver` (jika dikonfigurasi di compose) di Port `8081`.

### 4. Periksa Log (Troubleshooting)
Untuk memastikan aplikasi berhasil terhubung ke MySQL atau MQTT Broker:
```bash
docker compose logs -f
```

---

## 🌐 Akses Aplikasi

Setelah container berstatus *Running*, buka browser dan akses URL berikut:

- **Dashboard Peta Utama**: [`http://localhost:8080`](http://localhost:8080)
- **Panel Konfigurasi Node & MQTT**: [`http://localhost:8080/config`](http://localhost:8080/config)

---

## ⚙️ Petunjuk Konfigurasi Tambahan

**1. Mengatur MQTT Wildcard**
Untuk menghemat *resource*, Anda bisa menggunakan *wildcard* (`#`) pada kolom **Global Topic** di menu Konfigurasi Koneksi MQTT (Misal: `wtp/telemetry/#`). Server otomatis hanya membuka 1 koneksi koneksi TCP untuk membaca seluruh sub-topik. Topik spesifik per-node kemudian didefinisikan pada masing-masing konfigurasi Node.

**2. Watchdog / Status Timeout**
Pada panel pengaturan Node, isi nilai **"Timeout Mati (Detik)"** dengan angka (misalnya `15`). Jika koneksi sensor putus atau tidak ada *publish* data MQTT terbaru lebih dari 15 detik, aplikasi akan secara otomatis mewarnai marker node menjadi MERAH (OFF).

**3. Format Payload MQTT yang Didukung**
Secara *default*, *backend* mendeteksi format array JSON standar SCADA/Historian berikut:
```json
{
  "Records": [
    {"TagName": "pH_AirBaku", "Value": 7.6},
    {"TagName": "Flow_InWTP", "Value": 182.1},
    {"TagName": "Status_Pompa", "Value": true}
  ],
  "Timestamp": "2026-11-12T08:59:02.6267180"
}
```
Untuk mengaitkan data, masukkan isi `TagName` ke dalam kolom **Column/JSON Tag** di antarmuka Web UI.