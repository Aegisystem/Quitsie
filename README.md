# Quitsie

Aplicación para registrar gastos compartidos y saber quién le debe cuánto a quién. Es una PWA servida, junto con la API, por un único binario Go.

- Vocabulario: [CONTEXT.md](CONTEXT.md)
- Arquitectura: [docs/arquitectura.md](docs/arquitectura.md)
- Reglas: [docs/reglas-de-negocio.md](docs/reglas-de-negocio.md)
- Decisiones: [docs/adr/](docs/adr/)

## Desarrollo

Requisitos: Go 1.26 y Node 22.

```bash
cd web && npm install && npm run build   # compila la PWA en api/cmd/server/dist
cd api && go run ./cmd/server            # http://127.0.0.1:8080
```

Para trabajar en la interfaz con recarga en caliente, deja el servidor Go corriendo y en otra terminal ejecuta `cd web && npm run dev`. Vite reenvía `/api` al puerto 8080.

Pruebas: `cd api && go test ./...` y `cd web && npm run typecheck`.

## Código generado

`openapi.yaml` es el contrato entre cliente y servidor. Después de cambiarlo, o de cambiar las consultas en `api/internal/store/queries/`:

```bash
cd api && go generate ./...   # oapi-codegen (httpapi) y sqlc (store)
cd web && npm run generate    # tipos del cliente
```

Las migraciones son archivos `api/migrations/NNNNN_descripcion.sql` con secciones `-- +goose Up` y `-- +goose Down`. El servidor las aplica al arrancar.

## Despliegue

[deploy/](deploy/) tiene los borradores de las unidades systemd (aplicación, Litestream y `cloudflared`) y de `litestream.yml`. Los secretos solo viven en la VM, en `/etc/quitsie/*.env`. Cada archivo indica dónde se instala.
