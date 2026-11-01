# Pendientes

Este documento dice qué falta para abrir Quitsie al público y en qué orden. Se comprobó el 2026-11-01 contra la rama `claude/exciting-thompson-t1qh2a`, que parte de `main` en `e73d9b0` y aún no está integrada. El vocabulario sigue [CONTEXT.md](../CONTEXT.md). Las decisiones tomadas están en [adr/](adr/) y las reglas en [reglas-de-negocio.md](reglas-de-negocio.md).

Borra cada punto de aquí cuando quede terminado. Antes de actuar sobre un punto, compruébalo en el código.

## Estado actual

- **API:** las 35 operaciones de `openapi.yaml` están implementadas.
- **PWA:** usa datos reales: réplica, cola, formularios, Grupos, Amistades, invitaciones y sesiones.
- **Calidad:** la rama pasa `go vet`, las pruebas de Go y de la web, el typecheck, el build, `shellcheck`, `actionlint` y `systemd-analyze verify`. `govulncheck` no pudo ejecutarse en esta sesión porque la red bloquea `vuln.go.dev`; lo ejecuta la CI, y la rama no añade dependencias de Go.
- **Seguridad:** de los 14 hallazgos quedan 3 abiertos (8, 9 y 11), y los tres esperan una decisión del dueño. El detalle está en [revision-seguridad.md](revision-seguridad.md).
- **Producción:** no hay nada desplegado. GitHub no tiene variables ni secretos, y `.quitsie-wizard.env` no existe, así que el asistente de despliegue nunca se ejecutó.

## Orden de trabajo

1. Revisar la rama `claude/exciting-thompson-t1qh2a` e integrarla en `main` (sección 0).
2. Integrar la rama de pruebas de punta a punta (sección 1).
3. Obtener del dueño las decisiones que bloquean (sección 3).
4. Arreglar los hallazgos de seguridad abiertos (sección 2).
5. Resolver lo que falta fuera del código (sección 5).
6. Desplegar y comprobar en producción (sección 4).

## 0. Rama sin revisar

`claude/exciting-thompson-t1qh2a` trae, en commits `Merge …` sobre `e73d9b0`:

- Los arreglos de los hallazgos 5, 6, 13 y 14, con el [ADR 0011](adr/0011-codigos-atados-al-navegador.md).
- Cabeceras de seguridad, `Cache-Control: no-store` en la API, y HSTS y redirección a HTTPS en producción.
- El salto de `version_counter` al restaurar un respaldo.
- `shellcheck`, `actionlint` y `systemd-analyze verify` en la CI, las acciones fijadas por SHA, alertas por correo desde la VM y una prueba semanal de restauración.
- La deuda menor de interfaz que había en esta lista.
- «Continuar con Google» solo cuando el servidor lo tiene configurado, cookies con prefijo `__Host-` y los fallos de los manejadores en el log como `ERROR`.
- Notas para el dueño: [opciones para la IP pública](investigacion/ip-publica.md) y el [inventario de datos personales](datos-personales.md).

Dos cambios afectan a quien ya probó la app: las migraciones `00003` a `00005` se aplican al arrancar, y un código de acceso solo vale en el navegador que lo pidió.

## 1. Rama sin integrar

`claude/sweet-einstein-7c3928` (`ff2d2a4`) tiene 6 commits sobre `26948a3`. Trae pruebas de Playwright (5 pruebas en 4 archivos), su paso en la CI y un aviso de Saldo provisional en Inicio y en el libro. No está en GitHub, solo en la máquina del dueño. Unía con `main` sin conflictos antes de la rama de la sección 0, que cambió `Balance.tsx` y `Home`: el aviso de Saldo provisional puede chocar con ellos.

Pasos:

1. Actualiza la rama con `main` y ejecuta `npm run e2e` en `web/`.
2. Revisa los fallos contra los límites nuevos:
   - `GET /sync` admite un pull por Usuario, así que una prueba con el mismo Usuario en dos contextos puede recibir `429` y tardar un reintento. Cada contexto ya envía su propio `CF-Connecting-IP`.
   - Un código solo vale en el contexto que lo pidió, con su cookie de flujo.
   - Un navegador nuevo pide 3 códigos por hora por email e IP, y una IP pide 10 por hora. Las pruebas que entran muchas veces deben rotar `CF-Connecting-IP` o reutilizar el contexto, que pasa a ser un dispositivo conocido.
   - Las cookies se llaman ahora `__Host-quitsie_session`, `__Host-quitsie_code_flow`, `__Host-quitsie_known_device` y `__Host-quitsie_google`, todas con `Path=/`.
   - La CSP prohíbe scripts en línea, también los que inyecte una prueba con `addScriptTag({ content })`. `addInitScript` sí funciona.
3. Integra y mira la primera ejecución en GitHub: `playwright install --with-deps` nunca corrió allí. Estas pruebas frenan el despliegue si fallan.
4. Menciona las pruebas en la sección Despliegue de `docs/arquitectura.md`.

