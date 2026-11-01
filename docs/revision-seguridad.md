# Revisión de seguridad

Revisión previa a abrir Quitsie al público, hecha el 2026-10-31 sobre el commit `26948a3` de `main`. El vocabulario sigue [CONTEXT.md](../CONTEXT.md). Los hallazgos 1 a 4, 7, 10 y 12 se arreglaron después, y cada uno dice con qué commits y qué se volvió a medir.

> **El repositorio es público y este documento explica cómo reproducir ataques.** Los hallazgos 5, 6, 8, 9, 11, 13 y 14 siguen abiertos.

## Resumen

Hay **14 hallazgos verificados: 1 crítico, 2 altos, 7 medios y 4 bajos**. **Los cuatro que bloqueaban abrir al público (1 a 4) están arreglados**, y también el 7, el 10 y el 12. El aislamiento entre libros aguantó todo lo que se le probó: sin un enlace válido, nadie leyó ni cambió un libro ajeno. Los problemas graves son de disponibilidad, y el único camino hacia una cuenta ajena es adivinar su código (hallazgo 4).

| # | Gravedad | Hallazgo | Estado | Quién puede |
| --- | --- | --- | --- | --- |
| 1 | Crítica | Una petición sin sesión consume un minuto de CPU (Go 1.26.1) | **Arreglado** | Cualquiera |
| 2 | Alta | Un email de Miembro pendiente rompe `GET /sync` para todo el Grupo | **Arreglado** | Quien tenga el enlace |
| 3 | Alta | Una sola cuenta agota la memoria, la CPU y el disco | **Arreglado** | Cualquier Usuario |
| 4 | Media | Los intentos de código no tienen tope por cuenta | **Arreglado** | Cualquiera |
| 5 | Media | Cualquiera impide entrar a otra persona durante un día | Abierto, empeoró con el arreglo del 4 | Cualquiera |
| 6 | Media | El envío de correos no tiene tope global | Abierto | Cualquiera |
| 7 | Media | El limitador de códigos crece sin tope en memoria | **Arreglado** | Cualquiera |
| 8 | Media | Quien fue sacado de un Grupo vuelve con el enlace viejo | Abierto | Antiguo Miembro |
| 9 | Media | La réplica sigue en el dispositivo tras cerrar la sesión a distancia | Abierto | Quien tenga el dispositivo |
| 10 | Media | El email sobrevive al borrado de la cuenta y reaparece | **Arreglado** | Miembros del Grupo |
| 11 | Baja | Quien entra con el enlace ve los emails de los Miembros pendientes | Abierto | Quien tenga el enlace |
| 12 | Baja | Un enlace falsificado carga el libro entero antes de comprobarse | **Arreglado** | Cualquiera con un id de libro |
| 13 | Baja | Los Saldos desbordan `int64` e invierten las Deudas | Abierto | Miembro del Grupo |
| 14 | Baja | Pedir un código recorre toda la tabla con el bloqueo de escritura | Abierto | Cualquiera con más de mil direcciones |

Los hallazgos 8 y 11 necesitan una decisión de producto.

**Gravedad.** Crítica: alguien sin cuenta tumba el servicio o lee datos ajenos. Alta: una cuenta normal deja sin servicio a otras personas. Media: daño acotado a una persona o incumplimiento de una regla de privacidad. Baja: efecto pequeño o limitado al propio libro.

## Cómo se probó

Cada hallazgo se comprobó con un ataque real contra el binario local (`QUITSIE_ENV=development QUITSIE_MAIL=log`) en hosts `revseg.localhost` propios, con cuentas de prueba creadas para la ocasión, y el cliente en el navegador integrado. No se probó contra ningún servicio externo. Las cifras salen de un Mac con Apple Silicon, así que en una `e2-micro` el coste de CPU será mayor. Las pruebas de memoria y de la vecina del hallazgo 3 se hicieron con `GOMAXPROCS=2`, para acercarse a sus 2 vCPU.

El binario confía en `CF-Connecting-IP` por loopback, así que las pruebas rotaron esa cabecera en dos casos: para ahorrar tiempo de preparación y para simular muchas IP en el hallazgo 7. Cada ataque de un solo origen se hizo desde una IP fija.

