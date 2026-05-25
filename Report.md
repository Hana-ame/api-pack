# API-Pack Project Report

## 1. Project Overview
`api-pack` is a multi-purpose API proxy and service hub written in Go. Its primary function is to act as a reverse proxy for various third-party services, allowing users to bypass network restrictions, manage authentication (via cookies/API keys), and inject custom logic or scripts into the served content.

## 2. How the Code Works

### 2.1 Core Architecture
The project follows a modular structure where a central hub manages multiple independent proxy services.

- **Main Hub (`main.go`)**: The entry point of the application. It initializes a global HTTP client and launches various proxy services as concurrent goroutines based on environment variable configurations. It also hosts a general-purpose reverse proxy using the Gin framework.
- **Specialized Proxy Modules**: These are isolated packages (e.g., `/exhentai`, `/qwen`, `/shijima`) that implement specific logic for target websites or APIs. For example, the `exhentai` module performs HTML transformation and access control.
- **Utility Library (`/tools`)**: A comprehensive set of internal tools used across all modules, including:
    - `my_fetch`: A customized HTTP client for advanced request handling.
    - `my_gin_middleware`: Custom middleware for CORS and proxy management.
    - `utils`: General helpers for JSON, file I/O, and string manipulation.
    - `sqlite`: A wrapper for SQLite database operations.

### 2.2 Key Features
- **General Reverse Proxy**: Can forward any request to a destination specified via the `proxy_host` query parameter or `X-Host` header.
- **Specialized API Proxies**: Dedicated proxies for LLM providers (Groq, Siliconflow, Gemini, Qwen) that handle API keys and endpoint routing.
- **Content Transformation**: The `exhentai` proxy demonstrates the ability to modify response bodies on the fly, such as injecting JavaScript (`ex.js`), replacing domains, and stripping tracking beacons.
- **Access Control**: Implements whitelist-based path filtering and region blocking (via Cloudflare headers).

## 3. Setup and Configuration

### 3.1 Prerequisites
- Go environment installed.
- `.env` file for configuration (the project uses `godotenv`).

### 3.2 Environment Variables
The application's functionality is toggled and configured via environment variables:

| Variable | Description |
| :--- | :--- |
| `PROXY` | The address for the main Gin server (e.g., `:8080`). |
| `LOCAL_IP` | (Optional) Specific local IP to bind for outgoing requests. |
| `NYAA_PROXY` | Set to enable the Nyaa proxy. |
| `SUKEBEI_PROXY` | Set to enable the Sukebei proxy. |
| `GROQ_PROXY` | Set to enable Groq proxy; requires `GROQ_API_KEY`. |
| `SILICONFLOW_PROXY` | Set to enable Siliconflow proxy; requires `SILICONFLOW_API_KEY`. |
| `GEMINI_PROXY` | Set to enable Gemini proxy. |
| `SHIJIMA` | Set to enable the Shijima service. |
| `TWIMG` | Configuration for Twitter image proxy. |
| `PXIMG` | Configuration for Pixiv image proxy. |
| `QWEN_PROXY` | Set to enable Qwen proxy. |
| `EX_PROXY` | Set to enable exhentai proxy. |
| `EXHENTAI_PROXY_COOKIE` | Cookie required for exhentai proxy authentication. |

### 3.3 How to Run
1. Configure the `.env` file with the desired variables.
2. Build and run the application:
   ```bash
   go build -o api-pack main.go
   ./api-pack
   ```

## 4. Directory Structure

- `/` : Root directory containing the main entry point and core proxy definitions.
- `/exhentai` : Specialized proxy for exhentai.org including content injection and access control.
- `/pastejson` : A JSON storage/sharing service implementation.
- `/qwen` : Proxy for Qwen LLM.
- `/R2` : Cloudflare R2 integration tools.
- `/shijima` : Specialized bot/interaction handler.
- `/tools` : Shared utility libraries (Database, Fetch, Middlewares, etc.).
- `/scripts` : Maintenance and helper bash scripts.
