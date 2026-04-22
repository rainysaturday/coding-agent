# Feature #071: HTTP Request Testing Tool

## Description
A comprehensive HTTP request testing tool that supports all standard HTTP methods, custom headers, authentication, request body handling, and response validation. Extends the existing `web_fetch` tool (which only supports GET requests) into a full-featured testing tool.

## Requirements

### Core Functionality
- [ ] Supports all HTTP methods: GET, POST, PUT, DELETE, PATCH, HEAD, OPTIONS
- [ ] Accepts URL as required parameter
- [ ] Supports custom headers via key-value pairs
- [ ] Supports request body for POST/PUT/PATCH methods (JSON, form-encoded, raw text)
- [ ] Supports authentication: Bearer token, Basic auth
- [ ] Follows redirects automatically (configurable max redirects, default 10)
- [ ] Configurable timeout (default: 30 seconds)

### Response Handling
- [ ] Returns HTTP status code
- [ ] Returns response headers
- [ ] Returns response body
- [ ] Returns response content type
- [ ] Returns response size in bytes
- [ ] Returns timing information (duration)

### Convenience Methods
- [ ] `get` - convenience method for GET requests
- [ ] `post` - convenience method for POST requests
- [ ] `put` - convenience method for PUT requests
- [ ] `delete` - convenience method for DELETE requests
- [ ] `patch` - convenience method for PATCH requests
- [ ] `head` - convenience method for HEAD requests (headers only, no body)
- [ ] `options` - convenience method for OPTIONS requests

### Validation
- [ ] Optional status code validation (e.g., expect 200, expect 2xx, expect 4xx)
- [ ] Optional content-type validation
- [ ] Optional JSON response parsing and validation

### Safety
- [ ] Blocks dangerous URL patterns (file://, etc.)
- [ ] Maximum response size limit (default 10MB)
- [ ] Zero external dependencies (Go stdlib only)

## Example Usage
```
http_request(url='https://api.example.com/users', method='POST', 
             headers={'Authorization': 'Bearer token123'},
             body='{"name": "new user"}')
```