## Hallazgos

### 1. Una petición sin sesión consume un minuto de CPU

**Crítica. Arreglado en `6e1fe10`.** Cualquiera, sin cuenta.

**Archivos:** `api/go.mod:3`, `.github/workflows/ci.yml:28`, `api/internal/httpapi/limits.go:17` y `github.com/oapi-codegen/runtime/types/email.go`.

**Ataque.** `POST /api/v1/auth/login-code` es público y su cuerpo admite 2 MiB. El tipo generado `openapi_types.Email` llama a `mail.ParseAddress` al decodificar el JSON, antes de cualquier comprobación de longitud. En Go 1.26.1 ese analizador es cuadrático con comentarios anidados (`GO-2026-4986`). Un email formado por `x@y.test ` seguido de `(a` repetido cientos de miles de veces, con sus cierres, basta. El límite por IP permite 300 de estas peticiones por minuto, y `Origin` lo fija cualquier cliente que no sea un navegador. `/auth/login-code/verify` hereda lo mismo.

**Verificado.**

| Cuerpo | Go 1.26.1 | Go 1.26.10 |
| --- | --- | --- |
| 0,38 MB | 4,3 s de reloj, 8,6 s de CPU | 0,00 s |
| 1,57 MB | **58 s de reloj, 114 s de CPU** | 0,02 s |

Dos peticiones simultáneas bastarían para ocupar los 2 vCPU compartidos de la VM. La CI usa `go-version-file`, así que compila con la versión vulnerable exacta.

**Arreglo.**

1. `api/go.mod` pasó a `go 1.26.10`. `govulncheck` pasa de 42 avisos a `No vulnerabilities found`.
2. `POST /auth/*` lee 4 KiB como máximo, y el servidor responde `413` sin analizar el resto.
3. La CI ejecuta `govulncheck ./...`. Un aviso nuevo pone la CI en rojo y frena el despliegue hasta actualizar.

**Medido después, contra el binario real.** El cuerpo de 1,57 MB responde `413` en 14 ms, sin CPU apreciable.

### 2. Un email de Miembro pendiente rompe `GET /sync` para todo el Grupo

**Alta. Arreglado en `033ad77`.** Cualquier Miembro vinculado, y por tanto cualquiera con el Enlace de invitación.

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

1. El email de los Miembros y Amigos pendientes pasa por `auth.NormalizeEmail`, la regla del login. Un valor inválido recibe `400 validation_failed`.
2. En las respuestas, `Member.email` es texto sin `format: email`, así que una fila guardada ya no puede romper el pull. Una prueba lo comprueba con una fila dañada escrita directamente en la base.

**Medido después, contra el binario real.** Los cuatro emails del ataque reciben 400 por `POST`, `PUT`, `POST /groups` y `POST /friendships`, y `GET /sync` sigue en 200 para todo el Grupo. No hizo falta migrar datos porque aún no hay producción.

### 3. Una sola cuenta agota la memoria, la CPU y el disco

**Alta. Arreglado en `c03f82e`, `764952b`, `8c850d4`, `ca09625` y `c7340d6`.** Cualquier Usuario.

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

1. Topes: 200 Miembros por Grupo, 200 libros por Usuario y 200.000 filas por libro. Un libro lleno solo admite borrar registros y las salidas (salir, sacar a un Miembro, deshacer una Reclamación, borrar el Grupo), para que ningún Miembro atrape a los demás. Los rechazos usan el código nuevo `limit_reached`.
2. Presupuesto de 5 MiB de cambios por Usuario y hora. Leer y todo `/auth/` quedan fuera, para que cerrar sesión siempre funcione.
3. `GET /sync` corta cada respuesta en unas 20.000 filas. El cliente ya recorría `hasMore`, así que no cambió.
4. Dos pulls a la vez y uno por Usuario. El resto espera hasta 5 segundos o recibe `429`.
5. `GOMEMLIMIT=700MiB` en `deploy/quitsie.service`, y `WriteTimeout` e `IdleTimeout` en `http.Server`.

