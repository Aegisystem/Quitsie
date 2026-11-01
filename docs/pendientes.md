# Pendientes

Este documento dice qué falta para abrir Quitsie al público y en qué orden. Se comprobó contra `main` en `00f1ea4` el 2026-11-01. El vocabulario sigue [CONTEXT.md](../CONTEXT.md). Las decisiones tomadas están en [adr/](adr/) y las reglas en [reglas-de-negocio.md](reglas-de-negocio.md).

Borra cada punto de aquí cuando quede terminado. Antes de actuar sobre un punto, compruébalo en el código.

## Estado actual

- **API:** las 34 operaciones de `openapi.yaml` están implementadas.
- **PWA:** usa datos reales: réplica, cola, formularios, Grupos, Amistades, invitaciones y sesiones.
- **Calidad:** `main` pasa la CI con Go 1.26.10, y `govulncheck` no encuentra nada.
- **Seguridad:** hay 14 hallazgos, 7 arreglados y 7 abiertos. Ninguno de los abiertos es crítico o alto. El detalle está en [revision-seguridad.md](revision-seguridad.md).
- **Producción:** no hay nada desplegado. GitHub no tiene variables ni secretos, y `.quitsie-wizard.env` no existe, así que el asistente de despliegue nunca se ejecutó.

## Orden de trabajo

1. Integrar la rama de pruebas de punta a punta (sección 1).
2. Obtener del dueño las decisiones que bloquean (sección 3).
3. Arreglar los hallazgos de seguridad abiertos (sección 2).
4. Resolver lo que falta fuera del código (sección 5).
5. Desplegar y comprobar en producción (sección 4).

## 1. Rama sin integrar

`claude/sweet-einstein-7c3928` (`ff2d2a4`) tiene 6 commits sobre `26948a3`. Trae pruebas de Playwright (5 pruebas en 4 archivos), su paso en la CI y un aviso de Saldo provisional en Inicio y en el libro. Une con `main` sin conflictos.

Pasos:

1. Actualiza la rama con `main` y ejecuta `npm run e2e` en `web/`.
2. Revisa los fallos contra los límites nuevos. `GET /sync` admite un pull por Usuario, así que una prueba con el mismo Usuario en dos contextos puede recibir `429` y tardar un reintento. Cada contexto ya envía su propio `CF-Connecting-IP`.
3. Integra y mira la primera ejecución en GitHub: `playwright install --with-deps` nunca corrió allí. Estas pruebas frenan el despliegue si fallan.
4. Menciona las pruebas en la sección Despliegue de `docs/arquitectura.md`.

## 2. Seguridad abierta

Cada hallazgo tiene su ataque y su arreglo en [revision-seguridad.md](revision-seguridad.md). Al arreglar uno, actualiza allí su estado y lo medido.

| # | Gravedad | Trabajo | Requisito |
| --- | --- | --- | --- |
| 14 | Baja | Migración con índices en `login_codes (expires_at)` y `sessions (last_used_at)` | Ninguno |
| 13 | Baja | Rechazar el registro que deje un Saldo por encima de 2^62 y corregir la regla de los 10^15 | Ninguno |
| 6 | Media | Tope global de correos con aviso, `maxSendsPerIP` a 10 y agrupar IPv6 por /56 | Elegir las cifras |
| 9 | Media | Borrar la réplica ante un 401 y conservar solo la cola | Decisión 3 |
| 5 | Media | Atar cada código al navegador con una cookie, y contar intentos por flujo y envíos por email e IP | Ninguno |
| 8 | Media | Regenerar el enlace al sacar a alguien, o exigir readmisión | Decisión 1 |
| 11 | Baja | Mostrar el email de un pendiente solo a quien lo añadió, o enmascarado | Decisión 2 |

El hallazgo 5 importa más de lo que dice su gravedad: 12 peticiones bloquean los códigos de un email durante 24 horas. Esa fue la contrapartida del arreglo del hallazgo 4.

## 3. Decisiones del dueño

Ningún agente debe tomar estas decisiones por su cuenta.

