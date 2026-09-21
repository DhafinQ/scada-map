
# SCADA Map Monitoring System

Aplikasi pemantauan visual berbasis web interaktif untuk memetakan titik distribusi dan telemetri fasilitas (seperti WTP dan Reservoir) secara langsung dari database MySQL lokal. Dibangun menggunakan Go untuk sisi backend dan Leaflet.js pada antarmuka web.

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

- **Go**: Versi 1.18 atau lebih baru
- **MySQL Database**: Server database lokal aktif
- Browser modern (Chrome, Edge, Firefox, Safari)

---

## Struktur Proyek

```text
├── index.html     # Tampilan peta SCADA, styling CSS, dan client fetch script
├── main.go        # Server HTTP Go, handler API, dan query builder MySQL
├── config.json    # Konfigurasi database MySQL, port, dan daftar node telemetri
└── go.mod         # Berkas dependensi Go modul

```

---

## Persiapan Database

Aplikasi mengambil data telemetri terakhir (`ORDER BY id DESC LIMIT 1`) berdasarkan konfigurasi mapping tabel dan kolom di `config.json`. Pastikan tabel terkait memiliki kolom auto-increment `id` dan kolom-kolom yang didefinisikan (contoh: tabel `Flow`, `Level`, `Analyzer`, `250_FTHrs`, `300_FTHrs`).

---

## Konfigurasi

Sesuaikan koneksi database MySQL pada berkas `config.json`:

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

---

## Instalasi dan Menjalankan Proyek

1. **Inisialisasi Proyek (Github Clone)**:
```bash
git clone https://github.com/DhafinQ/scada-map.git
cd scada-map
```


2. **Unduh Driver MySQL**:
```bash
go get [github.com/go-sql-driver/mysql](https://github.com/go-sql-driver/mysql)

```


3. **Jalankan Aplikasi**:
```bash
go run main.go

```


4. **Akses Antarmuka**:
Buka peramban dan kunjungi:
```text
http://localhost:8080

```



---

## Dokumentasi API Endpoint

| Endpoint | Method | Deskripsi |
| --- | --- | --- |
| `/` | `GET` | Menyajikan berkas antarmuka statis (`index.html`) |
| `/api/nodes` | `GET` | Mengambil data koordinat serta telemetri terbaru dari MySQL |
| `/api/config` | `GET` | Mengambil seluruh skema konfigurasi dari `config.json` |
| `/api/config` | `POST` | Memperbarui berkas `config.json` berdasarkan input modal editor |

```

```