**Medido después, contra el binario real con `GOMAXPROCS=2`.** Para la memoria y la vecina se preparó el peor caso dentro de los topes: una cuenta con 5 libros llenos de 199.000 filas, 200 Miembros y Gastos de 100 pagadores por 100 deudores, sin Simplificación (369 MB de base).

| Medida | Antes | Después |
| --- | --- | --- |
| Memoria con 8 lectores simultáneos | 2.095 MB | **150 MB** |
| `POST /records` de la vecina, p50 | 113 ms | 4 ms |
| `GET /me` de la vecina, p50 | 87 ms | menos de 1 ms |
| `GET /sync` de la vecina, p50 | No medido | 3 ms, 11 ms de máximo |
| Respuesta más grande de `GET /sync` | Todo el historial | 4,1 MB |
| Disco, con una cuenta | 345 MB por minuto posibles | Lote rechazado al llegar a 4,9 MB enviados |

Un pull completo de esa cuenta tarda 40 s y 50 páginas. El cliente real, en el navegador, recorrió dos páginas de un libro de 36.000 filas y guardó los 12.000 Gastos con el Saldo correcto.

**Riesgo que queda.** Los límites son por cuenta, y el presupuesto y los pulls simultáneos viven en memoria. Una persona con varias cuentas los suma, y cada IP puede pedir 30 códigos por hora. Dos cuentas con libros llenos pueden ocupar las dos plazas de `GET /sync`.

### 4. Los intentos de código no tienen tope por cuenta

**Media. Arreglado en `835a94c`.** Bloqueaba porque es el único camino hacia la cuenta de otra persona.

**Archivos:** `api/internal/auth/code.go:25`, `:62` y `:75-99`, `api/internal/auth/ratelimit.go:13-14`.

**Ataque.** Los 5 intentos son por código, y pedir un código nuevo los pone a cero. Verificar no tiene límite propio: solo pasa por el general de 300 peticiones por minuto.

**Verificado.** Desde una IP se probaron 5 códigos con 5 intentos cada uno: **25 intentos por hora sobre un email**, y el sexto código recibe `429`. Los contadores viven en memoria: tras reiniciar el binario, el siguiente pedido responde 204. Cada despliegue y cada caída los borra.

Eso da un 1,8 % al mes contra una cuenta concreta, a cambio de 3.600 correos que la víctima vería. Repartido, es peor: una IP puede pedir 30 códigos por hora para 30 emails distintos, o sea 150 intentos. Con 100 direcciones son 15.000 intentos por hora, es decir, **una cuenta cada 3 días** de media sobre una lista de al menos 600 emails. Además, por lectura de `code.go:75-99`, el código se guarda antes de enviar el correo: si el envío falla o la cuota de Resend se agotó, el código sigue siendo válido y la víctima no recibe aviso.

**Arreglo.**

1. Un email admite 10 códigos equivocados en 24 horas, sumando todos sus códigos. La cuenta vive en la tabla `login_failures`, con el hash del email.
2. Una IP admite 60 verificaciones por hora.
3. Si el correo no se pudo enviar, el código se borra.

**Medido después, contra el binario real.** Un email recibe 10 intentos en vez de 25, y el tercer código responde `429` con `Retry-After: 86400`. El código correcto también se rechaza. Tras reiniciar el binario, la respuesta sigue siendo `429`. Una IP recibe `429` en la verificación 61.

Con este tope, una cuenta concreta acumula 3.650 intentos sobre un millón en un año.

**Coste.** Cualquiera puede bloquear los códigos de un email durante un día: ver el hallazgo 5.

### 5. Cualquiera impide entrar a otra persona durante un día

**Media. Abierto.** Cualquiera, sin cuenta. El arreglo del hallazgo 4 lo empeoró: el bloqueo pasó de una hora a 24 horas. Antes de ese arreglo el atacante podía repetirlo cada hora, así que el bloqueo ya podía ser permanente; ahora le cuesta menos peticiones.

**Archivos:** `api/internal/auth/code.go:62` y `:127`, `api/internal/auth/ratelimit.go:13`.

