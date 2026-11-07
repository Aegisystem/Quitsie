# Despliegue

Quitsie corre en Railway como dos servicios del mismo proyecto: la aplicación, que es un binario Go con la PWA embebida, y una base PostgreSQL ([ADR 0012](adr/0012-postgres-en-railway.md)). Railway compila la imagen con el [`Dockerfile`](../Dockerfile) y la despliega con lo que dice [`railway.toml`](../railway.toml).

## Primer despliegue

1. En Railway, crea un proyecto y añade **PostgreSQL** (New → Database → PostgreSQL).
2. Añade la aplicación desde el repositorio de GitHub (New → GitHub Repo), con la rama `main`. Deja vacío el Root Directory: el `Dockerfile` necesita `web/` y `api/`.
3. Verifica en Resend el dominio del remitente de los códigos ([Dominio del correo](#dominio-del-correo)).
4. Rellena las variables de la aplicación ([Variables](#variables)).
5. En **Settings → Networking**, pulsa **Generate Domain**, o añade tu dominio propio. Railway detecta el puerto por la variable `PORT`, que él mismo pone.
6. En **Settings → Deploy**, activa **Wait for CI**. Así Railway solo despliega los commits de `main` que pasaron la CI.

Al terminar, `https://<dominio>/` muestra la PWA y `https://<dominio>/api/v1/health` responde `{"status":"ok","database":"ok","version":"<commit>"}`.

No hay que escribir comandos de build ni de arranque en el panel: los da el `Dockerfile`. `railway.toml` fija la comprobación de salud y qué cambios despliegan.

## Variables

Sin las obligatorias, el servidor no arranca y el despliegue falla en la comprobación de salud. El despliegue anterior sigue sirviendo.

| Variable | Valor |
| --- | --- |
| `QUITSIE_DATABASE_URL` | `${{Postgres.DATABASE_URL}}`. Railway la sustituye por la URL interna de la base. Si el servicio de la base tiene otro nombre, cambia `Postgres` por ese nombre. |
| `QUITSIE_PUBLIC_ORIGIN` | URL base de la aplicación, con `https` y sin barra final: `https://${{RAILWAY_PUBLIC_DOMAIN}}` o `https://<tu-dominio>`. |
| `QUITSIE_RESEND_API_KEY` | Clave de API de Resend con permiso de envío. |
| `QUITSIE_MAIL_FROM` | Remitente de un dominio verificado en Resend: `Quitsie <codigos@<dominio>>` ([Dominio del correo](#dominio-del-correo)). |
| `QUITSIE_INVITE_SECRET` | 32 bytes o más, que firman los Enlaces de invitación. Genéralos con `openssl rand -base64 32`. Cambiarlo revoca todos los enlaces. Guárdalo también en tu gestor de contraseñas. |

Opcionales:

| Variable | Para qué |
| --- | --- |
| `QUITSIE_GOOGLE_CLIENT_ID` y `QUITSIE_GOOGLE_CLIENT_SECRET` | Inicio de sesión con Google. Sin los dos, el botón no aparece. La URI de redirección del cliente en Google Cloud es `<QUITSIE_PUBLIC_ORIGIN>/api/v1/auth/google/callback`. |
| `QUITSIE_DEFAULT_CURRENCY` | Moneda preferida de los Usuarios nuevos. Por defecto, `USD`. |
| `QUITSIE_MAIL_MAX_PER_HOUR` y `QUITSIE_MAIL_MAX_PER_DAY` | Tope de correos con códigos por hora y por 24 horas. Por defecto 40 y 90, por debajo de los 100 diarios gratuitos de Resend. |
| `QUITSIE_VAPID_PUBLIC_KEY` y `QUITSIE_VAPID_PRIVATE_KEY` | Notificaciones push. Genera el par en tu equipo con `go -C api run ./cmd/server vapid-keys`. Sin las dos, el servidor no envía notificaciones. Cambiarlas apaga las notificaciones de cada dispositivo hasta que la PWA se abre allí de nuevo. Guarda la privada también en tu gestor de contraseñas. |
| `GOMEMLIMIT` | Memoria a partir de la cual Go recoge basura con más fuerza, por ejemplo `400MiB` en un plan de 512 MB. |

`QUITSIE_ENV=development` y `QUITSIE_MAIL=log` son para desarrollo local. En producción, `QUITSIE_MAIL=log` impide arrancar, porque escribiría los códigos en el log.

## Despliegue continuo

Cada push a `main` que pase la CI llega a Railway:

- **Qué despliega.** Solo los cambios en `api/`, `web/`, `openapi.yaml`, `Dockerfile` o `railway.toml` (`watchPatterns`). Un cambio solo en `docs/` no despliega.
- **Cuándo pasa a servir.** Railway arranca la imagen nueva y espera hasta 60 s a que `/api/v1/health` responda 200. Si no responde, el despliegue falla y el anterior sigue sirviendo.
- **Qué comprueba la CI.** Las pruebas de Go y de la PWA, las e2e de Playwright contra el binario y la compilación de la misma imagen que usa Railway (job `image` de [`ci.yml`](../.github/workflows/ci.yml)).
- **A mano.** Desde el panel de Railway, **Redeploy** en el último despliegue.

Las migraciones se aplican al arrancar y solo avanzan. Si se vuelve a un despliegue anterior, el binario viejo arranca contra el esquema nuevo. Por eso cada migración debe ser compatible con la versión anterior: primero se añade y después se quita.

## Dominio del correo

Los códigos de acceso salen de Resend con el remitente de `QUITSIE_MAIL_FROM`. Outlook, Hotmail y Gmail rechazan o mandan a spam el correo de un dominio que no pasa SPF, DKIM y DMARC. Outlook responde entonces `550 5.7.1 Message rejected: Message lacks SPF/DKIM/DMARC Auth Records`, y Resend lo muestra como rebote.

1. En Resend, entra a **Domains → Add Domain** y escribe el dominio del remitente.
2. Copia en el DNS del dominio los tres registros que muestra Resend, con sus valores exactos:

   | Tipo | Nombre | Para qué |
   | --- | --- | --- |
   | `TXT` | `resend._domainkey` | DKIM: la clave pública con la que Resend firma cada correo. |
   | `MX` | `send` | Recibe los avisos de rebote en el subdominio `send`. |
   | `TXT` | `send` | SPF: autoriza a Resend a enviar desde el subdominio `send`. |

3. Agrega el registro DMARC. Resend no lo pide para verificar el dominio, pero Outlook lo exige:

   | Tipo | Nombre | Valor |
   | --- | --- | --- |
   | `TXT` | `_dmarc` | `v=DMARC1; p=none;` |

   `p=none` solo pide reportes y no rechaza nada. Cuando los correos lleguen bien, puedes subirlo a `p=quarantine`. Un dominio tiene un solo registro `_dmarc`: si ya existe uno, conserva ese.
4. Espera a que Resend marque los tres registros como **Verified**. Un cambio de DNS tarda de minutos a horas en propagarse.
5. Comprueba que el DNS responde. Cambia `<dominio>` por el tuyo; cada comando debe devolver un valor:

   ```bash
   dig +short TXT _dmarc.<dominio>
   dig +short TXT resend._domainkey.<dominio>
   dig +short TXT send.<dominio>
   dig +short MX send.<dominio>
   ```

6. Usa en `QUITSIE_MAIL_FROM` un remitente de ese mismo dominio. Con otro dominio u otro subdominio, la firma DKIM no coincide con el remitente y DMARC falla.

Un rebote de tipo **Transient** no bloquea la dirección en Resend: el siguiente código llega cuando el DNS esté bien.

## HTTPS

Railway termina el TLS. El servidor envía `Strict-Transport-Security: max-age=31536000` en producción. La redirección de `http://` a `https://` que hace el propio servidor solo actúa detrás de `cloudflared` en la misma máquina, así que en Railway depende de Railway. Compruébalo tras el primer despliegue: `curl -sI http://<dominio>/` debe responder con una redirección a `https://`, y `curl -sI https://<dominio>/` debe traer `strict-transport-security`.

## Respaldo y restauración

Los respaldos son los de la base de Railway (pestaña **Backups** del servicio PostgreSQL). Comprueba en tu plan si están incluidos y cada cuánto se hacen.

### Qué pasa al restaurar

Un respaldo es una copia vieja de la base. Un dispositivo que sincronizó después del instante del respaldo guarda filas que la base restaurada no tiene, y su cursor apunta a versiones que la base volvería a entregar. Para que no se mezclen, después de restaurar se ejecuta una vez:

```bash
QUITSIE_DATABASE_URL='<DATABASE_PUBLIC_URL de la base>' go -C api run ./cmd/server record-restore
```

`DATABASE_PUBLIC_URL` está en las variables del servicio PostgreSQL de Railway. El comando hace saltar la versión de sincronización y anota el tramo saltado en `restore_gaps`. Los dispositivos con un cursor dentro de ese tramo reciben `410` y rehacen su réplica desde cero, sin perder sus Registros sin sincronizar ([arquitectura.md](arquitectura.md#restaurar-un-respaldo)). Escribe en el log una línea con `restored_version` y `jumped_version`.

- Ejecútalo antes de que la aplicación vuelva a atender peticiones. Detén la aplicación en Railway mientras restauras.
- Ejecutarlo dos veces, o sin restauración, no hace daño.
- Si no se ejecuta, los dispositivos que sincronizaron después del instante restaurado pueden perder filas sin aviso.

## Operación

- **Logs.** En Railway, pestaña **Logs** de la aplicación. El servidor escribe en `level=ERROR` todo lo que necesita al dueño, también cada respuesta 500.
- **Avisos.** Railway avisa por email cuando un despliegue falla o el servicio se cae (Settings de la cuenta → Notifications). Para enterarte de que la aplicación no responde aunque Railway la vea viva, crea un monitor externo gratuito, por ejemplo en UptimeRobot, que pida `https://<dominio>/api/v1/health` cada 5 minutos.
- **Límites de peticiones.** Cada IP tiene 300 peticiones por minuto. Un cuerpo no puede pasar de 2 MiB, ni de 4 KiB en `POST /auth/*`. Cada Usuario envía como máximo 5 MiB de cambios por hora. Un `429` de estos límites es normal y el cliente reintenta.

### Rotar secretos

| Qué | Cómo |
| --- | --- |
| Clave de Resend | Crea una nueva en Resend, cámbiala en las variables de Railway y revoca la vieja. Railway vuelve a desplegar al cambiar una variable. |
| Secreto de invitaciones | Cámbialo en las variables. Revoca todos los Enlaces de invitación a la vez. |
| Contraseña de la base | Desde el servicio PostgreSQL de Railway. `QUITSIE_DATABASE_URL` la toma por referencia, así que basta con volver a desplegar la aplicación. |

## Límites conocidos

- **Todos los visitantes cuentan como la IP del proxy de Railway.** El servidor solo cree la IP del visitante si la conexión llega por loopback, que era como entraba `cloudflared` en la VM ([limits.go](../api/internal/httpapi/limits.go)). En Railway llega desde su proxy. Los límites por IP (300 peticiones por minuto, 60 verificaciones de código por hora) los comparten entonces todos los Usuarios. Está en [pendientes](pendientes.md#2-seguridad).
- **Una sola instancia.** Los límites de peticiones viven en la memoria del proceso. Con dos réplicas, cada una contaría por su lado.

## Si algo falla

| Síntoma | Causa probable |
| --- | --- |
| El despliegue falla en la comprobación de salud y el log nombra una variable `QUITSIE_…` | Falta una variable obligatoria o tiene un valor inválido ([Variables](#variables)). |
| El log dice `open database` | `QUITSIE_DATABASE_URL` no apunta a la base, o la base no está arrancada. |
| El build falla en `npm run build` o en `go build` | El mismo fallo aparece en el job `image` de la CI. Reprodúcelo con `docker build .`. |
| Tras restaurar, `record-restore` dice «check the clock» | El reloj de la máquina donde lo ejecutas va tan adelantado que el salto pasaría de lo que la PWA lee con exactitud. Corrige la hora y repítelo. |
| Muchos `429` con poco tráfico | Los límites por IP ([Límites conocidos](#límites-conocidos)). |
| Resend muestra un rebote con `Message lacks SPF/DKIM/DMARC Auth Records`, o los códigos llegan a spam | Falta un registro DNS del correo, casi siempre el DMARC ([Dominio del correo](#dominio-del-correo)). |
