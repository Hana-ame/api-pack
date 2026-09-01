# Params Query Parameter Usage

## Overview

The main proxy handler (`/*any`) now supports a `params` query parameter that accepts URL-safe base64-encoded JSON containing proxy configuration.

## Format

The `params` parameter should contain a URL-safe base64-encoded JSON object with the following fields:

```json
{
  "host": "example.com",
  "referer": "https://referrer.com",
  "scheme": "https",
  "origin": "https://origin.com"
}
```

### Fields

- **host** (string): The target host to proxy to
- **referer** (string): The Referer header value to send to the target
- **scheme** (string): The protocol scheme (http or https)
- **origin** (string): The Origin header value to send to the target

### Why URL-safe Base64?

URL-safe base64 uses `-` and `_` instead of `+` and `/`, making it safe to use directly in URLs without additional encoding. This avoids issues with URL encoding/decoding.

## Usage Examples

### Example 1: Basic Usage

```bash
# Original URL parameters
curl "http://localhost:8080/test/path?proxy_host=example.com&proxy_scheme=https"

# New params format
# First, create the JSON and encode it to URL-safe base64
echo -n '{"host":"example.com","scheme":"https"}' | base64 | tr '+/' '-_' | tr -d '='
# Output: eyJob3N0IjoiZXhhbXBsZS5jb20iLCJzY2hlbWUiOiJodHRwcyJ9

# Use the encoded string
curl "http://localhost:8080/test/path?params=eyJob3N0IjoiZXhhbXBsZS5jb20iLCJzY2hlbWUiOiJodHRwcyJ9"
```

### Example 2: Complete Parameters

```javascript
// JavaScript example
const params = {
  host: "api.example.com",
  referer: "https://myapp.com",
  scheme: "https",
  origin: "https://myapp.com"
};

// URL-safe base64 encoding function
function urlSafeBase64Encode(str) {
  return btoa(str)
    .replace(/\+/g, '-')
    .replace(/\//g, '_')
    .replace(/=/g, '');
}

const base64Params = urlSafeBase64Encode(JSON.stringify(params));
const url = `/test/path?params=${base64Params}&other=query&params=here`;

fetch(url);
```

### Example 3: Python Example

```python
import base64
import json
import requests

params = {
    "host": "api.example.com",
    "referer": "https://myapp.com",
    "scheme": "https",
    "origin": "https://myapp.com"
}

# URL-safe base64 encoding without padding
json_str = json.dumps(params)
base64_params = base64.urlsafe_b64encode(json_str.encode()).rstrip(b'=').decode()

url = f"http://localhost:8080/test/path?params={base64_params}"

response = requests.get(url)
```

### Example 4: Online Tool

You can use online tools to encode your JSON:
1. Create your JSON: `{"host":"example.com","scheme":"https"}`
2. Use a URL-safe base64 encoder (or regular base64 then replace `+` with `-`, `/` with `_`, and remove `=`)
3. Append to your URL as `?params=<encoded_value>`

## Priority

The parameters follow this priority order (highest to lowest):

1. Individual query parameters (`proxy_host`, `proxy_scheme`, etc.)
2. Headers (`X-Host`, `X-Scheme`, etc.)
3. **params JSON fields** (new feature)
4. Default values

This means you can still use the traditional individual parameters, and they will override the params JSON if both are provided.

## Behavior

- The `params` parameter is automatically removed from the forwarded request (not passed to the target server)
- Other query parameters and the path remain unchanged and are forwarded as-is
- If the JSON is malformed or base64 decoding fails, the params are silently ignored
- Empty or missing fields in the JSON are treated as if they weren't provided
- Supports both padded (`URLEncoding`) and unpadded (`RawURLEncoding`) URL-safe base64

## Benefits

1. **Cleaner URLs**: Instead of multiple query parameters, you have one compact parameter
2. **URL-safe**: No need for additional URL encoding, uses `-` and `_` instead of `+` and `/`
3. **Easier to generate programmatically**: Just create a JSON object and encode it
4. **Backward compatible**: All existing query parameters and headers still work
5. **Flexible**: Can provide any combination of host, referer, scheme, and origin
