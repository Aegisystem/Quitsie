# Revisión de seguridad

Revisión previa a abrir Quitsie al público, hecha el 2026-10-31 sobre el commit `26948a3` de `main`. El vocabulario sigue [CONTEXT.md](../CONTEXT.md). No se cambió código: este archivo es el único resultado.

> **El repositorio es público y este documento explica cómo reproducir ataques que hoy funcionan.** No lo lleves a `main` hasta arreglar los hallazgos 1 a 4.

## Resumen

Hay **13 hallazgos verificados: 1 crítico, 2 altos, 7 medios y 3 bajos**. **Cuatro bloquean abrir al público** (1 a 4). El aislamiento entre libros aguantó todo lo que se le probó: sin un enlace válido, nadie leyó ni cambió un libro ajeno. Los problemas graves son de disponibilidad, y el único camino hacia una cuenta ajena es adivinar su código (hallazgo 4).

| # | Gravedad | Hallazgo | ¿Bloquea? | Quién puede |
| --- | --- | --- | --- | --- |
| 1 | Crítica | Una petición sin sesión consume un minuto de CPU (Go 1.26.1) | **Sí** | Cualquiera |
| 2 | Alta | Un email de Miembro pendiente rompe `GET /sync` para todo el Grupo | **Sí** | Quien tenga el enlace |
| 3 | Alta | Una sola cuenta agota la memoria, la CPU y el disco | **Sí** | Cualquier Usuario |
| 4 | Media | Los intentos de código no tienen tope por cuenta | **Sí** | Cualquiera |
| 5 | Media | Cualquiera impide entrar a otra persona durante una hora | No | Cualquiera |
| 6 | Media | El envío de correos no tiene tope global | No | Cualquiera |
| 7 | Media | El limitador de códigos crece sin tope en memoria | No | Cualquiera |
| 8 | Media | Quien fue sacado de un Grupo vuelve con el enlace viejo | No | Antiguo Miembro |
| 9 | Media | La réplica sigue en el dispositivo tras cerrar la sesión a distancia | No | Quien tenga el dispositivo |
| 10 | Media | El email sobrevive al borrado de la cuenta y reaparece | No | Miembros del Grupo |
| 11 | Baja | Quien entra con el enlace ve los emails de los Miembros pendientes | No | Quien tenga el enlace |
| 12 | Baja | Un enlace falsificado carga el libro entero antes de comprobarse | No | Cualquiera con un id de libro |
| 13 | Baja | Los Saldos desbordan `int64` e invierten las Deudas | No | Miembro del Grupo |

Orden propuesto: el hallazgo 1 se arregla en una línea y conviene hacerlo hoy. Los hallazgos 10 y 12 también son cambios mínimos.

**Gravedad.** Crítica: alguien sin cuenta tumba el servicio o lee datos ajenos. Alta: una cuenta normal deja sin servicio a otras personas. Media: daño acotado a una persona o incumplimiento de una regla de privacidad. Baja: efecto pequeño o limitado al propio libro.

## Cómo se probó

Cada hallazgo se comprobó con un ataque real contra el binario local (`QUITSIE_ENV=development QUITSIE_MAIL=log`) en hosts `revseg.localhost` propios, con cuentas de prueba creadas para la ocasión, y el cliente en el navegador integrado. No se probó contra ningún servicio externo. Las cifras salen de un Mac con Apple Silicon, así que en una `e2-micro` el coste de CPU será mayor. Las pruebas de memoria y de la vecina del hallazgo 3 se hicieron con `GOMAXPROCS=2`, para acercarse a sus 2 vCPU.

El binario confía en `CF-Connecting-IP` por loopback, así que las pruebas rotaron esa cabecera en dos casos: para ahorrar tiempo de preparación y para simular muchas IP en el hallazgo 7. Cada ataque de un solo origen se hizo desde una IP fija.

## Hallazgos

### 1. Una petición sin sesión consume un minuto de CPU

**Crítica. Bloquea.** Cualquiera, sin cuenta.

**Archivos:** `api/go.mod:3`, `.github/workflows/ci.yml:28`, `api/internal/httpapi/limits.go:17` y `github.com/oapi-codegen/runtime/types/email.go`.