## 2. Seguridad abierta

Cada hallazgo tiene su ataque y su arreglo en [revision-seguridad.md](revision-seguridad.md). Al arreglar uno, actualiza allí su estado y lo medido.

| # | Gravedad | Trabajo | Requisito |
| --- | --- | --- | --- |
| 9 | Media | Borrar la réplica ante un 401 y conservar solo la cola | Decisión 3 |
| 8 | Media | Regenerar el enlace al sacar a alguien, o exigir readmisión | Decisión 1 |
| 11 | Baja | Mostrar el email de un pendiente solo a quien lo añadió, o enmascarado | Decisión 2 |

## 3. Decisiones del dueño

Ningún agente debe tomar estas decisiones por su cuenta.

**Bloquean abrir al público:**

| # | Decisión | Opciones |
| --- | --- | --- |
| 1 | Qué pasa con quien fue sacado de un Grupo | Regenerar el enlace, lo que invalida los ya enviados, o exigir que otro Miembro lo readmita |
| 2 | Quién ve el email de un Miembro pendiente | Quien lo añadió, enmascarado, o todos, como ahora |
| 3 | Qué pasa con la cola si la sesión termina sola y entra otro Usuario | Vaciarla sin aviso, como ahora, o conservarla por Usuario |
| 4 | Dominio principal | El asistente propone `quitsie.com`, y `quitsie.app` redirige |
| 5 | IP pública de la VM | Aceptar unos 3,65 USD al mes, o cambiar de proveedor con un ADR nuevo. El ADR 0007 dice 0 USD. Hay una tercera vía: una VM con IPv6 externa y sin IPv4 externa, a 0 USD, si R2 y Resend responden por IPv6. Ver [opciones](investigacion/ip-publica.md) |
| 6 | Retención de respaldos | Hoy son 7 días, y un dato borrado sigue en ellos ese tiempo |
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
| 14 | Cifras de los límites de entrada | Provisionales: 40 correos por hora y 90 al día entre todos, 10 códigos por hora por IP, y 5 códigos por hora y 10 fallos al día por dispositivo conocido. Se cambian por entorno las dos primeras y en `api/internal/auth/` el resto |
| 15 | HSTS con `includeSubDomains` y `preload` | Solo la precarga protege la primera visita por `http://`, y exige `includeSubDomains`, que alcanza a los demás subdominios del dominio |
| 16 | Deudas por parejas por encima de 2^53 - 1 | El Grupo muestra en silencio las Deudas simplificadas aunque tenga la Simplificación desactivada. Solo pasa con billones en círculo dentro del propio Grupo. ¿Hace falta un aviso? |
| 17 | Banda a cero mientras se vacía su fila | Una moneda que llega a cero sigue en verde con «0» hasta que sus fichas salen. La alternativa es mostrar la cifra anterior |
| 18 | Error de un campo por persona | Aparece al intentar guardar, como el del total. La alternativa es mostrarlo al salir del campo |

## 4. Despliegue

Lo tiene que hacer el dueño, porque exige comprar el dominio, abrir cuentas y pagar. La guía es `bash deploy/first-deploy-wizard.sh`, y [despliegue.md](despliegue.md) explica sus 14 etapas. Pide `gh` con sesión iniciada y `gcloud`. La etapa 9 pide ahora el email que recibe las alertas, y la 14 envía un correo de prueba.

Durante la instalación:

