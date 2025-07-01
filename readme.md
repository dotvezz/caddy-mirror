# caddy-shadow

caddy-shadow is
a [Shadow Testing](https://microsoft.github.io/code-with-engineering-playbook/automated-testing/shadow-testing/) module
for Caddy 2, with full Caddyfile support.

# Features

- Request Mirroring
    - Default 1:1 mirroring
    - Configurable fractional mirroring **(⚠️ Planned)**
- Optional response timing metrics for Prometheus
    - Primary/Shadow Time to First Byte
    - Primary/Shadow Total Response Time
- Optional response comparison
    - Full response body comparison
    - Configurable selective comparison of JSON responses (powered by [itchyny/gojq](https://github.com/itchyny/gojq))
    - Configurable response header comparison
    - Response status comparison
- Reporting features **(⚠️ Planned)**

## Caddyfile

caddy-shadow fully supports Caddyfile as well as native JSON configuration.

### Example File

```caddyfile
{
    metrics
}

http://localhost:8080 {
    shadow {
        metrics shadow
        compare_body
        primary {
            reverse_proxy https://my-old-backend.com
        }
        shadow {
            reverse_proxy https://my-new-backend.com
        }
    }
}
```

### Caddyfile Options

| Name              | Description                                                  | Required? | Arguments            | Default |
|-------------------|--------------------------------------------------------------|-----------|----------------------|---------|
| `primary`         | The primary/vcurrent definition                              | Required  | Subroute             |         |
| `shadow`          | The shadow/vcurrent definition                               | Required  | Subroute             |         |
| `compare_status`  | Enables response-status comparison                           | Optional  |                      | false   |
| `compare_headers` | Enables response-status comparison                           | Optional  | List of header names | false   |
| `compare_body`    | Enables response-body comparison                             | Optional  |                      | false   |
| `compare_jq`      | Enables jq-based response comparison                         | Optional  | List of jq queries   |         |
| `no_log`          | Disables logging for mismatched responses                    | Optional  |                      | false   |
| `metrics`         | Enables metrics                                              | Optional  | Prefix/Namespace     |         |
| `Timeout`         | Set the maximum time that the module will wait for responses | Optional  | Duration string      | 30s     |

## Response Comparison

caddy-shadow provides a set of simple, optional features for comparing the shadowed response against the primary
response.

- Straight comparison of response body
- For JSON responses: JQ queries to select certain aspects of the JSON to compare, ignoring the rest of the result
- Comparison of response headers
- Comparison of response status codes

### Comparison Result Reporting

> [!NOTE]
> This is a planned feature that has not been implemented yet

