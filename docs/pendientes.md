# Pendientes

Este documento dice qué falta para abrir Quitsie al público y en qué orden. Se comprobó contra `main` en `927f7f8` el 2026-11-01, y la seguridad se actualizó el 2026-11-02. El vocabulario sigue [CONTEXT.md](../CONTEXT.md). Las decisiones tomadas están en [adr/](adr/) y las reglas en [reglas-de-negocio.md](reglas-de-negocio.md).

Borra cada punto de aquí cuando quede terminado. Antes de actuar sobre un punto, compruébalo en el código.

## Estado actual

- **API:** las 40 operaciones de `openapi.yaml` están implementadas.
- **PWA:** usa datos reales: réplica, cola, formularios, Grupos, Amistades, invitaciones y sesiones.
- **Calidad:** la CI corre con Go 1.26.10 las pruebas de Go contra PostgreSQL, las de la web y las de punta a punta, `govulncheck`, `actionlint` y la compilación de la imagen de Railway.
- **Seguridad:** los 14 hallazgos están arreglados. El detalle está en [revision-seguridad.md](revision-seguridad.md).
- **Producción:** Quitsie se despliega en Railway con PostgreSQL ([ADR 0012](adr/0012-postgres-en-railway.md), [despliegue.md](despliegue.md)). Las comprobaciones de la sección 4 están sin hacer.

## Orden de trabajo

1. Obtener del dueño las decisiones que bloquean (sección 3).
2. Resolver lo que falta fuera del código (sección 5).
3. Desplegar y comprobar en producción (sección 4).

## 2. Seguridad

Los 14 hallazgos de [revision-seguridad.md](revision-seguridad.md) están arreglados. Quedan las comprobaciones que solo se pueden hacer al desplegar (sección 4).

- **Límites por IP detrás del proxy de Railway.** El servidor solo cree la IP del visitante en `CF-Connecting-IP`, y solo si la conexión llega por loopback ([limits.go](../api/internal/httpapi/limits.go)). En Railway todas las conexiones llegan desde su proxy, así que todos los visitantes comparten los límites por IP: 300 peticiones por minuto y 60 verificaciones por hora. Por eso tampoco hay un cupo de envío de códigos por IP. Mientras falte, quien pida códigos para muchos emails distintos puede agotar el tope global de 40 correos por hora, y entonces nadie recibe códigos hasta la hora siguiente. Arreglarlo exige saber qué encabezado escribe el proxy de Railway con la IP real y en qué conexiones se puede creer.

## 3. Decisiones del dueño

Ningún agente debe tomar estas decisiones por su cuenta.

**Bloquean abrir al público:**

| # | Decisión | Opciones |
| --- | --- | --- |
| 4 | Dominio principal | Propuesta: `quitsie.com`, y `quitsie.app` redirige |
| 6 | Retención de respaldos | La dan los respaldos de la base en Railway. Un dato borrado sigue en ellos ese tiempo |
| 7 | Inicio de sesión con Google | Crear el cliente OAuth y configurarlo, o no hacerlo. Sin configurar, el botón ya no aparece |

**Pueden esperar:**

| # | Decisión | Situación actual |
| --- | --- | --- |
| 8 | Amistades | La API no permite borrar una Amistad, editar un Amigo pendiente ni deshacer su vinculación |
| 9 | Liquidación con céntimos | Impide para siempre bajar el Grupo a una moneda sin decimales: no se edita, y borrada sigue contando |
| 10 | Usuario nuevo con Google | Se salta el alta y queda con la moneda por defecto. Arreglarlo exige que el callback lo indique |
| 11 | Moneda propuesta | `es` propone EUR y `es-419` no propone ninguna, lo que encaja mal con Latinoamérica |
| 12 | Formato de importes | Se agrupan miles siempre, y es-CO escribe «EUR 88» |
| 13 | Registros rechazados sin conexión | Descartar y corregir están deshabilitados, aunque ambos son locales |
| 14 | Cifras de los límites de entrada | Provisionales: 40 correos por hora y 90 al día entre todos, un minuto de espera entre códigos por email y por dispositivo conocido (decidido el 2026-11-07), y 10 fallos al día por email y por dispositivo conocido. Se cambian por entorno las dos primeras y en `api/internal/auth/` el resto |
| 15 | HSTS con `includeSubDomains` y `preload` | Solo la precarga protege la primera visita por `http://`, y exige `includeSubDomains`, que alcanza a los demás subdominios del dominio |
| 17 | Banda a cero mientras se vacía su fila | Una moneda que llega a cero sigue en verde con «0» hasta que sus fichas salen. La alternativa es mostrar la cifra anterior |
| 18 | Error de un campo por persona | Aparece al intentar guardar, como el del total. La alternativa es mostrarlo al salir del campo |