**Ataque.** Basta conocer el email. Hay tres formas, y todas se pueden repetir con unas pocas peticiones desde una IP.

**Verificado.**

- Cinco peticiones del atacante agotan el cupo de envíos: la víctima, desde su propia IP, recibe `429 rate_limited` con `Retry-After: 3600`.
- Si la víctima ya pidió su código, cinco intentos fallidos del atacante lo queman: al escribir el código correcto recibe `429 too_many_attempts`.
- Diez intentos fallidos del atacante, con dos códigos, bloquean ese email durante 24 horas: la víctima recibe `429` con `Retry-After: 86400`, y un código correcto se rechaza.

Las sesiones abiertas y Google no se ven afectados: el daño recae en quien estrena dispositivo o quedó sin sesión y entra con código.

**Arreglo.** Ata cada código al navegador que lo pidió con una cookie de flujo `HttpOnly`, y cuenta los intentos por flujo y el cupo de envíos por pareja de email e IP. El tope por email de 10 fallos se mantiene como techo más alto, para que un atacante con muchas IP no recupere los intentos que quitó el hallazgo 4.

### 6. El envío de correos no tiene tope global

**Media.** Cualquiera, sin cuenta.

**Archivos:** `api/internal/auth/ratelimit.go:14`, `api/internal/auth/code.go:62`.

**Ataque.** Una IP envía 30 correos por hora a direcciones distintas, y nada limita el total. A ese ritmo, una sola IP gasta los 3.000 correos mensuales gratuitos de Resend en 4 días. Después nadie puede entrar con código. Con direcciones inexistentes, los rebotes ponen además en riesgo la cuenta de Resend. El límite por IP agrupa las IPv6 por /64, pero un cliente con un /56 tiene 256 de esos bloques.

**Verificado.** Treinta envíos aceptados desde una IP a direcciones distintas y `429` en el siguiente. El plus-addressing (`ana+1@…`) cuenta como direcciones distintas. El agotamiento de la cuota real no se probó.

**Arreglo.** Añade un tope global por hora y por día que deje de enviar y avise. Baja `maxSendsPerIP` a 10 y agrupa las IPv6 por /56 para este límite. Si llega el abuso, añade Cloudflare Turnstile, gratuito, delante de pedir código.

### 7. El limitador de códigos crece sin tope en memoria

**Media. Arreglado en `acd1907`.** Cualquiera, sin cuenta.

**Archivos:** `api/internal/auth/ratelimit.go:46` y `:64`.

**Ataque.** `allow` escribe en el mapa la clave de cada email antes de decidir, también cuando la IP ya agotó su cupo y la petición se rechaza. Cada email nuevo deja una clave, y el barrido solo pasa una vez por hora.

**Verificado.** 90.000 peticiones con 300 IP subieron la memoria de 18 a 93 MB, unos 830 bytes por clave en el peor caso (emails de casi 254 caracteres). Una IP sostiene 18.300 claves por hora, y cada clave vive hasta dos horas: unos 30 MB. Con unas 35 direcciones se llena 1 GB, y un /56 de IPv6 da 256. El llenado completo de la VM no se probó.

**Arreglo.** Una petición rechazada ya no deja clave, y cada limitador guarda 100.000 claves como máximo, como `internal/ratelimit`.

**Medido después, contra el binario real.** Tras cuatro tandas de 90.000 peticiones desde 1.200 IP, la memoria fue 60, 82, 89 y 90 MB: se estabiliza. Con el código anterior, esas 360.000 peticiones habrían dejado unos 300 MB de claves.

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

**Media. Arreglado en `6d602a2`.** Lo veían los Miembros del Grupo. Incumplía [reglas-de-negocio.md](reglas-de-negocio.md): «De la cuenta se borran el email y el nombre; solo queda el nombre guardado en cada Miembro».

**Archivos:** `api/internal/store/queries/members.sql:20` (`UnlinkMembersOfUser`).

**Ataque.** Ana añade a Pepa con su email. Pepa Reclama ese Miembro, y mientras está vinculada su email deja de mostrarse. Pepa borra su cuenta y confía en que el email desapareció.