**Bloquean abrir al público:**

| # | Decisión | Opciones |
| --- | --- | --- |
| 1 | Qué pasa con quien fue sacado de un Grupo | Regenerar el enlace, lo que invalida los ya enviados, o exigir que otro Miembro lo readmita |
| 2 | Quién ve el email de un Miembro pendiente | Quien lo añadió, enmascarado, o todos, como ahora |
| 3 | Qué pasa con la cola si la sesión termina sola y entra otro Usuario | Vaciarla sin aviso, como ahora, o conservarla por Usuario |
| 4 | Dominio principal | El asistente propone `quitsie.com`, y `quitsie.app` redirige |
| 5 | IP pública de la VM | Aceptar unos 3,65 USD al mes, o cambiar de proveedor con un ADR nuevo. El ADR 0007 dice 0 USD |
| 6 | Retención de respaldos | Hoy son 7 días, y un dato borrado sigue en ellos ese tiempo |
| 7 | Inicio de sesión con Google | Crear el cliente OAuth, u ocultar el botón (sección 5) |

**Pueden esperar:**

| # | Decisión | Situación actual |
| --- | --- | --- |
| 8 | Amistades | La API no permite borrar una Amistad, editar un Amigo pendiente ni deshacer su vinculación |
| 9 | Liquidación con céntimos | Impide para siempre bajar el Grupo a una moneda sin decimales: no se edita, y borrada sigue contando |
| 10 | Usuario nuevo con Google | Se salta el alta y queda con la moneda por defecto. Arreglarlo exige que el callback lo indique |
| 11 | Moneda propuesta | `es` propone EUR y `es-419` no propone ninguna, lo que encaja mal con Latinoamérica |
| 12 | Formato de importes | Se agrupan miles siempre, y es-CO escribe «EUR 88» |
| 13 | Registros rechazados sin conexión | Descartar y corregir están deshabilitados, aunque ambos son locales |

## 4. Despliegue

Lo tiene que hacer el dueño, porque exige comprar el dominio, abrir cuentas y pagar. La guía es `bash deploy/first-deploy-wizard.sh`, y [despliegue.md](despliegue.md) explica sus 14 etapas. Pide `gh` con sesión iniciada y `gcloud`.

Durante la instalación:

- Activa «Always Use HTTPS» y HSTS en Cloudflare. El asistente no lo hace.
- Crea la regla de límite para `/api/v1/auth/*`. Los documentos citan un límite de Cloudflare que nada crea.

Después de instalar, comprueba:

- Que dos visitantes tengan límites distintos y que un `CF-Connecting-IP` enviado por el cliente se sobrescriba.
- Que `sudo quitsie-restore-test` termine en `OK`.
- Un correo real de Resend y, si se activa, un inicio de sesión real con Google.
- La memoria y el tiempo de `GET /sync` en la VM, con `GOMEMLIMIT=700MiB`.
- La PWA instalada en Safari de iOS.

Nada de `deploy/` se probó en una VM real. `shellcheck` y `actionlint` nunca se ejecutaron sobre esos archivos. La imagen `debian-13` no está confirmada, y `QUITSIE_IMAGE_FAMILY=debian-12` es la alternativa.

## 5. Falta fuera del código

- **Política de privacidad y términos.** La app no tiene ninguno. Maneja emails e importes, también de personas sin cuenta, y debe decir cuánto duran los respaldos.
- **Alertas.** Solo existe el presupuesto de 5 USD en Google Cloud. Nada avisa de una caída, del disco lleno, de errores 500 ni de un fallo de Litestream.
- **Cliente OAuth de Google.** El asistente no lo crea. Sin él, el botón «Continuar con Google» sigue visible y termina en error.
- **Restaurar un respaldo.** `version_counter` no salta al restaurar. Si el contador vuelve a superar el cursor de un dispositivo antes de que sincronice, no hay `410` y ese dispositivo pierde filas. La propuesta es sumar un salto grande al restaurar.

## 6. Deuda menor

**Interfaz:**

