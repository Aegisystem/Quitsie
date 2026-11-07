# Cada código se ata a su navegador, y los navegadores conocidos tienen límites propios

> Cambio del 2026-11-07, decidido por el dueño: los cupos de envío por hora (5 por email, 3 por email desde una IP, 10 por IP y 5 por dispositivo conocido) pasan a una espera de un minuto entre códigos, por email o por dispositivo conocido. Un envío que el servicio de correo rechaza no cuenta. Ya no hay cupo de envío por IP. Los cupos anteriores dejaban a alguien fuera hasta una hora después de unos pocos intentos fallidos. Los límites de códigos equivocados no cambian.

Pedir un código deja en el navegador una cookie de flujo `HttpOnly` con un valor aleatorio de 128 bits, y la fila del código guarda solo su hash. Solo ese navegador puede comprobar el código, los intentos se cuentan por código, y pedir otro desde el mismo navegador reemplaza solo el suyo. Además, cada inicio de sesión, con código o con Google, deja una cookie de dispositivo conocido que dura un año y dice que ese navegador ya entró en ese Usuario; en la base solo está su hash. Un dispositivo conocido no gasta los límites por email, que cualquiera puede agotar sabiendo la dirección, y tiene los suyos: 5 envíos por hora y 10 códigos equivocados al día. Los demás navegadores siguen compartiendo los del email, así que el tope del hallazgo 4 de la [revisión de seguridad](../revision-seguridad.md) se mantiene. Es la defensa que OWASP llama *device cookies*.

## Considered Options

- **Solo la cookie de flujo, con el tope por email como techo**, la propuesta de la revisión: protege el código de cada navegador, pero diez fallos del atacante en sus propios flujos siguen bloqueando el email un día.
- **Contar los fallos por flujo o por IP, sin tope por email**: devuelve a quien tiene muchas IP los intentos que quitó el hallazgo 4.
- **Una cookie de dispositivo firmada con una clave del servidor, sin tabla**: no se puede olvidar un dispositivo ni borrarlo con la cuenta, y añade otra clave que guardar.

## Consequences

- Cerrar sesión no borra la cookie de dispositivo conocido: es lo que deja volver a entrar a quien sufre el ataque. Borrar la cuenta olvida sus dispositivos.
- La cookie guarda un token por cada Usuario que entró en ese navegador, hasta 5, y cada Usuario conserva sus 20 dispositivos usados más recientemente. El navegador nunca elige su token, así que una cookie plantada no da acceso al dispositivo de otro.
- Un código ya no sirve en otro navegador que el que lo pidió.
- Queda un riesgo: quien estrena dispositivo, o borró las cookies, depende de los límites por email, y cualquiera puede bloquearle los códigos un día. Google no se ve afectado.
- Quien robe la cookie de dispositivo de alguien gana 10 intentos al día sobre esa cuenta, además de los 10 del email.
