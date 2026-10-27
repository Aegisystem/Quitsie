# Los Enlaces de invitación se derivan, no se guardan

El token de un Enlace de invitación se calcula como un HMAC de una clave secreta del servidor, el identificador del libro y un contador de generación que se guarda en el libro. Así el enlace se puede volver a mostrar siempre, sin guardar el token en la base. Regenerarlo es incrementar el contador, lo que invalida el anterior.

## Considered Options

- **Guardar solo el hash de un token aleatorio**, la idea inicial: el enlace no se puede volver a mostrar, y compartirlo obliga a regenerarlo, lo que anula el que ya se había enviado.
- **Guardar el token tal cual**: se puede mostrar, pero una copia filtrada de la base daría acceso a todos los Grupos.

## Consequences

- La clave secreta vive en la configuración del servidor; rotarla invalida todos los enlaces a la vez.
- El enlace lleva el identificador del libro junto al token, para saber qué contador usar al comprobarlo.