- Activa «Always Use HTTPS» en Cloudflare. El servidor ya redirige a HTTPS y envía HSTS en producción, así que es una segunda capa.
- Crea la regla de límite para `/api/v1/auth/*`. Los documentos citan un límite de Cloudflare que nada crea.
- Opcional: un monitor externo gratuito sobre `/api/v1/health`. Una VM caída no puede avisar de sí misma ([despliegue](despliegue.md#alertas)).

Después de instalar, comprueba:

- Que dos visitantes tengan límites distintos y que un `CF-Connecting-IP` enviado por el cliente se sobrescriba.
- Que `http://` responda 301 a través del túnel real, y que una visita por `https://` no se redirija.
- Que `sudo quitsie-restore-test` termine en `OK`, y que `systemctl list-timers` muestre la prueba semanal y la comprobación de alertas.
- Un correo real de Resend, el correo de prueba de las alertas y, si se activa, un inicio de sesión real con Google.
- La memoria y el tiempo de `GET /sync` en la VM, con `GOMEMLIMIT=700MiB`.
- La PWA instalada en Safari de iOS.

Nada de `deploy/` se probó en una VM real ni bajo systemd real: las unidades nuevas, el script de alertas y el marcador de restauración solo se probaron con sustitutos. La imagen `debian-13` no está confirmada, y `QUITSIE_IMAGE_FAMILY=debian-12` es la alternativa.

## 5. Falta fuera del código

- **Política de privacidad y términos.** La app no tiene ninguno. Maneja emails e importes, también de personas sin cuenta, y debe decir cuánto duran los respaldos. Qué se guarda, dónde y cuánto dura está en [datos-personales.md](datos-personales.md), con lo que solo el dueño puede decidir. La cookie de dispositivo conocido dura un año y sobrevive a cerrar sesión.
- **Respaldo que deja de subir.** Si Litestream sigue activo pero no puede escribir en R2 mientras R2 se puede leer, nada avisa, y la prueba semanal restaura datos viejos y pasa. Comparar en la prueba la versión del respaldo con la de la base viva lo cerraría, pero antes hay que medir en la VM cuánto se retrasa el respaldo de forma normal.

## 6. Deuda menor

**Interfaz:**

- Editar un Gasto mientras el Grupo cambia de moneda muestra el aviso de conflicto, no «La moneda del grupo cambió…». No se confirmó por qué; probablemente el cambio de moneda da una versión nueva al Gasto.
- Abrir un libro por su dirección antes de que termine la primera sincronización dice «Este grupo ya no está en tu lista». No se comprobó si se corrige solo al terminar.
- Tras reconstruir la réplica por un `410`, los registros que vuelven a la cola salen en la siguiente sincronización, no en la misma.

**Documentación:**

- `docs/arquitectura.md`, `docs/despliegue.md` y `api/internal/httpapi/limits.go` citan el límite de Cloudflare. Corrígelos si la regla no se crea.

**Limpieza:** en la máquina del dueño, los worktrees `api-endurecer-48a67d`, `web-escritura-c04f34`, `web-grupos-25864f` y `web-lectura-59f6a4` ya están integrados en `main`.

## 7. Sin verificar

- Cloudflare, Google, Resend y la VM reales.
- Safari, Firefox y dos dispositivos reales a la vez. En desarrollo, Safari puede rechazar las cookies `Secure` por `http://localhost`.
- Copiar y compartir el enlace con el portapapeles real, sin el permiso que concede Playwright.
- La caché HTTP del navegador tras cerrar sesión, aunque la API ya responde `no-store`.

## Cómo trabajar aquí

**Comprobaciones.** Son las de la CI: `go vet ./...`, `go test ./...` y `govulncheck` en `api/`; `npm run typecheck`, `npm test` y `npm run build` en `web/`; y el trabajo `lint`, que pasa `shellcheck`, `actionlint`, `deploy/alert-check.test.sh` y `systemd-analyze verify`. Tras cambiar `openapi.yaml` o las consultas, ejecuta `go generate ./...` en `api/` y `npm run generate` en `web/`.

**Integración.** El proyecto es trunk-based: commits pequeños y en verde, y ramas de horas. El historial usa commits `Merge …`. Hacer push a `main` exige confirmarlo con el dueño cada vez. Cuando `main` está abierta en la carpeta principal, desde un worktree se crea el merge con `git commit-tree`, se sube con `git push origin "${M}:refs/heads/main"` y la carpeta principal se actualiza con `git merge --ff-only origin/main`. En zsh las llaves son necesarias, porque `$M:r` es un modificador.

**Probar en local.** Usa un host propio para que las cookies no choquen con otras sesiones:

```bash
QUITSIE_ENV=development QUITSIE_MAIL=log QUITSIE_ADDR=127.0.0.1:18731 QUITSIE_PUBLIC_ORIGIN=http://prueba.localhost:18731 ./quitsie
```

Los códigos de acceso salen en el log. `curl` no guarda las cookies `Secure` por `http`, así que léelas de `Set-Cookie` y reenvíalas: un código solo se comprueba con la cookie de flujo que dejó su petición. Por loopback puedes rotar `CF-Connecting-IP` en las pruebas para no chocar con el límite por IP.

**Migraciones.** Solo avanzan, y cada una debe ser compatible con la versión anterior del binario, porque un despliegue puede revertir el binario.

**Errores para el dueño.** Lo que necesite su atención se registra en nivel `ERROR`: las alertas de la VM saltan con esas líneas del journal.

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
| Correos de alerta al día | `deploy/alert-check.sh` |

**Trampas conocidas:**

- `updatePendingMember` es `PUT`. Con `PATCH` responde 404, lo que en una prueba de permisos parece un acierto.
- sqlc ignora `sqlc.arg()` dentro de `ON CONFLICT ... DO UPDATE` sin avisar.
- `openapi_types.Email` valida al escribir la respuesta, así que un campo de salida con `format: email` puede romper la respuesta entera.
- Los archivos de `assets/` son `immutable`, así que una prueba de precache debe exigir `response.fromServiceWorker()`.
- La PWA enruta tras `#`: un libro es `/#/ledgers/<id>`, no `/groups/<id>`.
- Tras restaurar un respaldo, las versiones de sincronización saltan a unos 1,8·10^15. Siguen siendo enteros exactos en JavaScript hasta el año 2255.