**Ataque.** `POST /api/v1/auth/login-code` es público y su cuerpo admite 2 MiB. El tipo generado `openapi_types.Email` llama a `mail.ParseAddress` al decodificar el JSON, antes de cualquier comprobación de longitud. En Go 1.26.1 ese analizador es cuadrático con comentarios anidados (`GO-2026-4986`). Un email formado por `x@y.test ` seguido de `(a` repetido cientos de miles de veces, con sus cierres, basta. El límite por IP permite 300 de estas peticiones por minuto, y `Origin` lo fija cualquier cliente que no sea un navegador. `/auth/login-code/verify` hereda lo mismo.

**Verificado.**

| Cuerpo | Go 1.26.1 | Go 1.26.10 |
| --- | --- | --- |
| 0,38 MB | 4,3 s de reloj, 8,6 s de CPU | 0,00 s |
| 1,57 MB | **58 s de reloj, 114 s de CPU** | 0,02 s |

Dos peticiones simultáneas bastarían para ocupar los 2 vCPU compartidos de la VM. La CI usa `go-version-file`, así que compila con la versión vulnerable exacta.

**Arreglo.**

1. Sube `api/go.mod` a `go 1.26.10`. Compilado así, el mismo código pasa de 42 avisos a `No vulnerabilities found` en `govulncheck` y el ataque queda en 0,02 s.
2. Pon un tope de cuerpo de unos 4 KiB en las rutas públicas de `/auth/`, para que el próximo fallo de un analizador no tenga 2 MiB con los que trabajar.
3. Añade `govulncheck ./...` a la CI, para que un Go desactualizado rompa el build en vez de pasar inadvertido.

### 2. Un email de Miembro pendiente rompe `GET /sync` para todo el Grupo

**Alta. Bloquea.** Cualquier Miembro vinculado, y por tanto cualquiera con el Enlace de invitación.

**Archivos:** `api/internal/httpapi/handlers_ledgers.go:88` (`cleanPendingMember`), `api/internal/httpapi/handlers_records.go:323` (`pulledMember`) y `handlers_ledgers.go:468` (`memberResponse`).

**Ataque.** El tipo `Email` no es simétrico. Al leer `"a b"@x.test` guarda `a b@x.test`, sin comillas, y ese valor ya no se puede volver a analizar al escribir la respuesta. El login lo rechaza con `normalizeEmail`, pero `cleanPendingMember` solo mira la longitud:

```http
POST /api/v1/invites/<token>/join
POST /api/v1/groups/<grupo>/members
{"name":"x","email":"\"a b\"@x.test"}
```

La segunda petición responde 500, pero la fila ya se guardó.

**Verificado.**

- `GET /sync` responde 500 a todos los Miembros del Grupo, con cursor o sin él, para siempre. La causa queda en el log: `json: error calling MarshalJSON for type *types.Email`.
- El daño alcanza **todos los libros de cada víctima**, porque el pull es uno solo. Un Usuario sin libros compartidos con el atacante sigue en 200.
- Los mismos efectos se obtienen con `PUT /groups/{id}/members/{id}`, con `pendingMembers` en `POST /groups`, y con `"a,b"@x.test`, `"a@b"@x.test` o `"a\"b"@x.test`. `POST /friendships` con ese email rompe solo a quien la crea.
- La interfaz no ofrece salida. El motor reintenta con espera creciente, la réplica se queda congelada y un dispositivo nuevo no recibe nada.
- Sacar al Miembro no lo arregla: responde 204 y el pull sigue en 500, porque el Antiguo Miembro también viaja. Después `PUT` responde 404.
- Corregirlo con `PUT` funciona, pero exige un id que ninguna respuesta da: el `POST` falló y `/sync` no carga.
- Salir del Grupo o borrarlo libera a quien lo haga, pero ambas cosas exigen Saldo cero, y el atacante puede registrar un Gasto antes.
- `type="email"` lo impide en el formulario, no en la API. Go 1.26.10 no lo corrige.

**Arreglo.**

1. Valida el email de los Miembros pendientes con la misma regla que el login: comparte `normalizeEmail` y responde `400 validation_failed` si el valor no sobrevive a la ida y vuelta.
2. Haz que una fila no pueda tumbar el pull: en las respuestas, declara el email del Miembro como texto sin `format: email`, para que el serializador no lo valide.
3. Añade una prueba: ese email recibe 400 y `/sync` sigue en 200.

No hace falta migrar datos porque aún no hay producción.

### 3. Una sola cuenta agota la memoria, la CPU y el disco

**Alta. Bloquea.** Cualquier Usuario.

