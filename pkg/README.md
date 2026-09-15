# pkg

Reusable library packages shared across `api-pack`. (Previously `tools/`.)

## Categories

### Networking & HTTP
- **`fetch`**: Advanced HTTP client with IPv6 address rotation, connection pooling and custom transports (`fetch/v2` is the v2 API).
- **`curl`**: A Go wrapper around the `curl` command.
- **`header`**: Common HTTP header constants.

### Gin Framework Extensions
- **`ginutil/handler`**: Reusable Gin handlers (proxying, file serving, timestamps, …).
- **`ginutil/middleware`**: Gin middleware for CORS, request blocking and proxying.

### Data Structures & Utilities
- **`types`**: B-Trees, linked lists and specialized numeric types.
- **`streams` / `iter` / `iterator`**: Functional-style stream and iterator helpers.
- **`orderedmap`**: Insertion-ordered map.
- **`hash`**: Hashing helpers.
- **`fastjson`**: JSON processing helpers.
- **`deadline`**: Operation deadline helpers.
- **`utils`**: General-purpose helpers (headers, images, slices, timestamps, …).
- **`randomreader`**: Random reader helpers.

### Integrations
- **`r2`**: Cloudflare R2 storage.
- **`mastodon`**: Mastodon API client.
- **`liblib`**: star3 service utilities.
- **`openai`**: OpenAI-compatible client helpers.

### Database & Storage
- **`db/pq`**, **`db/pgx`**: Postgres drivers/helpers.
- **`db/filehash`**: File-hash database utility.
- **`sqlite`**: SQLite helpers.

### Debugging
- **`debug`**: Internal debugging helpers.

## Usage

Import with the module path, e.g.:

```go
import tools "github.com/Hana-ame/api-pack/pkg/utils"
```
