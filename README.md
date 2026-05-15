# dz-pixel-email

Email open pixel + click redirect tracker.

- `GET /o/{emailID}` → returns 1×1 GIF, logs `email_open` event
- `GET /c/{emailID}?u={destURL}` → logs `email_click`, redirects to destURL

`destURL` can be plain or base64-encoded (URL-safe). Events go to the same Redis Stream as the collector, processed by `dz-pixel-worker`.

## Environment Variables

| Variable | Default | Purpose |
|---|---|---|
| `PORT` | `8080` | HTTP port |
| `REDIS_HOST` | `localhost` | Redis host |
| `REDIS_PASSWORD` | — | Redis password |

## Example email HTML

```html
<!-- Open tracking pixel (place in email body) -->
<img src="https://email.pixel.example.com/o/abc123" width="1" height="1" alt="">

<!-- Tracked link -->
<a href="https://email.pixel.example.com/c/abc123?u=https%3A%2F%2Fexample.com%2Foffer">Click here</a>
```