**Archivos:** `api/internal/httpapi/handlers_ledgers.go:475` (sin tope de Miembros), `api/internal/sync/pull.go:65` y `:129` (`GET /sync` sin paginar, Deudas de cada libro cambiado), `api/internal/domain/debt.go:26`, `api/cmd/server/main.go:114` (solo `ReadHeaderTimeout`).

**Ataque.** El tope de 100 registros por lote limita una petición, no el total: no hay máximo de Miembros por Grupo, de Grupos por Usuario ni de registros por libro. `GET /sync` construye la respuesta entera en memoria y recalcula las Deudas en cada pull.

**Verificado**, siempre con una cuenta y dentro de los 2 MiB:

| Recurso | Qué hizo la cuenta | Resultado |
| --- | --- | --- |
| Disco | 60 lotes válidos de 408 KB en 17 s | 69 MB. **1,15 MB por petición, 2,8 veces lo enviado** |
| CPU | 1.000 Miembros y Gastos de 500 pagadores por 500 deudores, sin Simplificación | Cada petición de 2 MiB añade unos **0,5 s de CPU a todo `GET /sync` posterior** |
| Memoria | 8 `GET /sync` simultáneos sobre esos libros (87 MB de base) | **2.095 MB de memoria**, usando 32 de las 300 peticiones por minuto |
| Otras personas | Vecina sin libros compartidos, durante lo anterior | `POST /records` pasa de 4 a 113 ms y `GET /me` de 1 a 87 ms, sin fallos |

La preparación lleva unos 4 minutos respetando el límite. La VM tiene 1 GB: el proceso no cabe, y al reiniciarse pierde también los contadores de los hallazgos 4 y 7. Al ritmo que permite el límite (345 MB por minuto) se llenarían los 30 GB de disco en hora y media, y Litestream subiría todo eso a R2. Ninguna de estas dos consecuencias se probó en la VM.

**Arreglo.**

1. Pon topes de producto en el servidor: Miembros por Grupo (200 sobra), Grupos por Usuario, registros por libro. El contrato ya tiene `maxItems: 50` al crear un Grupo, pero `AddPendingMember` no tiene nada parecido.
2. Pagina `GET /sync`: un máximo de filas por respuesta, con `hasMore`, y escritura en streaming en vez de un único objeto en memoria.
3. Permite como mucho dos pulls grandes a la vez con un semáforo, y responde `429` al resto.
4. Pon un límite de bytes escritos por Usuario y hora, además del de peticiones.
5. Fija `GOMEMLIMIT=700MiB` en `deploy/quitsie.service`, y `WriteTimeout` e `IdleTimeout` en `http.Server`.

### 4. Los intentos de código no tienen tope por cuenta

**Media. Bloquea**, porque es el único camino hacia la cuenta de otra persona y arreglarlo cuesta poco.

**Archivos:** `api/internal/auth/code.go:25`, `:62` y `:75-99`, `api/internal/auth/ratelimit.go:13-14`.

**Ataque.** Los 5 intentos son por código, y pedir un código nuevo los pone a cero. Verificar no tiene límite propio: solo pasa por el general de 300 peticiones por minuto.

**Verificado.** Desde una IP se probaron 5 códigos con 5 intentos cada uno: **25 intentos por hora sobre un email**, y el sexto código recibe `429`. Los contadores viven en memoria: tras reiniciar el binario, el siguiente pedido responde 204. Cada despliegue y cada caída los borra.

Eso da un 1,8 % al mes contra una cuenta concreta, a cambio de 3.600 correos que la víctima vería. Repartido, es peor: una IP puede pedir 30 códigos por hora para 30 emails distintos, o sea 150 intentos. Con 100 direcciones son 15.000 intentos por hora, es decir, **una cuenta cada 3 días** de media sobre una lista de al menos 600 emails. Además, por lectura de `code.go:75-99`, el código se guarda antes de enviar el correo: si el envío falla o la cuota de Resend se agotó, el código sigue siendo válido y la víctima no recibe aviso.

**Arreglo.** Guarda en la base un presupuesto de intentos fallidos por email, que no se reinicie con cada código (por ejemplo 10 al día), y añade un límite de verificaciones fallidas por IP. Borra el código si el envío falla. Persistir en SQLite también cubre el hallazgo 7.

### 5. Cualquiera impide entrar a otra persona durante una hora

**Media.** Cualquiera, sin cuenta.

