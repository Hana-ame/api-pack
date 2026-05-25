# Tools

A collection of utility libraries and helper tools used across the `api-pack` project.

## Categories

### 🌐 Networking & HTTP
- **`my_fetch`**: Advanced HTTP client with support for IPv6 address rotation, connection pooling, and custom transports.
- **`my_curl`**: A Go wrapper around the `curl` command for flexible request execution.
- **`header`**: Common HTTP header constants.

### 🚀 Gin Framework Extensions
- **`my_gin_handler`**: A set of reusable Gin handlers for common tasks like proxying, file serving, and timestamping.
- **`my_gin_middleware`**: Custom Gin middleware for CORS, request blocking, and proxying.

### 🛠 Data Structures & Utilities
- **`my_types`**: Custom implementation of data structures including B-Trees, Linked Lists, and specialized numeric types.
- **`my_streams` / `myiter` / `iterator`**: Functional-style stream and iterator utilities for collection processing.
- **`my_hash`**: Hashing utility functions.
- **`fastjson`**: Optimized JSON processing helpers.
- **`my_deadline`**: Utilities for managing operation deadlines.

### 🔌 Third-Party Integrations
- **`cloudflare`**: Integration for Cloudflare R2 storage.
- **`mastodon_client`**: A dedicated client for interacting with Mastodon APIs.
- **`liblib`**: Specialized utilities for the star3 service.

### 🗄 Database & Storage
- **`db`**: Database connection and management helpers (supporting `pq` and `pgx`).
- **`db_filehash`**: Utility for managing and verifying file hashes in a database.

### 💻 System & Debugging
- **`dlls`**: Examples and utilities for interacting with Windows DLLs (Export/Import/Load).
- **`debug`**: Internal debugging tools.
- **`mux_by_gpt`**: A custom multiplexer implementation.

## Usage

Most of these tools are designed as internal libraries. You can import them into your Go projects using the path `github.com/Hana-ame/api-pack/tools/<tool_name>`.
