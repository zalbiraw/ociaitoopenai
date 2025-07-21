# OCI AI to OpenAI Plugin

Traefik plugin that transforms OpenAI API requests to OCI GenAI format.

## Installation

### Static Configuration

```yaml
experimental:
  plugins:
    ociaitoopenai:
      moduleName: github.com/zalbiraw/ociaitoopenai
      version: v0.0.1
```

## Configuration

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `compartmentId` | string | `""` | OCI compartment ID where GenAI service is located. Required. |
| `region` | string | `""` | OCI region where GenAI service is located. Required. |

## Usage

The plugin intercepts OpenAI requests and transforms them to OCI GenAI format.

### Supported Endpoints

- `POST /chat/completions` → `POST /20231130/actions/chat`
- `GET /models` → `GET /20231130/models`

### Request Flow

1. Client sends OpenAI request to `/chat/completions` or `/models`
2. Plugin sets host to `generativeai.{region}.oci.oraclecloud.com`
3. Plugin transforms request from OpenAI format to OCI format
4. Request forwarded to OCI GenAI service
5. Plugin transforms OCI response back to OpenAI format
6. Client receives response in OpenAI format

### Models Endpoint

- Passes through all query parameters
- Defaults `capability=CHAT` if not specified
- Always adds required `compartmentId`