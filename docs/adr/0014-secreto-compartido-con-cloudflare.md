# El servidor cree a Cloudflare por un secreto compartido

Una regla de Cloudflare escribe un secreto en la cabecera `X-Quitsie-Cloudflare-Secret` de cada petición que reenvía, y el servidor tiene el mismo valor en `QUITSIE_CLOUDFLARE_SECRET`. El servidor solo cree la dirección del visitante que trae `CF-Connecting-IP` cuando la petición trae ese secreto. Las demás peticiones no pasaron por Cloudflare y cuentan todas juntas como un solo visitante en los límites por IP. Sin la variable, el servidor no cree ninguna cabecera y cada conexión cuenta por su otro extremo.

El camino en producción es visitante → Cloudflare → proxy de Railway → servidor. El servidor ve como otro extremo de la conexión un proxy de Railway, así que hasta esta decisión todos los visitantes compartían los límites por IP: 300 peticiones por minuto y 60 verificaciones de código por hora. Por eso tampoco había cupo de envío de códigos por IP, y una sola dirección podía agotar los 40 correos por hora del servidor.

## Considered Options

- **Creer `X-Real-IP`, que escribe Railway**: Railway sobrescribe la que mande el cliente. Pero cuando la conexión llega desde un rango de Cloudflare, copia en ella `CF-Connecting-IP` ([foro de Railway](https://station.railway.com/questions/need-authoritative-client-ip-proxy-tru-090eac86)). Un Worker de cualquier cuenta de Cloudflare puede pedir el dominio `*.up.railway.app` con el `CF-Connecting-IP` que quiera, y llega desde esos rangos ([cabeceras de Cloudflare](https://developers.cloudflare.com/fundamentals/reference/http-headers/)). Railway avisa además de que estas cabeceras pueden cambiar.
- **Creer la cabecera según la dirección de la conexión**: es siempre un proxy de Railway, que no publica sus rangos y atiende también a quien no pasa por Cloudflare.
- **Authenticated Origin Pulls**: Railway termina el TLS en su borde y no valida certificados de cliente.
- **Quitar el dominio `*.up.railway.app`**: según el foro de Railway, su borde sirve también el dominio propio a quien conecte directo, así que la entrada sigue abierta.
- **Cloudflare Tunnel como otro servicio en Railway**: cierra la entrada del todo. Añade un servicio que mantener, que es lo que quitó el [ADR 0012](0012-postgres-en-railway.md).
- **Rechazar con 403 lo que llega sin el secreto**: es más estricto, pero un error en la regla o al rotar el secreto deja la aplicación caída para todos. Contar esas peticiones como un solo visitante ya le quita la ventaja a quien se salta Cloudflare.

## Consequences

- El dueño mantiene el secreto en dos sitios: la regla de Cloudflare y la variable de Railway ([despliegue](../despliegue.md#secreto-de-cloudflare)). Si no coinciden, la aplicación sigue funcionando, pero todos los visitantes comparten un cupo.
- Lo que llega sin pasar por Cloudflare comparte 300 peticiones por minuto, 60 verificaciones de código por hora y 10 códigos por hora, sea quien sea.
- Vuelve el cupo de envío de códigos por IP, 10 por hora, que el [ADR 0011](0011-codigos-atados-al-navegador.md) había perdido el 2026-11-07.
- Un Worker de otra cuenta que pida el dominio que pasa por Cloudflare llega con un `CF-Connecting-IP` fijo, `2a06:98c0:3600::103`. Todos esos Workers comparten un cupo.
- Las pruebas de Go y las de punta a punta envían un secreto de prueba, así que recorren el mismo camino que producción.
- El modo SSL/TLS de Cloudflare tiene que ser **Full**, para que el secreto viaje cifrado hasta Railway.
