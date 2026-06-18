# 🌸 RoseDB Phân Tán – Hệ Quản Trị CSDL Khả Năng Chịu Lỗi & Mở Rộng

<div align="center">

[![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://golang.org/)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

**[English](README.md) · [简体中文](README-CN.md) · Tiếng Việt**

*Bài tập lớn môn Phát triển Hệ thống Phân tán – Nhóm 6 – Đại học Phenikaa*

</div>

---

## 📖 Giới thiệu

Dự án này mở rộng **RoseDB** — một cơ sở dữ liệu Key-Value nhúng (embedded) mã nguồn mở, hiệu năng cao, xây dựng trên mô hình **Bitcask** — thành một **hệ quản trị cơ sở dữ liệu phân tán** hoàn chỉnh với các tính năng:

- ✅ **Sao chép dữ liệu** (Master-Slave Replication)
- ✅ **Tự động chịu lỗi** (Automated Failover – thăng cấp Slave lên Master trong < 6 giây)
- ✅ **Phân mảnh dữ liệu** (Consistent Hashing Sharding)
- ✅ **Di cư dữ liệu nóng** (Zero-downtime Data Migration)
- ✅ **Giao diện Dashboard** theo dõi trạng thái cụm theo thời gian thực
- ✅ **Công cụ dòng lệnh CLI** để tương tác với hệ thống

---

## 🏗️ Kiến trúc hệ thống

```
┌─────────────────┐
│     Client      │
│  (CLI / HTTP)   │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  Proxy Server   │  ← Cổng 9000 (Consistent Hashing Router)
│   :9000         │
└────────┬────────┘
         │
    ┌────┴────┐
    ▼         ▼
┌───────┐ ┌───────┐
│MASTER │◄─►│ SLAVE │  ← Heartbeat (2s)
│ :8080 │   │ :8081 │  ← Replication (/sync)
└───────┘   └───────┘
```

**Luồng hoạt động:**
1. Client gửi yêu cầu đến Proxy (cổng 9000).
2. Proxy dùng Consistent Hashing để định tuyến key đến đúng cụm node.
3. Yêu cầu **Ghi (PUT/DELETE)** → Master (cổng 8080).
4. Master đồng bộ dữ liệu ngầm sang Slave qua `/sync`.
5. Slave giám sát sức khỏe Master qua Heartbeat mỗi 2 giây.
6. Nếu Master lỗi → Slave **tự động thăng cấp** thành Master mới.

---

## 🚀 Cài đặt và Chạy Project

### Yêu cầu môi trường

| Thành phần | Phiên bản |
|---|---|
| Go | >= 1.21 |
| Git | Bất kỳ |
| OS | Linux / macOS / Windows |

### Bước 1: Clone và cài đặt phụ thuộc

```bash
git clone https://github.com/quaneluscore123/distributed-system-projects.git
cd distributed-system-projects

# Tải về các gói thư viện cần thiết
go mod tidy
```

### Bước 2: Biên dịch mã nguồn

```bash
# Biên dịch Server node
go build -o server-node.exe ./cmd/server

# Biên dịch Proxy Server
go build -o proxy-server.exe ./cmd/proxy

# Biên dịch CLI Client
go build -o cli-client.exe ./cmd/cli
```

> **Lưu ý Windows:** Có thể dùng `.exe`, trên Linux/macOS bỏ phần `.exe`.

---

## 🖥️ Hướng dẫn chạy Cụm Node (Cluster)

> **Thứ tự khởi chạy bắt buộc:** Master → Slave → Proxy

### Terminal 1 – Khởi chạy Node Master (Cổng 8080)

```bash
./server-node.exe \
  -port 8080 \
  -dbpath ./db_master \
  -slaves "http://localhost:8081" \
  -sync-key "my-secret-key"
```

| Flag | Mô tả | Giá trị mặc định |
|---|---|---|
| `-port` | Cổng HTTP của node | `8080` |
| `-dbpath` | Thư mục lưu dữ liệu RoseDB | `/tmp/rosedb_server` |
| `-slaves` | Danh sách slave (phân cách bởi dấu phẩy) | *(rỗng)* |
| `-sync-key` | Mã bảo mật xác thực đồng bộ nội bộ | *(rỗng)* |

### Terminal 2 – Khởi chạy Node Slave (Cổng 8081)

```bash
./server-node.exe \
  -port 8081 \
  -dbpath ./db_slave \
  -master "http://localhost:8080" \
  -sync-key "my-secret-key"
```

| Flag | Mô tả |
|---|---|
| `-master` | URL của node Master để giám sát nhịp tim (Heartbeat) |

> Khi slave phát hiện master bị sập (3 lần lỗi liên tiếp / 6 giây), nó sẽ **tự động thăng cấp** thành Master mới mà không cần can thiệp thủ công.

### Terminal 3 – Khởi chạy Proxy Server (Cổng 9000)

```bash
./proxy-server.exe \
  -port 9000 \
  -nodes "http://localhost:8080,http://localhost:8081"
```

| Flag | Mô tả | Giá trị mặc định |
|---|---|---|
| `-port` | Cổng HTTP của Proxy | `9000` |
| `-nodes` | Danh sách các node trong cụm (phân cách bởi dấu phẩy) | *(bắt buộc)* |

---

## 💻 Sử dụng CLI Client

CLI Client kết nối đến Proxy (mặc định `http://localhost:8080`).

### Ghi dữ liệu (PUT)

```bash
./cli-client.exe -server http://localhost:9000 put ten "Nguyen Van A"
./cli-client.exe -server http://localhost:9000 put tuoi "21"
```

### Đọc dữ liệu (GET)

```bash
./cli-client.exe -server http://localhost:9000 get ten
# Output: Nguyen Van A

./cli-client.exe -server http://localhost:9000 get tuoi
# Output: 21
```

### Xóa dữ liệu (DELETE)

```bash
./cli-client.exe -server http://localhost:9000 delete ten
```

### Kiểm tra sức khỏe node (HEALTH)

```bash
# Kiểm tra Master
./cli-client.exe -server http://localhost:8080 health

# Kiểm tra Slave
./cli-client.exe -server http://localhost:8081 health
```

**Kết quả trả về:**
```json
{
  "status": "OK",
  "role": "MASTER",
  "uptime": "5m30s",
  "memory_alloc_mb": 3.21
}
```

---

## 🌐 Giao diện Dashboard quản trị

Mỗi node cung cấp một trang web Dashboard theo dõi trạng thái trực quan, tự cập nhật mỗi 2 giây.

Truy cập theo địa chỉ:

| Node | URL Dashboard |
|---|---|
| Master | http://localhost:8080/admin |
| Slave | http://localhost:8081/admin |

Dashboard hiển thị:
- Vai trò hiện tại của node (MASTER / SLAVE)
- Trạng thái hoạt động (OK / ERROR)
- Thời gian hoạt động (Uptime)
- Mức tiêu thụ RAM (MB)

---

## 📡 Tham khảo toàn bộ API Endpoints

| Endpoint | Method | Mô tả | Giới hạn quyền |
|---|---|---|---|
| `/put` | POST | Ghi một cặp key-value | Chỉ MASTER |
| `/get?key=...` | GET | Đọc giá trị theo key | MASTER & SLAVE |
| `/delete?key=...` | DELETE / POST | Xóa một key | Chỉ MASTER |
| `/health` | GET | Trả về trạng thái node (JSON) | Tất cả |
| `/keys` | GET | Liệt kê tất cả các key (JSON) | Tất cả |
| `/admin` | GET | Giao diện Dashboard HTML | Tất cả |
| `/sync` | POST | Đồng bộ dữ liệu nội bộ từ Master | Yêu cầu `X-Sync-Key` |

### Ví dụ gọi API trực tiếp bằng `curl`

```bash
# Ghi dữ liệu
curl -X POST http://localhost:9000/put -d "key=city&value=HaNoi"

# Đọc dữ liệu
curl http://localhost:9000/get?key=city

# Xóa dữ liệu
curl -X DELETE http://localhost:9000/delete?key=city

# Kiểm tra sức khỏe
curl http://localhost:8080/health
```

---

## 🔥 Thêm Node Nóng & Di cư Dữ liệu (Scale-out)

Khi cần mở rộng thêm node lưu trữ mà **không làm gián đoạn hệ thống**:

### Bước 1: Khởi động node mới (ví dụ cổng 8082)

```bash
./server-node.exe -port 8082 -dbpath ./db_node3
```

### Bước 2: Đăng ký node mới với Proxy qua API `/add-node`

```bash
curl -X POST http://localhost:9000/add-node \
  -d "node=http://localhost:8082"
```

Proxy sẽ **tự động chạy ngầm** quá trình di cư dữ liệu:
1. Quét toàn bộ keys trên các node cũ.
2. Tính toán keys nào thuộc phân vùng của node mới.
3. Chuyển dữ liệu: Đọc từ node cũ → Ghi sang node mới → Xóa ở node cũ.

Trong suốt quá trình này, hệ thống vẫn tiếp tục phục vụ client bình thường.

---

## 🧪 Chạy Unit Tests

```bash
# Chạy toàn bộ bộ kiểm thử
go test ./...

# Chạy tests riêng cho package Server (bao gồm Failover test)
go test ./server/... -v

# Chạy tests riêng cho package Sharding
go test ./sharding/... -v
```

---

## 📁 Cấu trúc thư mục dự án

```
distributed-system-projects/
├── cmd/
│   ├── server/         # Entrypoint khởi chạy node RoseDB
│   │   └── main.go
│   ├── proxy/          # Entrypoint khởi chạy Proxy Server
│   │   └── main.go
│   └── cli/            # Công cụ dòng lệnh CLI
│       └── main.go
├── server/
│   ├── server.go       # Logic HTTP Server & các API Endpoints
│   ├── replication.go  # Cơ chế đồng bộ dữ liệu Master-Slave
│   ├── failover.go     # Heartbeat Worker & tự động thăng cấp
│   ├── admin.go        # Giao diện Dashboard Web HTML
│   ├── server_test.go  # Unit tests cho Server
│   └── failover_test.go# Integration test cho Failover
├── sharding/           # Thuật toán Consistent Hashing
│   └── ...
├── go.mod
└── README-VI.md        # Tài liệu này
```

---

## 👥 Thành viên thực hiện – Nhóm 6

| Thành viên | Nhiệm vụ chính |
|---|---|
| Nguyễn Tuấn Anh | Code Failover, Integration Test, Slide & Báo cáo |
| Hoàng Minh Quân | HTTP Server, Replication, Graceful Shutdown |
| Đinh Xuân Tài | Consistent Hash Ring, Proxy Router, Data Migration |
| Lê Anh Duy | CLI Client, Web Admin Dashboard |

**Giảng viên hướng dẫn:** TS. Nguyễn Lệ Thu – Khoa CNTT, Đại học Phenikaa

---

## 📜 Giấy phép

Dự án sử dụng giấy phép **Apache License 2.0**. Xem chi tiết tại [LICENSE](LICENSE).