**Archivos:** `api/internal/auth/code.go:62` y `:127`, `api/internal/auth/ratelimit.go:13`.

**Ataque.** Basta conocer el email. Hay dos formas, y las dos se pueden repetir cada hora con unas pocas peticiones desde una IP.

**Verificado.**

- Cinco peticiones del atacante agotan el cupo: la víctima, desde su propia IP, recibe `429 rate_limited` con `Retry-After: 3600`.
- Si la víctima ya pidió su código, cinco intentos fallidos del atacante lo queman: al escribir el código correcto recibe `429 too_many_attempts`.

Las sesiones abiertas no se ven afectadas: el daño recae en quien estrena dispositivo o quedó sin sesión.

**Arreglo.** Ata cada código al navegador que lo pidió con una cookie de flujo `HttpOnly`, y cuenta los intentos por flujo, de modo que las conjeturas de otro no gasten el código de la víctima. Cuenta el cupo de envíos por pareja de email e IP y deja el tope por email como un techo más alto.

### 6. El envío de correos no tiene tope global

**Media.** Cualquiera, sin cuenta.

**Archivos:** `api/internal/auth/ratelimit.go:14`, `api/internal/auth/code.go:62`.

**Ataque.** Una IP envía 30 correos por hora a direcciones distintas, y nada limita el total. A ese ritmo, una sola IP gasta los 3.000 correos mensuales gratuitos de Resend en 4 días. Después nadie puede entrar con código. Con direcciones inexistentes, los rebotes ponen además en riesgo la cuenta de Resend. El límite por IP agrupa las IPv6 por /64, pero un cliente con un /56 tiene 256 de esos bloques.

**Verificado.** Treinta envíos aceptados desde una IP a direcciones distintas y `429` en el siguiente. El plus-addressing (`ana+1@…`) cuenta como direcciones distintas. El agotamiento de la cuota real no se probó.

**Arreglo.** Añade un tope global por hora y por día que deje de enviar y avise. Baja `maxSendsPerIP` a 10 y agrupa las IPv6 por /56 para este límite. Si llega el abuso, añade Cloudflare Turnstile, gratuito, delante de pedir código.

### 7. El limitador de códigos crece sin tope en memoria

**Media.** Cualquiera, sin cuenta.

**Archivos:** `api/internal/auth/ratelimit.go:46` y `:64`.

**Ataque.** `allow` escribe en el mapa la clave de cada email antes de decidir, también cuando la IP ya agotó su cupo y la petición se rechaza. Cada email nuevo deja una clave, y el barrido solo pasa una vez por hora.

**Verificado.** 90.000 peticiones con 300 IP subieron la memoria de 18 a 93 MB, unos 830 bytes por clave en el peor caso (emails de casi 254 caracteres). Una IP sostiene 18.300 claves por hora, y cada clave vive hasta dos horas: unos 30 MB. Con unas 35 direcciones se llena 1 GB, y un /56 de IPv6 da 256. El llenado completo de la VM no se probó.

**Arreglo.** No escribas la clave si no hay eventos, comprueba primero la cuota de IP y pon un tope de claves, como ya hace `internal/ratelimit`. Si los contadores pasan a SQLite por el hallazgo 4, este desaparece.

### 8. Quien fue sacado de un Grupo vuelve con el enlace viejo

**Media.** Antiguo Miembro que conserve el enlace.

**Archivos:** `api/internal/httpapi/handlers_ledgers.go:583` y `:844`, `web/src/ui/screens/LedgerSettings.tsx:304`.

**Ataque.** Ana saca a Eva de un Grupo por un conflicto. El enlace no cambia, Eva lo conserva en su chat, y con él vuelve al Grupo y recupera todo el historial, sin que nadie lo apruebe.

**Verificado.** Tras `DELETE /groups/{id}/members/{id}`, `POST /invites/{token}/join` responde 201 y el siguiente pull trae el Grupo completo. La interfaz no propone regenerar el enlace al sacar a alguien.

La regla actual dice que quien vuelve recupera su Antiguo Miembro, lo cual tiene sentido para quien salió por su cuenta, pero anula el efecto de sacar a alguien.

**Arreglo.** Regenera el enlace en la misma escritura de `RemoveMember` cuando el Miembro estaba vinculado. Es lo más simple, a costa de invalidar enlaces ya enviados a otras personas. La alternativa es marcar al Miembro como sacado y exigir que otro Miembro lo readmita. Es una decisión de producto, y debe quedar en `docs/reglas-de-negocio.md`.