**Verificado.** La fila de `users` queda vacía, pero `members.email` conserva la dirección, y como el Miembro vuelve a estar pendiente, `GET /sync` la entrega de nuevo a todo el Grupo y a quien entre después. Además, los respaldos guardan el estado anterior durante 7 días (`deploy/litestream.yml`), lo que conviene decir en la política de privacidad.

**Arreglo.** `UnlinkMembersOfUser` pone `email = NULL`, y el pull siguiente quita el email de las réplicas de los demás.

**Medido después, contra el binario real.** Tras borrar la cuenta, `GET /sync` no trae el email y `members.email` queda vacío en la base. Los respaldos lo conservan hasta 7 días.

### 11. Quien entra con el enlace ve los emails de los Miembros pendientes

**Baja.** Quien tenga el enlace y una cuenta.

**Archivos:** `api/internal/httpapi/handlers_records.go:323`, `handlers_ledgers.go:468`.

**Ataque.** `GET /invites/{token}` oculta los emails con cuidado, pero tras unirse al Grupo `GET /sync` los entrega todos. Si el enlace circula en un chat grande, cualquiera recoge los emails de personas que nunca entraron en la aplicación. Según el contrato, ese email solo sirve «para sugerir la Reclamación», y esa sugerencia la calcula el servidor.

**Verificado.** Un Usuario recién unido por enlace recibe `pepa@test.example` en su pull.

**Arreglo.** Muestra el email solo a quien lo añadió, o enmascarado (`p***@test.example`). Afecta a la pantalla de editar un Miembro pendiente, así que es una decisión de producto.

### 12. Un enlace falsificado carga el libro entero antes de comprobarse

**Baja. Arreglado en `eabfcf3`.** Cualquiera con un id de libro, sin cuenta. Los ids viajan dentro de cada enlace, así que los tiene cualquiera que haya visto uno.

**Archivos:** `api/internal/httpapi/handlers_ledgers.go:649-661` (`openInvite`).

**Ataque.** `openInvite` lee el id del token sin comprobarlo y llama a `ledgers.Load`, que trae todos los registros. El HMAC se comprueba después.

**Verificado.** Con un token inventado y sin sesión, responde 404 en 369 ms sobre un libro de 6.000 Gastos y en 0,3 ms sobre un id inexistente. Sirve para gastar CPU y además revela si un libro existe.

**Arreglo.** `openInvite` lee solo la fila con `GetLedger`, comprueba `invites.Valid` y llama a `ledgers.Load` únicamente si es válido.

**Medido después, contra el binario real.** Un token inventado tarda 0,2 ms, tanto con un libro de 198.990 filas como con un id inexistente. Un enlace real sobre ese libro tarda 242 ms.

### 13. Los Saldos desbordan `int64` e invierten las Deudas

**Baja.** Miembro del Grupo. Solo afecta a su propio libro, donde ya podía poner cualquier cifra.

**Archivos:** `api/internal/domain/balance.go:30-33`, `api/internal/domain/validate_expense.go:108`, `docs/reglas-de-negocio.md:11`.

**Ataque.** El tope de 10^15 por importe existe «para que las sumas de Saldos nunca desborden», pero no limita cuántos registros se suman.

**Verificado.** 9.300 Gastos de 10^15 en 93 peticiones y 3 segundos. Lo real es que la otra persona debe 9.300.000.000.000.000.000; `GET /sync` dice que el pagador debe 9.146.744.073.709.551.616 a esa persona, con el sentido invertido. No hubo pánicos.

**Arreglo.** Rechaza al escribir cualquier registro que deje un Saldo por encima de 2^62, con `invalid_amounts`. El tope de filas del hallazgo 3 no lo impide, porque 9.300 Gastos caben en un libro. Corrige la frase de las reglas.

### 14. Pedir un código recorre toda la tabla con el bloqueo de escritura

**Baja. Arreglado en `bf6c2c1`.** Cualquiera con más de mil direcciones, sin cuenta. Apareció al volver a medir el hallazgo 7.

