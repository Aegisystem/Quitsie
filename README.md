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

Pruebas: `cd api && go test ./...`, `cd web && npm test` y `cd web && npm run typecheck`.

## Código generado

`openapi.yaml` es el contrato entre cliente y servidor. Después de cambiarlo, o de cambiar las consultas en `api/internal/store/queries/`:

```bash
cd api && go generate ./...   # oapi-codegen (httpapi) y sqlc (store)
cd web && npm run generate    # tipos del cliente
```

Las migraciones son archivos `api/migrations/NNNNN_descripcion.sql` con secciones `-- +goose Up` y `-- +goose Down`. El servidor las aplica al arrancar.

## Despliegue

Quitsie corre en una VM gratuita de Google Cloud detrás de Cloudflare ([ADR 0007](docs/adr/0007-go-nativo-en-vm-gratuita.md)). El primer despliegue, con los pasos que solo puede hacer su dueño (dominio, cuentas, tokens), lo guía un asistente interactivo:

```bash
bash deploy/first-deploy-wizard.sh
```

Después, cada push a `main` que pase la CI se despliega solo (`.github/workflows/deploy.yml`). Los secretos solo viven en la VM, en `/etc/quitsie/*.env`, y en los secretos de GitHub Actions. La instalación, el despliegue continuo, la operación y la prueba de restauración del respaldo están en [docs/despliegue.md](docs/despliegue.md).
