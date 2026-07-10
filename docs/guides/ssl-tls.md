# SSL / TLS setup

By default, the PatchBase server listens on plain HTTP. For anything beyond local development, you should enable HTTPS.

## Using the built-in TLS support

PatchBase can terminate TLS directly. Enable it in `config.yaml`:

```yaml
ssl:
  enabled: true
  certificate_file: "/etc/patchbase-server/cert.pem"
  key_file: "/etc/patchbase-server/key.pem"
```

The certificate and key files must exist and be readable by the server process. The server validates this at startup — if either file is missing or is a directory, it will refuse to start.

### Getting a certificate

For a self-signed certificate (testing only):

```bash
openssl req -x509 -newkey rsa:4096 -keyout key.pem -out cert.pem \
  -days 365 -nodes -subj "/CN=localhost"
```

For production, use [Let's Encrypt](https://letsencrypt.org/) or your organization's CA.

## Using a reverse proxy

For most deployments, putting PatchBase behind a reverse proxy (Nginx, Caddy, Traefik) is the simplest approach. The proxy handles TLS termination and forwards to PatchBase on port 5199.

### Nginx example

```nginx
server {
    listen 443 ssl http2;
    server_name patchbase.example.com;

    ssl_certificate /etc/ssl/patchbase/cert.pem;
    ssl_certificate_key /etc/ssl/patchbase/key.pem;

    location / {
        proxy_pass http://127.0.0.1:5199;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    # WebSocket support for live dashboard updates
    location /api/v1/ws {
        proxy_pass http://127.0.0.1:5199;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
    }
}
```

### Caddy example

```caddyfile
patchbase.example.com {
    reverse_proxy localhost:5199
}
```

Caddy handles TLS automatically with Let's Encrypt.

## Agent configuration with TLS

When the server uses HTTPS, agents need to trust the certificate. If you're using a well-known CA (Let's Encrypt, etc.), no extra configuration is needed.

For self-signed certificates or private CAs, pass the CA bundle during enrollment:

```bash
patchbase-agent enroll https://patchbase.example.com pb_reg_token \
  --ca-cert /path/to/ca-bundle.pem
```

The agent stores this path in its config and uses it for all subsequent sync calls.