- `Account.tsx` no pasa `used` a `CurrencySelect`, así que solo sube la Moneda preferida.
- Una Liquidación rechazada solo se corrige si cambió la moneda: la ruta `settle` no tiene `fix`.
- `ExpenseForm` solo explica el error de importe en el total. En los campos por persona, un valor ilegible cuenta como 0.
- `ExpenseForm` mantiene el aviso de fallo hasta el siguiente intento de guardar.
- Las fichas sobrantes desaparecen de golpe, y el contrato de diseño pide ver la fila vaciarse.

**Documentación:**

- `docs/arquitectura.md` no incluye la IP pública en la tabla de costos.
- `docs/arquitectura.md:137`, `docs/despliegue.md:122` y `limits.go:26` citan el límite de Cloudflare. Corrígelos si la regla no se crea.

**Limpieza:** los worktrees `api-endurecer-48a67d`, `web-escritura-c04f34`, `web-grupos-25864f` y `web-lectura-59f6a4` ya están integrados en `main`.

## 7. Sin verificar

- Cloudflare, Google, Resend y la VM reales.
- Sin conexión real con la PWA compilada, fuera de las pruebas de Playwright que aún no están en `main`.
- El service worker en ejecución: no se registra en el navegador integrado de la app.
- Safari, Firefox y dos dispositivos reales a la vez.
- Copiar y compartir el enlace con el portapapeles real.
- La caché HTTP del navegador tras cerrar sesión.

## Cómo trabajar aquí

**Comprobaciones.** Son las de la CI: `go vet ./...`, `go test ./...` y `govulncheck` en `api/`, y `npm run typecheck`, `npm test` y `npm run build` en `web/`. Tras cambiar `openapi.yaml` o las consultas, ejecuta `go generate ./...` en `api/` y `npm run generate` en `web/`.

**Integración.** El proyecto es trunk-based: commits pequeños y en verde, y ramas de horas. El historial usa commits `Merge …`. Hacer push a `main` exige confirmarlo con el dueño cada vez. Cuando `main` está abierta en la carpeta principal, desde un worktree se crea el merge con `git commit-tree`, se sube con `git push origin "${M}:refs/heads/main"` y la carpeta principal se actualiza con `git merge --ff-only origin/main`. En zsh las llaves son necesarias, porque `$M:r` es un modificador.

**Probar en local.** Usa un host propio para que las cookies no choquen con otras sesiones:

```bash
QUITSIE_ENV=development QUITSIE_MAIL=log QUITSIE_ADDR=127.0.0.1:18731 QUITSIE_PUBLIC_ORIGIN=http://prueba.localhost:18731 ./quitsie
```

Los códigos de acceso salen en el log. `curl` no guarda la cookie `Secure`, así que léela de `Set-Cookie`. Por loopback puedes rotar `CF-Connecting-IP` en las pruebas para no chocar con el límite por IP.

**Migraciones.** Solo avanzan, y cada una debe ser compatible con la versión anterior del binario, porque un despliegue puede revertir el binario.

**Dónde están los límites:**

| Límite | Archivo |
| --- | --- |
| Peticiones por IP, bytes por Usuario, cuerpos, pulls a la vez | `api/internal/httpapi/limits.go` |
| Miembros por Grupo, libros por Usuario | `api/internal/httpapi/handlers_ledgers.go` |
| Filas por libro | `api/internal/ledgers/ledgers.go` |
| Filas por pull | `api/internal/sync/pull.go` |
| Intentos de código | `api/internal/auth/code.go` |
| Envíos y verificaciones por IP | `api/internal/auth/ratelimit.go` |

**Trampas conocidas:**

- `updatePendingMember` es `PUT`. Con `PATCH` responde 404, lo que en una prueba de permisos parece un acierto.
- sqlc ignora `sqlc.arg()` dentro de `ON CONFLICT ... DO UPDATE` sin avisar.
- `openapi_types.Email` valida al escribir la respuesta, así que un campo de salida con `format: email` puede romper la respuesta entera.
- Los archivos de `assets/` son `immutable`, así que una prueba de precache debe exigir `response.fromServiceWorker()`.
