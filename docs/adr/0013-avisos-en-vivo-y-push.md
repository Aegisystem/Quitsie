# Avisos en vivo por SSE y notificaciones push

El dueño decidió estos avisos el 2026-11-06. Cada escritura en un libro dispara una notificación de PostgreSQL desde un trigger (`NOTIFY quitsie_changes`, migración 00002). El paquete `api/internal/changefeed` la recibe y la reparte a dos consumidores:

- **La PWA abierta.** Mantiene una conexión SSE con el servidor. Cuando cambia un libro que el Usuario ve, el servidor le avisa y la PWA hace un pull. El aviso no lleva datos; los datos siguen llegando por `GET /api/v1/sync`.
- **Las notificaciones push.** Usan Web Push con claves VAPID. Avisan de un Gasto nuevo en el que el Usuario participa y de una Liquidación que lo incluye. Nunca avisan de lo que hizo el propio Usuario. Muestran quién lo hizo, en qué Grupo o Amistad, qué Gasto y cuánto le toca al Usuario. Varios registros de un libro en el mismo minuto llegan en una sola notificación.

## Considered Options

- **Preguntar cada 30 segundos.** Tarda hasta 30 segundos y genera peticiones aunque nada cambie.
- **WebSocket.** La PWA no necesita enviar nada por esa conexión, así que SSE basta.
- **Avisar desde el código de cada escritura.** Una escritura nueva podría olvidarlo. El trigger cubre todas las tablas de un libro.

## Consequences

- En iOS, las notificaciones push solo funcionan con la PWA instalada en la pantalla de inicio, desde iOS 16.4. El permiso se pide después de un toque del Usuario.
- Las notificaciones viajan cifradas por el servicio push del navegador (Apple, Google o Mozilla).
- El servidor corta las respuestas a los 60 segundos (`WriteTimeout`). La conexión SSE tiene que alargar ese plazo.
- Con varias instancias del servidor funciona igual, porque cada instancia escucha a PostgreSQL.
- Una notificación de PostgreSQL perdida (por ejemplo, durante una reconexión) solo retrasa el aviso: el `changefeed` avisa de todos los libros al reconectar, y la PWA sigue sincronizando al abrirse y al volver a la pantalla.
