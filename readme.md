# caddy-mirror

> [!WARNING]
> This project's configuration and behavior should be considered unstable until it reaches its first `v1.0.0` release.
> Any change before `v1.0.0` may be a breaking change. If you choose to incorporate this into your xcaddy build,
> please use a tagged release and read release notes if you upgrade to a newer version.

caddy-mirror is a Request Mirroring and [Shadow Testing](https://microsoft.github.io/code-with-engineering-playbook/automated-testing/shadow-testing/) module for Caddy 2, with full Caddyfile support.

## Features

- Request Mirroring
    - Default 1:1 mirroring
    - Configurable fractional mirroring
- Optional response timing metrics for Prometheus
    - Primary/Shadow Time to First Byte
    - Primary/Shadow Total Response Time
- Optional shadow testing via response comparison
    - Full response body comparison
    - Configurable selective comparison of JSON responses (powered by [itchyny/gojq](https://github.com/itchyny/gojq))
    - Configurable response header comparison
    - Response status comparison
- Reporting features **(⚠️ Planned)**

### Feature Wishlist (Feedback and ideas welcome!)

In no particular order, the following feature goals are being actively considered as development moves forward, before
a `v1.0.0` release.

- Decoding compressed response bodies for comparison
- Low-overhead request multiplexing
  - Currently, the request is buffered and copied before sending to the primary and secondary handlers. This introduces
    latency and increases memory usage. 
- Low-overhead response body comparison
  - Currently, if response body comparison is enabled, this project buffers responses and compares them as `[]byte`.
  - Ideally, we'd be able to do (at least optionally) perform direct comparisons as the response is streamed, without
    buffering
- Reporting for response comparisons (matches, mismatches, etc)
  - Would love to get feedback on how to best make reporting available in your workflows. Some ideas are...
    - Response headers (`X-Shadow-Mismatch: true`, etc)
    - Messages over a configurable message queue (Kafka, SQS, etc)
    - Some companion API service that can run separately from your Caddy server and collate reports
- Optional blocking rules
- Tests and benchmarks to help users evaluate the safety and performance implications of using this module.

## Building with `xcaddy`

As with all Caddy plugin modules, you can use the [`xcaddy`](https://github.com/caddyserver/xcaddy) tool to compile 
Caddy with this plugin included.

```sh
> xcaddy build --with github.com/dotvezz/caddy-mirror
```

## Caddyfile

caddy-mirror fully supports Caddyfile as well as native JSON configuration.

### Example File

```caddyfile
{
    metrics
}

http://localhost:8080 {
    mirror {
        metrics shadow
        compare_body
        primary {
            reverse_proxy https://my-old-backend.com
        }
        secondary {
            reverse_proxy https://my-new-backend.com
        }
    }
}
```

### Caddyfile Options

| Name                | Description                                               | Required? | Arguments            | Default |
|---------------------|-----------------------------------------------------------|-----------|----------------------|---------|
| `primary`           | The primary handler definition                            | Required  | Subroute             |         |
| `secondary`         | The secondary handler definition                          | Required  | Subroute             |         |
| `mirror_rate`       | Rate of requests which should be mirrored (-1 to disable) | Optional  | Percentage           | 100%    |
| `compare_status`    | Enables response-status comparison                        | Optional  |                      | false   |
| `compare_headers`   | Enables response-status comparison                        | Optional  | List of header names | false   |
| `compare_body`      | Enables response-body comparison                          | Optional  |                      | false   |
| `compare_jq`        | Enables jq-based response comparison                      | Optional  | List of jq queries   |         |
| `no_log`            | Disables logging for mismatched responses                 | Optional  |                      | false   |
| `metrics`           | Enables metrics                                           | Optional  | Prefix/Namespace     |         |
| `secondary_timeout` | Set the maximum time to wait for the mirroed request      | Optional  | Duration string      | 30s     |

## Response Comparison

> [!NOTE]
> There are currently a few points to consider for response comparison.
> - Response body comparisons are only possible for uncompressed responses.
>   - One goal of the project is to support decompressing responses, but I want to get robust benchmarks in place
>     before we do this.
> - If comparison is enabled, responses are buffered and read as `[]byte`, which has some latency and memory
>   implications, especially for large responses.
>   - Probably not an issue for most JSON APIs.
>   - One goal of the project is to establish benchmarks which set realistic expectations for anyone evaluating this
>     as part of their Caddy deployment.
> - In order to minimize overall latency experienced by downstream/clients, the primary response is streamed down
>   as early as possible, without waiting for comparisons to complete.
>   - This means we start goroutines which may outlive the original request handler's goroutine (if your shadow is
>     slower than your primary). We take care not to leak them, but they can live roughly as long as the
>     `shadow_timeout` config value.

caddy-mirror provides a set of simple, optional features for comparing the shadowed response against the primary
response.

- Straight comparison of response body
- For JSON responses: JQ queries to select certain aspects of the JSON to compare, ignoring the rest of the result
- Comparison of response headers
- Comparison of response status codes

### Comparison Result Reporting

> [!NOTE]
> This is a planned feature that has not been implemented yet

