# Go nativo en una VM gratuita; Cloudflare D1 descartado

La API corre como binario Go nativo en la VM gratuita de Google Cloud (`e2-micro`), con SQLite en su disco y respaldo continuo con Litestream a Cloudflare R2. Cloudflare, en su plan gratis, queda delante como DNS, CDN y túnel. Se descartó Cloudflare D1, a pesar de ser gratis, porque solo es práctico desde código que corre en Workers. Su API REST está pensada para administrar la base y comparte el límite general de la API de Cloudflare (1.200 peticiones cada 5 minutos). Correr Go dentro de Workers exige compilarlo a WebAssembly con una librería que se declara experimental, y el plan gratis da 10 ms de CPU por petición.

## Considered Options

- **Go compilado a WebAssembly en Workers + D1**: gratis, pero frágil por la librería experimental y el límite de CPU.
- **TypeScript en Workers + D1**: gratis y con soporte oficial, pero abandona Go (ver ADR 0002).
- **Cloudflare Containers**: Go nativo dentro de Cloudflare, pero solo en el plan de pago (desde 5 USD/mes).
- **VPS de pago (~5 USD/mes)**: misma arquitectura que la elegida; queda como siguiente paso cuando la VM gratuita se quede corta.

## Consequences

- Pasar a un VPS de pago es copiar el mismo binario y el mismo archivo de base de datos.
- La VM gratuita incluye 1 GB de salida al mes, así que los archivos estáticos deben servirse desde la caché de Cloudflare.
