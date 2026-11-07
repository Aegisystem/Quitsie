# Datos personales

Inventario de los datos personales que maneja Quitsie: qué se guarda, dónde, cuánto dura y quién más lo ve. Sirve de base para la política de privacidad, que todavía no existe ([pendientes](pendientes.md#5-falta-fuera-del-código)). Se comprobó contra el código el 2026-11-01, y las notificaciones push el 2026-11-06. El vocabulario sigue [CONTEXT.md](../CONTEXT.md).

No es la política de privacidad. Lo que solo puede decidir o firmar el dueño está al final.

## De quién son los datos

- **Usuarios:** quienes tienen cuenta.
- **Personas sin cuenta:** los Miembros pendientes y los Amigos pendientes. Otro Usuario escribe su nombre y, si quiere, su email. Esa persona nunca aceptó nada.
- **El dueño:** su email recibe los avisos de Railway.

## En el servidor (PostgreSQL en Railway)

| Dato | Tabla | Cuánto dura |
| --- | --- | --- |
| Email, nombre, Moneda preferida, identificador de Google, fecha de alta | `users` | Hasta borrar la cuenta. Entonces se vacían el email, el nombre y el identificador de Google, y la fila queda sin datos personales para los Gastos pasados |
| Sesiones: hash del token, navegador (el `User-Agent`), alta y último uso | `sessions` | Hasta cerrar la sesión, borrar la cuenta o pasar 90 días sin uso |
| Códigos de acceso: email, hash del código, hash de la cookie de flujo, intentos | `login_codes` | 10 minutos. La fila caducada se borra con la siguiente petición de código de cualquiera |
| Códigos equivocados: hash del email o del dispositivo, cuántos y hasta cuándo | `login_failures` | 24 horas, y se borra con la siguiente petición de código |
| Dispositivos conocidos: hash del token, Usuario, alta y último uso | `known_devices` | Un año sin entrar desde ese navegador. La fila caducada se borra con el siguiente inicio de sesión de cualquiera. Como mucho 20 por Usuario. Se borran con la cuenta |
| Nombre y email de cada Miembro o Amigo, y quién escribió ese email; nombre de cada Grupo | `members`, `ledgers` | Lo que dure el libro. El email de un pendiente solo lo ve quien lo escribió. Al borrar una cuenta, sus Miembros vuelven a ser pendientes, conservan el nombre y pierden el email |
| Gastos (descripción, fecha, importes, Categoría, autor), Liquidaciones y Actividades | `expenses`, `paid_shares`, `owed_shares`, `settlements`, `activities` | Lo que dure el libro. Un Gasto borrado se puede restaurar sin límite de tiempo, así que no desaparece |
| Correos enviados por hora, sin direcciones | `mail_sends` | 24 horas |
| Suscripción push de cada dispositivo que activó las notificaciones: el endpoint del servicio push del navegador y sus claves | `push_subscriptions` | Hasta desactivarlas, cerrar esa sesión, que caduque o borrar la cuenta. También se borra cuando el servicio push dice que ya no existe |

Los Gastos y Liquidaciones son de todos los Miembros del libro. Borrar una cuenta no los borra, porque cambiaría los Saldos de los demás ([reglas](reglas-de-negocio.md)).

## Respaldos

Los respaldos son los de la base de Railway. Un dato borrado o una cuenta borrada siguen en ellos hasta que el respaldo caduca. El plazo depende de cómo se configuren en Railway, y es la decisión 6 de [pendientes](pendientes.md#3-decisiones-del-dueño).

## En el log del servidor

El binario no registra cada petición. Escribe en los logs de Railway:

- Los errores 500, con el método y la ruta. Una ruta puede llevar el id de un libro o el token de un Enlace de invitación.
- Los fallos al entrar con Google y el agotamiento del presupuesto de correos, sin el email.

En producción no escribe correos ni códigos: el servidor no arranca con `QUITSIE_MAIL=log`. Railway guarda los logs según el plan; el plazo está en su panel.

## En el navegador

| Dato | Dónde | Cuánto dura |
| --- | --- | --- |
| Réplica de todos los libros del Usuario, con nombres, los emails de pendientes que escribió, Gastos y Actividades | IndexedDB | Hasta cerrar sesión, entrar como otro Usuario o que la sesión termine sola (caduca, se cierra desde otro dispositivo o se borra la cuenta). En ese último caso se borra cuando el dispositivo vuelve a tener conexión; mientras no la tenga, sigue allí |
| Registros sin sincronizar y rechazados | IndexedDB | Hasta que se envían, se corrigen o se descartan. Si la sesión termina sola y entra otro Usuario, quedan apartados y ocultos hasta que su dueño vuelva a entrar en ese dispositivo. Los de una cuenta borrada se quedan allí sin enviarse |
| El Usuario actual (id, email, nombre, Moneda preferida), para abrir sin conexión | `localStorage` | Hasta cerrar sesión |
| Cookie de sesión | Cookie `HttpOnly` | 90 días sin uso, o hasta cerrar sesión |
| Cookie de flujo del código | Cookie `HttpOnly` | 10 minutos |
| Cookie de dispositivo conocido | Cookie `HttpOnly` | Un año. **Cerrar sesión no la borra**, a propósito ([ADR 0011](adr/0011-codigos-atados-al-navegador.md)) |
| Cookie del flujo de Google | Cookie `HttpOnly` | 10 minutos, o hasta volver de Google |
| Suscripción push, si el Usuario activó las notificaciones | El navegador | Hasta desactivarlas o cerrar sesión |

Todas las cookies son propias y necesarias para entrar. No hay analítica, publicidad ni recursos de terceros: la `Content-Security-Policy` solo admite archivos propios.

## Quién más ve los datos

| Servicio | Para qué | Qué ve | Dónde |
| --- | --- | --- | --- |
| Railway | El servidor, la base y sus respaldos | Todo lo que guarda el servidor, y cada petición descifrada, porque Railway termina el TLS | La región del proyecto (comprobar en su panel) |
| Resend | Enviar los códigos | El email de destino y el contenido, código incluido | Estados Unidos (comprobar en su panel) |
| Google | Entrar con Google, si se activa | Que esa persona usa Quitsie | Global |
| Servicios push de Apple, Google, Mozilla y Microsoft | Entregar las notificaciones del Usuario que las activa | El contenido va cifrado para el dispositivo y el servicio no lo lee. Ve a qué dispositivo va cada notificación, cuándo y cuánto pesa | El del navegador del Usuario |
| GitHub | Código y despliegue | Ningún dato de Usuarios | Global |

## Derechos que la app ya cubre

- **Borrar la cuenta**, desde Cuenta. Se borran el email, el nombre, las sesiones y los dispositivos conocidos. Los Gastos se quedan, con el nombre guardado en cada Miembro.
- **Corregir** el nombre y la Moneda preferida, desde Cuenta.
- **Ver las sesiones abiertas y cerrarlas**, desde Cuenta.
- **Activar y desactivar las notificaciones** de cada dispositivo, desde Cuenta.

No hay forma de **exportar** los datos propios, ni de que una persona sin cuenta pida que se borre su nombre o su email de un libro ajeno, salvo que un Miembro lo edite.

## Lo que tiene que decidir o escribir el dueño

- Quién es el responsable de los datos y cómo se le contacta.
- Qué leyes aplican según dónde viva el dueño y dónde estén los Usuarios. En Colombia, la Ley 1581 de 2012; en la Unión Europea, el RGPD si se ofrece allí.
- La base legal de cada tratamiento y el aviso de que los datos salen del país (la región de Railway y Estados Unidos).
- Cómo se informa a las personas sin cuenta cuyos datos escribe otro Usuario. Su email solo lo ve quien lo escribió.
- El plazo de los respaldos (decisión 6) y si hace falta exportar datos.
- Los términos de uso: que la app no mueve dinero, y qué pasa con una cuenta que abusa de los límites.
