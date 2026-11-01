# Datos personales

Inventario de los datos personales que maneja Quitsie: qué se guarda, dónde, cuánto dura y quién más lo ve. Sirve de base para la política de privacidad, que todavía no existe ([pendientes](pendientes.md#5-falta-fuera-del-código)). Se comprobó contra el código el 2026-11-01. El vocabulario sigue [CONTEXT.md](../CONTEXT.md).

No es la política de privacidad. Lo que solo puede decidir o firmar el dueño está al final.

## De quién son los datos

- **Usuarios:** quienes tienen cuenta.
- **Personas sin cuenta:** los Miembros pendientes y los Amigos pendientes. Otro Usuario escribe su nombre y, si quiere, su email. Esa persona nunca aceptó nada.
- **El dueño:** su email recibe las alertas de la VM.

## En el servidor (SQLite en la VM)

| Dato | Tabla | Cuánto dura |
| --- | --- | --- |
| Email, nombre, Moneda preferida, identificador de Google, fecha de alta | `users` | Hasta borrar la cuenta. Entonces se vacían el email, el nombre y el identificador de Google, y la fila queda sin datos personales para los Gastos pasados |
| Sesiones: hash del token, navegador (el `User-Agent`), alta y último uso | `sessions` | Hasta cerrar la sesión, borrar la cuenta o pasar 90 días sin uso |
| Códigos de acceso: email, hash del código, hash de la cookie de flujo, intentos | `login_codes` | 10 minutos. La fila caducada se borra con la siguiente petición de código de cualquiera |
| Códigos equivocados: hash del email o del dispositivo, cuántos y hasta cuándo | `login_failures` | 24 horas, y se borra con la siguiente petición de código |
| Dispositivos conocidos: hash del token, Usuario, alta y último uso | `known_devices` | Un año sin entrar desde ese navegador. La fila caducada se borra con el siguiente inicio de sesión de cualquiera. Como mucho 20 por Usuario. Se borran con la cuenta |
| Nombre y email de cada Miembro o Amigo; nombre de cada Grupo | `members`, `ledgers` | Lo que dure el libro. Al borrar una cuenta, sus Miembros vuelven a ser pendientes, conservan el nombre y pierden el email |
| Gastos (descripción, fecha, importes, Categoría, autor), Liquidaciones y Actividades | `expenses`, `paid_shares`, `owed_shares`, `settlements`, `activities` | Lo que dure el libro. Un Gasto borrado se puede restaurar sin límite de tiempo, así que no desaparece |
| Correos enviados por hora, sin direcciones | `mail_sends` | 24 horas |

Los Gastos y Liquidaciones son de todos los Miembros del libro. Borrar una cuenta no los borra, porque cambiaría los Saldos de los demás ([reglas](reglas-de-negocio.md)).

## Respaldos

Litestream copia la base a Cloudflare R2 de forma continua y guarda **7 días** de puntos de restauración (`deploy/litestream.yml`). Un dato borrado o una cuenta borrada siguen en los respaldos ese tiempo. Cambiar ese plazo es la decisión 6 de [pendientes](pendientes.md#3-decisiones-del-dueño).

## En el log de la VM

El binario no registra cada petición. Escribe en el journal de systemd:

- Los errores 500, con el método y la ruta. Una ruta puede llevar el id de un libro o el token de un Enlace de invitación.
- Los fallos al entrar con Google y el agotamiento del presupuesto de correos, sin el email.

En producción no escribe correos ni códigos: el servidor no arranca con `QUITSIE_MAIL=log`. El journal no tiene un plazo configurado; systemd lo limita por tamaño. Las alertas copian al email del dueño solo el mensaje de cada línea de error ([despliegue](despliegue.md#alertas)).

## En el navegador

| Dato | Dónde | Cuánto dura |
| --- | --- | --- |
| Réplica de todos los libros del Usuario, con nombres, emails de pendientes, Gastos y Actividades | IndexedDB | Hasta cerrar sesión o entrar como otro Usuario. Tras cerrar la sesión desde otro dispositivo sigue allí (hallazgo 9 de la [revisión](revision-seguridad.md), abierto) |
| Registros sin sincronizar y rechazados | IndexedDB | Hasta que se envían, se corrigen o se descartan |
| El Usuario actual (id, email, nombre, Moneda preferida), para abrir sin conexión | `localStorage` | Hasta cerrar sesión |
| Cookie de sesión | Cookie `HttpOnly` | 90 días sin uso, o hasta cerrar sesión |
| Cookie de flujo del código | Cookie `HttpOnly` | 10 minutos |
| Cookie de dispositivo conocido | Cookie `HttpOnly` | Un año. **Cerrar sesión no la borra**, a propósito ([ADR 0011](adr/0011-codigos-atados-al-navegador.md)) |
| Cookie del flujo de Google | Cookie `HttpOnly` | 10 minutos, o hasta volver de Google |

Todas las cookies son propias y necesarias para entrar. No hay analítica, publicidad ni recursos de terceros: la `Content-Security-Policy` solo admite archivos propios.

## Quién más ve los datos

| Servicio | Para qué | Qué ve | Dónde |
| --- | --- | --- | --- |
| Google Cloud | La VM y su disco | Todo lo que guarda el servidor | `us-east1`, Estados Unidos |
| Cloudflare | DNS, CDN y túnel | Cada petición: IP, navegador, rutas y el tráfico descifrado, porque Cloudflare termina el TLS | Red global |
| Cloudflare R2 | Respaldos | La base entera | Según la cuenta de R2 |
| Resend | Enviar los códigos y las alertas | El email de destino y el contenido, código incluido | Estados Unidos (comprobar en su panel) |
| Google | Entrar con Google, si se activa | Que esa persona usa Quitsie | Global |
| GitHub | Código y despliegue | Ningún dato de Usuarios | Global |

## Derechos que la app ya cubre

- **Borrar la cuenta**, desde Cuenta. Se borran el email, el nombre, las sesiones y los dispositivos conocidos. Los Gastos se quedan, con el nombre guardado en cada Miembro.
- **Corregir** el nombre y la Moneda preferida, desde Cuenta.
- **Ver las sesiones abiertas y cerrarlas**, desde Cuenta.

No hay forma de **exportar** los datos propios, ni de que una persona sin cuenta pida que se borre su nombre o su email de un libro ajeno, salvo que un Miembro lo edite.

## Lo que tiene que decidir o escribir el dueño

- Quién es el responsable de los datos y cómo se le contacta.
- Qué leyes aplican según dónde viva el dueño y dónde estén los Usuarios. En Colombia, la Ley 1581 de 2012; en la Unión Europea, el RGPD si se ofrece allí.
- La base legal de cada tratamiento y el aviso de que los datos salen del país (Estados Unidos y la red de Cloudflare).
- Cómo se informa a las personas sin cuenta cuyos datos escribe otro Usuario. Relacionado con la decisión 2 de [pendientes](pendientes.md#3-decisiones-del-dueño).
- El plazo de los respaldos (decisión 6) y si hace falta exportar datos.
- Los términos de uso: que la app no mueve dinero, y qué pasa con una cuenta que abusa de los límites.