**Archivos:** `api/internal/auth/code.go:94`, `api/internal/store/queries/login_codes.sql` y `api/migrations/00001_initial_schema.sql`.

**Ataque.** Cada `POST /auth/login-code` borra los códigos caducados dentro de su escritura, y `login_codes` no tiene índice por `expires_at`. SQLite recorre la tabla entera con el bloqueo tomado, y las demás escrituras esperan. Un código vive 10 minutos y una IP pide 30 por hora, así que 36.000 filas exigen unas 1.200 direcciones en esa ventana. Un IPv6 /53 tiene 2.048 bloques /64.

**Verificado.** Con 36.000 códigos vivos, cada tanda de 90.000 peticiones pasó de 11 a 75 s, y 21 peticiones aceptadas respondieron 500 con `database is locked`. `EXPLAIN QUERY PLAN` muestra `SCAN login_codes`. El efecto sobre otros Usuarios no se midió. Por lectura, `DeleteExpiredSessions` repite el patrón en cada inicio de sesión.

**Arreglo.** La migración 00003 añade índices sobre `login_codes (expires_at)`, `sessions (last_used_at)` y `login_failures (resets_at)`, que `DeleteEndedLoginFailures` también recorría en cada pedido de código. Una prueba comprueba con `EXPLAIN QUERY PLAN` que los tres borrados buscan por su índice.

**Medido después, contra el binario real**, en Linux con 4 vCPU y 36.000 códigos vivos sembrados en la base. «Sin índices» es la misma base con los tres índices quitados. El plan pasa a `SEARCH login_codes USING INDEX login_codes_expires_at`.

| Medida | Sin índices | Con índices |
| --- | --- | --- |
| Pedir un código, mediana de 300 seguidos | 16 ms | 1 ms |
| 3.000 pedidos, 30 a la vez | 54 s, 47 respondieron 500 con `database is locked` | 4 s, ningún 500 |

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
| `govulncheck` (Go 1.26.1, en la revisión) | **42 avisos, todos de la biblioteca estándar: 27 alcanzables, 6 importados, 9 solo en el módulo** |
| `govulncheck` (Go 1.26.10, actual) | `No vulnerabilities found` |
| `npm audit --omit=dev` | 0 vulnerabilidades en lo que llega al navegador |
| `npm audit` | 12 (10 moderadas, 2 bajas), todas en herramientas de compilación |

De los 27 alcanzables, los que importan a este servidor son los de `net/mail` (hallazgo 1), el tamaño de `Range` en archivos estáticos (`GO-2026-6609`) y los de `crypto/x509` en el cliente de Resend. Los de HTTP/2 y TLS del servidor quedan lejos porque `cloudflared` llega por HTTP/1.1 sin TLS, aunque la subida los resuelve igual.

Las de npm vienen de `vite-plugin-pwa` (`workbox-build`, `ejs`, `glob`, `deepmerge`) y `openapi-typescript` (`minimatch`). No llegan al navegador; se resuelven actualizando esas dos herramientas cuando publiquen versiones nuevas. `go-jose/v4` tiene una versión más reciente (4.1.7), sin avisos conocidos para la actual.

## Lo que no se pudo verificar

- **Cloudflare real.** No hay despliegue: ni el túnel, ni `CF-Connecting-IP` a través de él, ni si el borde frenaría alguno de estos ataques. La regla de loopback se comprobó por lectura y por `limits_test.go`. La prueba con una dirección de red real se hizo al endurecer el servidor, y no se repitió para no exponer el binario en la red local.
- **Google real.** Solo el proveedor falso de las pruebas.
- **Resend real.** Ni cuotas ni rebotes.
- **La VM `e2-micro`.** La caída por memoria y el llenado del disco se dedujeron de lo medido en local. Las mediciones posteriores a los arreglos también son locales.
- **Service worker en ejecución.** No llegó a registrarse en el navegador integrado, que falló al obtener el script, aunque el servidor lo entrega con 200. La conclusión sale de leer el `sw.js` generado.
- **Safari en iOS y la PWA instalada.**
- **La caché HTTP del navegador** tras cerrar sesión.