### 9. La réplica sigue en el dispositivo tras cerrar la sesión a distancia

**Media.** Quien tenga el dispositivo, abriendo el almacenamiento del navegador.

**Archivos:** `web/src/session/replica.ts:5`, `web/src/session/session.ts:87` y `:109`.

**Ataque.** Alguien pierde el teléfono, o entró en un ordenador prestado, y cierra esa sesión desde otro dispositivo, o borra su cuenta. La pantalla pide entrar de nuevo y da a entender que todo terminó, pero la réplica sigue allí: es deliberado, para que la misma persona continúe al volver.

**Verificado en el navegador.**

- Sesión cerrada desde otro dispositivo: pantalla de entrada, y en IndexedDB siguen los 7 Gastos, 8 Miembros y 9 Actividades.
- Cuenta borrada desde otro dispositivo: siguen el Grupo y el email de una tercera persona.
- Cerrar sesión desde la propia app sí borra todo: 8 tablas vacías y `localStorage` vacío. Entrar como otro Usuario también vacía lo anterior.

**Arreglo.** Ante cualquier 401, borra la réplica y conserva solo la cola de registros, que es lo único que no se puede recuperar. Volver a entrar la reconstruye con un pull. Queda un límite que conviene explicar en la pantalla de sesiones: un dispositivo que nunca vuelve a conectarse no se entera.

### 10. El email sobrevive al borrado de la cuenta y reaparece

**Media.** Lo ven los Miembros del Grupo. Incumple [reglas-de-negocio.md](reglas-de-negocio.md): «De la cuenta se borran el email y el nombre; solo queda el nombre guardado en cada Miembro».

**Archivos:** `api/internal/store/queries/members.sql:20` (`UnlinkMembersOfUser`).

**Ataque.** Ana añade a Pepa con su email. Pepa Reclama ese Miembro, y mientras está vinculada su email deja de mostrarse. Pepa borra su cuenta y confía en que el email desapareció.

**Verificado.** La fila de `users` queda vacía, pero `members.email` conserva la dirección, y como el Miembro vuelve a estar pendiente, `GET /sync` la entrega de nuevo a todo el Grupo y a quien entre después. Además, los respaldos guardan el estado anterior durante 7 días (`deploy/litestream.yml`), lo que conviene decir en la política de privacidad.

**Arreglo.** Añade `email = NULL` a `UnlinkMembersOfUser`, con una prueba. También puede borrarse al Reclamar, ya que a partir de ahí no sirve para nada.

### 11. Quien entra con el enlace ve los emails de los Miembros pendientes

**Baja.** Quien tenga el enlace y una cuenta.

**Archivos:** `api/internal/httpapi/handlers_records.go:323`, `handlers_ledgers.go:468`.

**Ataque.** `GET /invites/{token}` oculta los emails con cuidado, pero tras unirse al Grupo `GET /sync` los entrega todos. Si el enlace circula en un chat grande, cualquiera recoge los emails de personas que nunca entraron en la aplicación. Según el contrato, ese email solo sirve «para sugerir la Reclamación», y esa sugerencia la calcula el servidor.

**Verificado.** Un Usuario recién unido por enlace recibe `pepa@test.example` en su pull.

**Arreglo.** Muestra el email solo a quien lo añadió, o enmascarado (`p***@test.example`). Afecta a la pantalla de editar un Miembro pendiente, así que es una decisión de producto.

### 12. Un enlace falsificado carga el libro entero antes de comprobarse

**Baja.** Cualquiera con un id de libro, sin cuenta. Los ids viajan dentro de cada enlace, así que los tiene cualquiera que haya visto uno.

**Archivos:** `api/internal/httpapi/handlers_ledgers.go:649-661` (`openInvite`).

**Ataque.** `openInvite` lee el id del token sin comprobarlo y llama a `ledgers.Load`, que trae todos los registros. El HMAC se comprueba después.

**Verificado.** Con un token inventado y sin sesión, responde 404 en 369 ms sobre un libro de 6.000 Gastos y en 0,3 ms sobre un id inexistente. Sirve para gastar CPU y además revela si un libro existe.

**Arreglo.** Carga solo la fila con `GetLedger`, comprueba `invites.Valid` y llama a `ledgers.Load` únicamente si es válido.

### 13. Los Saldos desbordan `int64` e invierten las Deudas

