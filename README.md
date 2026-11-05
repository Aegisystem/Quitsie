# Quitsie

Aplicación para registrar gastos compartidos y saber quién le debe cuánto a quién. Es una PWA servida, junto con la API, por un único binario Go.

- Vocabulario: [CONTEXT.md](CONTEXT.md)
- Arquitectura: [docs/arquitectura.md](docs/arquitectura.md)
- Reglas: [docs/reglas-de-negocio.md](docs/reglas-de-negocio.md)
- Decisiones: [docs/adr/](docs/adr/)

## Desarrollo

Requisitos: Go 1.26, Node 22 y Docker.

```bash
docker compose up -d                     # PostgreSQL en 127.0.0.1:5433, para el servidor y las pruebas
cd web && npm install && npm run build   # compila la PWA en api/cmd/server/dist
cd api && QUITSIE_ENV=development QUITSIE_MAIL=log go run ./cmd/server   # http://127.0.0.1:8080; los códigos de acceso salen en el log
```

Para trabajar en la interfaz con recarga en caliente, deja el servidor Go corriendo y en otra terminal ejecuta `cd web && npm run dev`. Vite reenvía `/api` al puerto 8080.

Pruebas (con el PostgreSQL de `compose.yaml` levantado; cada prueba crea su propia base): `cd api && go test ./...`, `cd web && npm test` y `cd web && npm run typecheck`.

Las pruebas de punta a punta (`web/e2e/`) usan Playwright con Chromium contra el binario real, con la PWA embebida. Cada ejecución arranca su propio servidor en un puerto libre de `quitsie-e2e.localhost`, con una base de PostgreSQL propia, así que no chocan con otro servidor que esté corriendo:

```bash
cd web && npx playwright install --only-shell chromium   # solo la primera vez
cd web && npm run e2e                                    # recompila la PWA y el binario, y corre las pruebas
```

Si ya compilaste el binario, `cd web && npx playwright test` las corre sin recompilar. Cuando una falla, su traza, su captura y el log del servidor quedan en `web/e2e/results/`, y la traza se abre con `npx playwright show-trace`. En la CI ese mismo contenido se guarda como el artefacto `e2e-traces`.

## Código generado

`openapi.yaml` es el contrato entre cliente y servidor. Después de cambiarlo, o de cambiar las consultas en `api/internal/store/queries/`:

```bash
cd api && go generate ./...   # oapi-codegen (httpapi) y sqlc (store)
cd web && npm run generate    # tipos del cliente
```

Las migraciones son archivos `api/migrations/NNNNN_descripcion.sql` con secciones `-- +goose Up` y `-- +goose Down`. El servidor las aplica al arrancar.

## Despliegue

Quitsie corre en Railway, con la base en PostgreSQL ([ADR 0012](docs/adr/0012-postgres-en-railway.md)). Railway compila la imagen del `Dockerfile` y despliega cada push a `main` que pase la CI. Los pasos, las variables y cómo restaurar un respaldo están en [docs/despliegue.md](docs/despliegue.md).