## 4. Despliegue

Lo tiene que hacer el dueño, porque exige abrir cuentas, comprar el dominio y pagar. Los pasos y las variables están en [despliegue.md](despliegue.md).

Después de desplegar, comprueba:

- Que `http://` redirija a `https://` y que `https://` traiga `strict-transport-security` ([HTTPS](despliegue.md#https)).
- Que los respaldos de la base estén activos, y una restauración de prueba con `record-restore` en una base aparte.
- Un correo real de Resend y, si se activa, un inicio de sesión real con Google.
- La memoria y el tiempo de `GET /sync` en el plan de Railway elegido.
- La PWA instalada en Safari de iOS.
- Que Cloudflare no guarde `/sw.js` en caché. El 2026-11-09 respondía `max-age=14400` y `cf-cache-status: EXPIRED`, aunque el servidor manda `no-cache` ([Caché de Cloudflare](despliegue.md#caché-de-cloudflare)).
- Las notificaciones push: generar las claves VAPID, ponerlas en Railway ([variables](despliegue.md#variables)), activarlas en un iPhone con la PWA instalada y recibir una de otro Usuario.

## 5. Falta fuera del código

- **Política de privacidad y términos.** La app no tiene ninguno. Maneja emails e importes, también de personas sin cuenta, y debe decir cuánto duran los respaldos. Qué se guarda, dónde y cuánto dura está en [datos-personales.md](datos-personales.md), con lo que solo el dueño puede decidir. La cookie de dispositivo conocido dura un año y sobrevive a cerrar sesión.

## 6. Deuda menor

**Interfaz:**

- Editar un Gasto mientras el Grupo cambia de moneda muestra el aviso de conflicto, no «La moneda del grupo cambió…». No se confirmó por qué; probablemente el cambio de moneda da una versión nueva al Gasto.
- Abrir un libro por su dirección antes de que termine la primera sincronización dice «Este grupo ya no está en tu lista». No se comprobó si se corrige solo al terminar.
- Tras reconstruir la réplica por un `410`, o cuando un pull completo no trae un registro que el servidor había aceptado, ese registro vuelve a la cola pero sale en la siguiente sincronización, no en la misma. Mientras tanto, Registros lo muestra sin sincronizar aunque haya conexión.
- Dos pestañas abiertas ejecutan dos motores de sincronización sobre la misma IndexedDB sin coordinarse: un pull atrasado de una puede retroceder el cursor o soltar un libro un momento.
- Si alguien borra su cuenta desde otro dispositivo y en este entra otro Usuario, los registros sin sincronizar de la cuenta borrada quedan apartados y ocultos en IndexedDB para siempre: nadie puede enviarlos ni descartarlos.

**Documentación:**

- `api/internal/httpapi/limits.go`, `headers.go` y sus comentarios hablan de `cloudflared` y del túnel, que ya no existen. Se corrigen con el arreglo de los límites por IP (sección 2).

**Limpieza:** en la máquina del dueño, los worktrees `api-endurecer-48a67d`, `web-escritura-c04f34`, `web-grupos-25864f` y `web-lectura-59f6a4` ya están integrados en `main`.

## 7. Sin verificar

- **Pantallas vacías en un Android** (reportado el 2026-11-09). En Quitsie instalada desde Chrome, al abrir un Grupo o un Gasto se ven la barra y las pestañas pero no el contenido, aunque los datos ya estén en el teléfono, y recargar lo arregla. Tampoco llegan los cambios en vivo. No se pudo reproducir en Chromium con la CPU lenta, navegando mientras otro Usuario añade gastos ni congelando la página. Desde entonces el motor abandona las peticiones colgadas y las consultas que fallan se reintentan. Falta una captura de **Cuenta → Diagnóstico** de ese teléfono después del fallo, para saber si las consultas a la réplica fallan o no responden, y cuántas páginas de Quitsie hay abiertas.

- Railway, su PostgreSQL, Google y Resend reales.
- Safari, Firefox y dos dispositivos reales a la vez. Las pruebas de punta a punta solo cubren Chromium. En desarrollo, Safari puede rechazar las cookies `Secure` por `http://localhost`.
- Copiar y compartir el enlace con el portapapeles real, sin el permiso que concede Playwright.
- Una notificación push real. Las pruebas simulan el servicio push del navegador y el permiso, porque el Chromium de las pruebas no tiene ninguno de los dos.
- La caché HTTP del navegador tras cerrar sesión, aunque la API ya responde `no-store`.

## Cómo trabajar aquí

**Comprobaciones.** Son las de la CI: `go vet ./...`, `go test ./...` y `govulncheck` en `api/`; `npm run typecheck`, `npm test` y `npm run build` en `web/`; el trabajo `lint`, que pasa `actionlint`, y el trabajo `image`, que compila el `Dockerfile`. Las pruebas de Go y las de punta a punta necesitan el PostgreSQL de `compose.yaml` (`docker compose up -d`). Tras cambiar `openapi.yaml` o las consultas, ejecuta `go generate ./...` en `api/` y `npm run generate` en `web/`.

**Integración.** El proyecto es trunk-based: commits pequeños y en verde, y ramas de horas. El historial usa commits `Merge …`. Hacer push a `main` exige confirmarlo con el dueño cada vez. Cuando `main` está abierta en la carpeta principal, desde un worktree se crea el merge con `git commit-tree`, se sube con `git push origin "${M}:refs/heads/main"` y la carpeta principal se actualiza con `git merge --ff-only origin/main`. En zsh las llaves son necesarias, porque `$M:r` es un modificador.

**Probar en local.** Usa un host propio para que las cookies no choquen con otras sesiones:

```bash
QUITSIE_ENV=development QUITSIE_MAIL=log QUITSIE_ADDR=127.0.0.1:18731 QUITSIE_PUBLIC_ORIGIN=http://prueba.localhost:18731 ./quitsie
```

Los códigos de acceso salen en el log. `curl` no guarda las cookies `Secure` por `http`, así que léelas de `Set-Cookie` y reenvíalas: un código solo se comprueba con la cookie de flujo que dejó su petición. Por loopback puedes rotar `CF-Connecting-IP` en las pruebas para no chocar con el límite por IP. Sin `QUITSIE_DATABASE_URL`, el modo de desarrollo usa la base de `compose.yaml`.

**Migraciones.** Solo avanzan, y cada una debe ser compatible con la versión anterior del binario, porque un despliegue puede revertir el binario.

**Errores para el dueño.** Lo que necesite su atención se registra en nivel `ERROR`, en los logs de Railway.

**Dónde están los límites:**

| Límite | Archivo |
| --- | --- |
| Peticiones por IP, bytes por Usuario, cuerpos, pulls a la vez, y cómo se agrupa IPv6 | `api/internal/httpapi/limits.go` |
| Miembros por Grupo, libros por Usuario | `api/internal/httpapi/handlers_ledgers.go` |
| Filas por libro | `api/internal/ledgers/ledgers.go` |
| Filas por pull | `api/internal/sync/pull.go` |
| Saldos | `api/internal/domain/validate_balance.go` y `web/src/domain/balances.ts` |
| Intentos de código y códigos vivos por email | `api/internal/auth/code.go` |
| Envíos y verificaciones por IP | `api/internal/auth/ratelimit.go` |
| Correos por hora y por día | `api/internal/auth/mailbudget.go` |
| Dispositivos conocidos | `api/internal/auth/knowndevice.go` |

**Trampas conocidas:**

- `updatePendingMember` es `PUT`. Con `PATCH` responde 404, lo que en una prueba de permisos parece un acierto. Sin `email`, solo borra el email que dio quien llama: uno que no ve se conserva.
- El email de un pendiente solo sale en las respuestas para quien lo escribió (`members.email_added_by`). Una prueba que añade Miembros directamente en la base y espera ver su email tiene que rellenar esa columna.
- sqlc ignora `sqlc.arg()` dentro de `ON CONFLICT ... DO UPDATE` sin avisar.
- `openapi_types.Email` valida al escribir la respuesta, así que un campo de salida con `format: email` puede romper la respuesta entera.
- Los archivos de `assets/` son `immutable`, así que una prueba de precache debe exigir `response.fromServiceWorker()`.
- `npx playwright test --repeat-each` comparte servidor y agota los 40 correos por hora: todo falla a los 10 s. Para repetir la suite, lánzala varias veces seguidas; cada ejecución tiene su servidor y su base.
- La PWA enruta tras `#`: un libro es `/#/ledgers/<id>`, no `/groups/<id>`.
- Tras restaurar un respaldo, las versiones de sincronización saltan a unos 1,8·10^15. Siguen siendo enteros exactos en JavaScript hasta el año 2255.