**Baja.** Miembro del Grupo. Solo afecta a su propio libro, donde ya podía poner cualquier cifra.

**Archivos:** `api/internal/domain/balance.go:30-33`, `api/internal/domain/validate_expense.go:108`, `docs/reglas-de-negocio.md:11`.

**Ataque.** El tope de 10^15 por importe existe «para que las sumas de Saldos nunca desborden», pero no limita cuántos registros se suman.

**Verificado.** 9.300 Gastos de 10^15 en 93 peticiones y 3 segundos. Lo real es que la otra persona debe 9.300.000.000.000.000.000; `GET /sync` dice que el pagador debe 9.146.744.073.709.551.616 a esa persona, con el sentido invertido. No hubo pánicos.

**Arreglo.** Rechaza al escribir cualquier registro que deje un Saldo por encima de 2^62, con `invalid_amounts`. El tope de registros del hallazgo 3 también lo cierra. Corrige la frase de las reglas.

## A comprobar al desplegar

Son afirmaciones de la documentación que nada en el repositorio cumple. Dependen del panel de Cloudflare, así que no se pudieron probar.

- **HTTPS forzado y HSTS.** Ni el asistente ni la documentación activan «Always Use HTTPS» ni HSTS, y `quitsie.com` no está en la lista de precarga, a diferencia de `.app`. Sin ellos, quien escriba `http://quitsie.com` en una red hostil puede recibir una página alterada que capture su email y su código. Actívalos en la etapa 6 del asistente.
- **El «límite de Cloudflare».** `docs/arquitectura.md:134` y `docs/despliegue.md:122` lo mencionan, pero nada lo crea: hay que definir una regla. Añade una para `/api/v1/auth/*` que además limite el tamaño del cuerpo.
- **`CF-Connecting-IP` a través del túnel real.** Comprueba tras el primer despliegue que dos visitantes distintos tienen cubos distintos y que el encabezado enviado por un cliente se sobrescribe.

## Endurecimiento sin ataque demostrado

No son hallazgos: no se encontró forma de aprovecharlos. Conviene añadirlos por si otra cosa falla.

- **Cabeceras.** Ninguna respuesta lleva `Content-Security-Policy`, `X-Content-Type-Options`, `frame-ancestors` ni `Referrer-Policy`. Con los scripts propios, una CSP estricta cabe sin esfuerzo, y sería la última barrera si un XSS futuro intentara leer la réplica. Enmarcar la app no sirve de nada hoy porque `SameSite=Lax` deja fuera la cookie.
- **API sin `Cache-Control: no-store`.** Las respuestas de `/api/` no lo declaran. No se inspeccionó la caché HTTP del navegador tras cerrar sesión.
- **Cookie con prefijo `__Host-`**, para que un subdominio nunca pueda fijarla.
- **Duración máxima de sesión.** Hoy se renueva indefinidamente mientras se use.
- **Borrar la cuenta sin código reciente.** Una sesión robada basta para hacerlo.
- **GitHub Actions por etiqueta.** Fijarlas por SHA protege el flujo que maneja la clave de despliegue.

## Revisado sin hallazgos

