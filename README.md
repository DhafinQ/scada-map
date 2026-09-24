
# SCADA Map Monitoring System

Aplikasi pemantauan visual berbasis web interaktif untuk memetakan titik distribusi dan telemetri fasilitas (seperti WTP dan Reservoir) secara langsung dari database MySQL. Dibangun menggunakan Go untuk sisi backend dan Leaflet.js pada antarmuka web.

---

## Fitur Utama

- **Peta Spasial Interaktif**: Visualisasi peta satelit resolusi tinggi menggunakan Leaflet.js dan label CartoDB.
- **Monitoring Data Real-Time**: Pembaruan berkala setiap 5 detik untuk metrik operasional (aliran air, jam kerja, level reservoir, turbidity, ph, klorin, dll.).
- **Live In-Place Marker Update**: Nilai metrik dan badge status pada popup diperbarui secara dinamis tanpa merusak sesi klik atau posisi peta.
- **Auto-Fit Viewport**: Otomatis menyesuaikan *zoom* dan fokus peta saat data node pertama kali dimuat.
- **Konfigurasi Visual**: Panel modal interaktif untuk menambah, mengubah, atau menghapus node koordinat serta memetakan kolom/tabel database langsung dari antarmuka web.
- **Konfigurasi Terpusat**: Pengaturan koneksi database dan skema metrik tersimpan otomatis di berkas `config.json`.

---

## Prasyarat Sistem

Pilih salah satu metode yang ingin digunakan:

- **Metode Docker (Direkomendasikan)**:
  - Docker Engine & Docker Compose
  - Akses ke MySQL Database (lokal di host atau remote)
- **Metode Standalone (Tanpa Docker)**:
  - Go (versi 1.20 atau lebih baru)
  - MySQL Database aktif

---

## Struktur Proyek

```text
scada-map-wika
├── docker-compose.yml   # Konfigurasi orkestrasi container Docker
├── Dockerfile           # Multi-stage container build Go & Alpine
├── config.example.json  # Template awal konfigurasi
├── config.json          # Konfigurasi aktif (database, node, metrik)
├── config.html          # Panel antarmuka manajemen konfigurasi
├── index.html           # Tampilan visual peta SCADA Leaflet
├── main.go              # Backend HTTP server dan MySQL query handler
├── go.mod               # Manajemen modul Go
└── README.md            # Dokumentasi proyek

```

---

## Persiapan Konfigurasi Database

1. Salin template konfigurasi jika belum memiliki `config.json`:
```bash
cp config.example.json config.json

```


2. Buka `config.json` dan sesuaikan koneksi database MySQL:
```json
{
  "port": 8080,
  "db": {
    "host": "127.0.0.1",
    "port": 3306,
    "user": "root",
    "password": "your_password",
    "database": "optix_logger"
  },
  "nodes": [ ... ]
}

```



> **Catatan Docker:** Jika server MySQL berjalan langsung di mesin host (bukan di dalam jaringan Docker), ubah `"host"` menjadi `"host.docker.internal"`.

---

## Menjalankan Proyek

### Cara 1: Menggunakan Docker Compose (Direkomendasikan)

1. **Jalankan container**:
```bash
docker compose up -d --build

```


2. **Periksa log aplikasi**:
```bash
docker compose logs -f

```


3. **Menghentikan container**:
```bash
docker compose down

```



Perubahan konfigurasi melalui web UI akan otomatis tersimpan di host karena berkas `config.json` di-mount menggunakan volume binding.

---

### Cara 2: Menjalankan Langsung (Go Local)

1. **Unduh dependensi**:
```bash
go mod download

```


2. **Jalankan server**:
```bash
go run main.go

```



---

## Akses Antarmuka

Buka peramban web dan kunjungi:

* **Peta SCADA**: `http://localhost:8080`
* **Editor Konfigurasi**: `http://localhost:8080/config.html`

---

## Dokumentasi API Endpoint

| Endpoint | Method | Deskripsi |
| --- | --- | --- |
| `/` | `GET` | Menyajikan antarmuka visual peta (`index.html`) |
| `/config.html` | `GET` | Menyajikan panel editor pengaturan |
| `/api/nodes` | `GET` | Mengambil data node beserta telemetri terbaru dari database |
| `/api/config` | `GET` | Mengambil seluruh konfigurasi aktif dari `config.json` |
| `/api/config` | `POST` | Menyimpan perubahan konfigurasi ke `config.json` |

```

```