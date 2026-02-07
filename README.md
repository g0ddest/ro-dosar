# 🇷🇴 RO-DOSAR

[![Test](https://github.com/g0ddest/ro-dosar/actions/workflows/test.yml/badge.svg)](https://github.com/g0ddest/ro-dosar/actions/workflows/test.yml)
[![Build](https://github.com/g0ddest/ro-dosar/actions/workflows/docker.yml/badge.svg)](https://github.com/g0ddest/ro-dosar/actions/workflows/docker.yml)
[![Go Version](https://img.shields.io/badge/Go-1.23-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

**RO-DOSAR** is a service for checking Romanian citizenship application status. It parses data from the official [cetatenie.just.ro](https://cetatenie.just.ro) website, stores it in PostgreSQL, and provides a modern REST API and web interface.

![Web Interface Preview](https://img.shields.io/badge/UI-Tailwind_CSS-38B2AC?style=flat&logo=tailwindcss)
![Alpine.js](https://img.shields.io/badge/JS-Alpine.js-8BC0D0?style=flat&logo=alpine.js)

## ✨ Features

- **Automated PDF Parsing** — Downloads and parses PDF documents from cetatenie.just.ro
- **JavaScript Protection Bypass** — Uses headless Chrome (chromedp) to handle anti-bot protection
- **Temporal Workflows** — Reliable, resumable parsing workflows with Temporal.io
- **Modern Web UI** — Beautiful SPA with dark mode, RO/EN languages, Tailwind CSS
- **REST API** — JSON API for document lookup
- **Legal Information** — Displays exact legal text from Romanian Citizenship Law (Legea nr. 21/1991)
- **GDPR Compliant** — No personal data collection, no tracking cookies

## 🏗 Architecture

```
ro-dosar/
├── cmd/
│   ├── server/          # Main server (API + Temporal worker)
│   └── parser/          # Parser CLI (starts parsing workflow)
├── internal/
│   ├── domain/          # Domain layer (entities, value objects)
│   ├── repository/      # Repository interfaces
│   ├── infrastructure/  # Implementations
│   │   ├── postgres/    # PostgreSQL repositories
│   │   ├── http/        # HTTP client + Browser client (chromedp)
│   │   └── pdf/         # PDF text extractor (pdftotext)
│   ├── workflow/        # Temporal workflows
│   ├── activity/        # Temporal activities
│   ├── api/             # REST API handlers
│   └── web/             # Web interface (SPA)
├── migrations/          # SQL migrations
├── pkg/parser/          # Reusable HTML/PDF parsing
└── docker/              # Docker configuration
```

## 🚀 Quick Start

### Prerequisites

- Go 1.23+
- Docker & Docker Compose
- `pdftotext` (from poppler-utils)
- Chrome/Chromium (for headless browser)

### Using Docker Compose (Recommended)

```bash
# Clone the repository
git clone https://github.com/yourusername/ro-dosar.git
cd ro-dosar

# Start all services
docker-compose up -d

# Run parser to fetch data
docker-compose run --rm parser
```

Services will be available at:
- **Web UI**: http://localhost:8080
- **Temporal UI**: http://localhost:8088
- **Metrics**: http://localhost:9090

### Local Development

```bash
# Install system dependencies
# macOS:
brew install poppler

# Ubuntu/Debian:
sudo apt install poppler-utils chromium

# Start infrastructure
docker-compose up -d postgres temporal temporal-ui

# Run server
DATABASE_URL=postgres://postgres:postgres@localhost:5432/dosar \
TEMPORAL_HOST=localhost:7233 \
go run cmd/server/main.go

# Run parser (in another terminal)
TEMPORAL_HOST=localhost:7233 \
go run cmd/parser/main.go
```

## 📡 API

### Get Document

```bash
GET /api/v1/documents/{number}/{category}/{year}
```

**Example:**
```bash
curl http://localhost:8080/api/v1/documents/10435/A/2025
```

**Response:**
```json
{
  "documentNumber": "10435/A/2025",
  "registeredAt": "2025-01-15",
  "category": {
    "code": "ART_8",
    "name": "Article 8",
    "nameRO": "Articolul 8",
    "description": "(1) Cetățenia română se poate acorda..."
  },
  "term": "2025-06-15",
  "appointments": [
    {
      "date": "2025-03-20",
      "time": "10:30",
      "type": "INVITATION"
    }
  ]
}
```

### Health Checks

```bash
GET /health  # Health check (on metrics port 9090)
GET /ready   # Readiness check
GET /metrics # Prometheus metrics
```

## 📋 Document Categories

The service tracks applications under different articles of the Romanian Citizenship Law (Legea nr. 21/1991):

| Code | Article |
|------|---------|
| `ART_8` | Article 8 |
| `ART_8_1` | Article 8¹ |
| `ART_8_2` | Article 8² |
| `ART_10` | Article 10 |
| `ART_11` | Article 11 |

## 🔧 Configuration

| Environment Variable | Default | Description |
|---------------------|---------|-------------|
| `DATABASE_URL` | `postgres://postgres:postgres@localhost:5432/dosar` | PostgreSQL connection string |
| `TEMPORAL_HOST` | `localhost:7233` | Temporal server address |
| `TEMPORAL_NAMESPACE` | `default` | Temporal namespace |
| `HTTP_PORT` | `8080` | HTTP server port |
| `METRICS_PORT` | `9090` | Metrics server port |
| `BASE_URL` | `https://cetatenie.just.ro` | Source website URL |

## 🔄 Temporal Workflows

1. **ParsePageWorkflow** — Fetches page with headless browser, extracts PDF links, spawns child workflows
2. **ProcessFileWorkflow** — Downloads PDFs, parses with pdftotext, extracts document data
3. **UpdateDocumentWorkflow** — Updates document records with audit logging
4. **ProcessAppointmentWorkflow** — Processes appointment/invitation data
5. **NotifyWorkflow** — Notification stub (for future integrations)

## 🛠 Tech Stack

- **Backend**: Go 1.23, Chi router
- **Database**: PostgreSQL 16
- **Workflow Engine**: Temporal.io
- **Browser Automation**: chromedp (headless Chrome)
- **PDF Parsing**: pdftotext (poppler-utils)
- **Frontend**: Alpine.js, Tailwind CSS (CDN)
- **Metrics**: Prometheus
- **CI/CD**: GitHub Actions

## 🔄 CI/CD

The project uses GitHub Actions for continuous integration and deployment:

### On Every Commit / PR
- **Tests** — Runs `go test` with race detection and coverage
- **Lint** — Runs `golangci-lint` for code quality
- **Build Check** — Verifies the project compiles

### On Push to `main`
- **Docker Build** — Builds and pushes two images to GitHub Container Registry:
  - `ghcr.io/g0ddest/ro-dosar/server:<commit-sha>`
  - `ghcr.io/g0ddest/ro-dosar/parser:<commit-sha>`
  - Also tagged as `latest`

## 🚀 Nomad Deployment

Nomad job files are located in `deploy/nomad/`:

```bash
# Deploy the main server
nomad job run deploy/nomad/ro-dosar.nomad

# Deploy the periodic parser (runs daily at 6:00 AM)
nomad job run deploy/nomad/ro-dosar-parser.nomad

# Run parser manually
nomad job dispatch ro-dosar-parser
```

## 📝 License

MIT License - see [LICENSE](LICENSE) for details.

## ⚠️ Disclaimer

This project is not affiliated with the Romanian government or the National Citizenship Authority. All data is sourced from publicly available information on [cetatenie.just.ro](https://cetatenie.just.ro). This service is provided as-is for informational purposes only.