| Área | Qué se intentó | Resultado |
| --- | --- | --- |
| Autorización | 20 operaciones sobre un libro ajeno, con un extraño, un Antiguo Miembro y uno sacado, incluido `POST /records` | 404 o `ledger_unavailable` siempre, datos intactos, y `GET /sync` sin nada del libro. Un Miembro real sí pudo |
| Ids cruzados | Miembros ajenos en un libro propio, ids de registros ajenos, Amistad usada como Grupo y al revés, sesión de otro | `invalid_participants`, `id_conflict` y 404 |
| Antiguos Miembros | Pull tras salir, tras ser sacados y tras deshacer la Reclamación | Solo reciben el id del libro para soltarlo |
| Códigos | Reutilización, código anterior, 6 intentos, 40 intentos simultáneos | Un solo uso. De los 40 simultáneos contaron exactamente 5. La caducidad está cubierta por pruebas con reloj falso |
| Enumeración | Pedir código con y sin cuenta | Misma respuesta |
| Análisis del email en el login | Listas, `Bcc:` en saltos de línea, comillas, nombre visible | Rechazados, o reducidos a la dirección que recibe el correo |
| Sesiones y cookies | Token tras cerrar sesión, tras borrar la cuenta, todos los dispositivos | 401. Token de 256 bits guardado con hash, cookie `HttpOnly; Secure; SameSite=Lax`, `document.cookie` vacío |
| Google (lectura y pruebas con proveedor falso) | `state`, `nonce`, PKCE S256, `email_verified`, `returnTo` | Correctos. El enlace por email solo acepta emails verificados |
| CSRF | Sin `Origin`, ajeno, `null`, subdominio, prefijo, solo `Referer` | 403 en los 6, operaciones públicas incluidas. Ningún `GET` cambia datos |
| CORS | Preflight y `GET` con `Origin` ajeno | Sin ninguna cabecera `Access-Control-*` |
| Enlaces de invitación | Bit cambiado, truncado, largo, regenerado, Grupo borrado, enlace de Amigo gastado o usado para unirse | 404 o 400. HMAC de 128 bits, secreto de 32 bytes o más y fuera de la base |
| Filtración del enlace | Log tras todas las pruebas | El token no aparece. El log solo incluye la ruta en errores 500. La ruta de la PWA va tras `#`, aunque la llamada a la API lo lleva en el path |
| Datos en respuestas | Emails en `/sync` y `GET /invites/{token}` | Ninguno de Miembros vinculados. Sesiones limitadas a las propias |
| Correo de log | Producción con `QUITSIE_MAIL=log` | El servidor no arranca |
| XSS | 7 cargas en nombres y descripciones, en 10 pantallas | Sin ejecución, sin nodos inyectados. No hay `dangerouslySetInnerHTML` ni `innerHTML` |
| Service worker (lectura del `sw.js` generado) | Rutas y lista de precarga | Solo archivos estáticos. `/api/` queda excluido y no hay rutas de datos |
| Valores extremos | NUL, sustitutos sueltos, años 0 y 9999, peso `int64`, cambio de moneda al límite | Se guardan y vuelven. Sin pánicos en el log |
| Archivos estáticos | Salto de carpetas, engaño de caché con `.css` y `.jpg` | Sin acceso, y 404 en las rutas de API |
| Cookie cambiada bajo la app | Otro Usuario entra por detrás de la sesión guardada | El siguiente pull suelta los libros anteriores, sin fuga |

## Dependencias

| Herramienta | Resultado |
| --- | --- |
| `govulncheck` (Go 1.26.1) | **42 avisos, todos de la biblioteca estándar: 27 alcanzables, 6 importados, 9 solo en el módulo** |
| `govulncheck` (Go 1.26.10) | `No vulnerabilities found`, sin cambiar código |
| `npm audit --omit=dev` | 0 vulnerabilidades en lo que llega al navegador |
| `npm audit` | 12 (10 moderadas, 2 bajas), todas en herramientas de compilación |

De los 27 alcanzables, los que importan a este servidor son los de `net/mail` (hallazgo 1), el tamaño de `Range` en archivos estáticos (`GO-2026-6609`) y los de `crypto/x509` en el cliente de Resend. Los de HTTP/2 y TLS del servidor quedan lejos porque `cloudflared` llega por HTTP/1.1 sin TLS, aunque la subida los resuelve igual.

Las de npm vienen de `vite-plugin-pwa` (`workbox-build`, `ejs`, `glob`, `deepmerge`) y `openapi-typescript` (`minimatch`). No llegan al navegador; se resuelven actualizando esas dos herramientas cuando publiquen versiones nuevas. `go-jose/v4` tiene una versión más reciente (4.1.7), sin avisos conocidos para la actual.

## Lo que no se pudo verificar

- **Cloudflare real.** No hay despliegue: ni el túnel, ni `CF-Connecting-IP` a través de él, ni si el borde frenaría alguno de estos ataques. La regla de loopback se comprobó por lectura y por `limits_test.go`. La prueba con una dirección de red real se hizo al endurecer el servidor, y no se repitió para no exponer el binario en la red local.
- **Google real.** Solo el proveedor falso de las pruebas.
- **Resend real.** Ni cuotas ni rebotes.
- **La VM `e2-micro`.** La caída por memoria y el llenado del disco se deducen de lo medido en local.
- **Service worker en ejecución.** No llegó a registrarse en el navegador integrado, que falló al obtener el script, aunque el servidor lo entrega con 200. La conclusión sale de leer el `sw.js` generado.
- **Safari en iOS y la PWA instalada.**
- **La caché HTTP del navegador** tras cerrar sesión.
