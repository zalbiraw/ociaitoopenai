# OCI AI to OpenAI Plugin

Traefik plugin that transforms OpenAI API requests to Oracle Cloud Infrastructure (OCI) GenAI format. This plugin enables OpenAI-compatible clients to work seamlessly with OCI's Generative AI service.

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

| Parameter | Type | Default | Required | Description |
|-----------|------|---------|----------|-------------|
| `compartmentId` | string | - | Yes | OCI compartment ID where GenAI service is located. |
| `region` | string | - | Yes | OCI region where GenAI service is located (e.g., `"us-chicago-1"`). |

## Usage

### Dynamic Configuration

```yaml
http:
  middlewares:
    openai-to-oci:
      plugin:
        ociaitoopenai:
          compartmentId: "ocid1.compartment.oc1..aaaaaaaa..."
          region: "us-chicago-1"
  routers:
    openai-api:
      rule: "Host(`openai.example.com`)"
      service: oci-genai
      middlewares:
        - openai-to-oci
```

### Supported Endpoints

- `POST /chat/completions` → `POST /20231130/actions/chat`
- `GET /models` → `GET /20231130/models`

### Request Flow

1. Client sends OpenAI request to `/chat/completions` or `/models`
2. Plugin transforms request from OpenAI format to OCI GenAI format
3. Plugin updates URL path and scheme for OCI GenAI endpoints
4. Request forwarded to next middleware (typically authentication)
5. Plugin transforms OCI response back to OpenAI format
6. Client receives response in OpenAI format

### Models Endpoint

- Passes through all query parameters
- Defaults `capability=CHAT` if not specified
- Always adds required `compartmentId`

## Integration with OCI Auth

This plugin is designed to work with the `ociauth` plugin for authentication:

```yaml
http:
  middlewares:
    openai-to-oci:
      plugin:
        ociaitoopenai:
          compartmentId: "ocid1.compartment.oc1..aaaaaaaa..."
          region: "us-chicago-1"
    oci-auth:
      plugin:
        ociauth:
          serviceName: "generativeai"
          region: "us-chicago-1"
  routers:
    openai-api:
      rule: "Host(`openai.example.com`)"
      service: oci-genai
      middlewares:
        - openai-to-oci  # Transform first
        - oci-auth       # Then authenticate
```

**Important**: The `ociaitoopenai` plugin should be applied before the `ociauth` plugin in the middleware chain.