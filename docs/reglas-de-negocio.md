# Reglas de negocio

Reglas acordadas durante el diseño. El vocabulario sigue [CONTEXT.md](../CONTEXT.md).

## Gastos

- Un Gasto tiene descripción, fecha (por defecto hoy, editable), un total mayor que cero y una Categoría opcional.
- Las Categorías son: comida, supermercado, transporte, alojamiento, vivienda, servicios, ocio, compras, salud, regalos y otros.
- Al escribir la descripción de un Gasto, la aplicación sugiere una Categoría: primero la que el Usuario dio a sus Gastos con una descripción igual o casi igual, y si no hay ninguno, la de una palabra clave («uber», «almuerzo», «D1»). Si nada coincide, no sugiere nada. Una Categoría elegida a mano nunca se reemplaza, y al editar un Gasto que ya tiene Categoría no se sugiere nada. La sugerencia se calcula en el dispositivo, sin enviar nada a terceros.
- Tiene al menos un Aporte y al menos una Cuota; los Aportes suman el total y las Cuotas también.
- Los Aportes son mayores que cero. Una Cuota puede quedar en cero por redondeo (1 céntimo al 50 %), y se guarda igualmente para no perder el reparto al reeditar.
- Ningún importe supera 10^15 unidades mínimas de su moneda. Eso limita cada registro, no los Saldos, que suman muchos: ellos tienen su propio tope (ver [Límites](#límites)).
- Con Porcentajes, cada porcentaje admite hasta dos decimales y entre todos suman exactamente 100.
- Con Proporciones, los pesos son enteros positivos; una proporción como 1,5:1 se escribe 3:2.
- Con Ajustes, un ajuste puede ser negativo, pero ninguna Cuota puede quedar por debajo de cero.
- Si el reparto no es exacto (100 entre 3), el sobrante se asigna de forma determinista, se ve en la previsualización y las Cuotas se guardan ya calculadas: nunca se recalculan.
- Los importes respetan los decimales de su moneda.
- Puede editar o borrar un Gasto de Grupo cualquier Miembro vinculado; uno sin grupo, cualquiera de sus dos Participantes.
- Un Gasto borrado se puede restaurar sin límite de tiempo.
- Si dos personas editan el mismo Gasto a la vez, la segunda en guardar recibe un aviso y ve la versión nueva; nunca se sobrescribe en silencio.
- Un Gasto en el que participa un Antiguo Miembro se puede editar, pero sin cambiar los Aportes ni las Cuotas de ese Antiguo Miembro. Borrarlo se rechaza, y también restaurarlo si ya estaba borrado, porque cambiaría su Saldo, que es cero.

## Deudas

- La Simplificación de deudas está siempre activada: la aplicación no ofrece desactivarla (decidido el 2026-11-10). Se parte de los Saldos y quien más debe le paga a quien más se le debe, hasta saldar todo. Deja como mucho un pago menos que el número de personas con Saldo distinto de cero; es una aproximación al mínimo.
- Una Deuda simplificada nunca supera los Saldos de quienes la deben y la cobran.
- La API todavía acepta desactivarla en un Grupo (`simplifyDebts` en `PATCH /groups/{groupId}`). Sin ella, las Deudas se calculan Gasto a Gasto: dentro de cada Gasto, quienes deben les deben a quienes pusieron de más, en proporción a lo que cada uno puso de más, con el sobrante repartido de forma determinista. Luego se compensan las Deudas en sentidos opuestos y se restan las Liquidaciones. Una Deuda entre dos personas sí puede superar los Saldos: si Ana paga por Beto, Beto por Carla y Carla por Ana, cada pareja se debe más con cada vuelta y los Saldos siguen en cero. Si alguna superara el tope de los Saldos (ver [Límites](#límites)), el libro muestra las Deudas simplificadas, que saldan lo mismo.

## Grupos

- No hay roles: todos los Miembros vinculados pueden lo mismo (añadir Miembros, editar Gastos, regenerar el Enlace de invitación).
- Nadie puede salir ni ser sacado de un Grupo con Saldo distinto de cero; al salir pasa a ser Antiguo Miembro y pierde el acceso al Grupo, aunque su nombre sigue en los Gastos pasados.
- Cualquier Miembro vinculado puede sacar a otro Miembro, pendiente o vinculado, si su Saldo es cero.
- Quien fue sacado de un Grupo no puede volver a entrar con el Enlace de invitación hasta que un Miembro vinculado lo readmita. Quien salió por su cuenta sí puede volver con el enlace.
- Readmitir no devuelve a nadie al Grupo: le permite volver con el Enlace de invitación, y al volver recupera su Antiguo Miembro. Queda en la Actividad.
- Quien vuelve a un Grupo del que salió recupera su Antiguo Miembro, con todo su historial; no se crea uno nuevo.
- El nombre y el email de un Miembro pendiente se pueden editar.
- El email de un Miembro pendiente solo lo ve quien lo añadió; los demás ven su nombre.
- Quien no ve ese email puede editar el nombre y el email sigue igual. Si escribe otro email, reemplaza al anterior, y desde entonces solo lo ve quien lo escribió.
- Un Usuario ocupa como máximo un Miembro por Grupo.
- Un Grupo solo se puede borrar cuando todos los Saldos están en cero.
- Un Grupo admite 200 Miembros, contando a los Antiguos Miembros. Quien vuelve a un Grupo lleno recupera su Antiguo Miembro igualmente.
- El último Miembro vinculado puede salir aunque queden Miembros pendientes: el Grupo sigue existiendo y su Enlace de invitación sigue sirviendo para Reclamarlos.

## Moneda del grupo

- Se puede cambiar en cualquier momento. El cambio reetiqueta todos los Gastos y Liquidaciones del Grupo, también los borrados (mismos números, otra moneda), y nunca aplica tipos de cambio.
- Si la moneda nueva tiene menos decimales (USD → CLP), el cambio solo se permite cuando ningún importe perdería céntimos, incluidos los de Gastos y Liquidaciones borrados, porque se pueden restaurar; si no, la aplicación indica qué corregir.
- Si tiene más decimales (CLP → USD), los mismos números son importes y Saldos mayores en unidades mínimas. El cambio se rechaza si algún importe pasaría de 10^15 o algún Saldo de su tope (ver [Límites](#límites)).
- Los Registros sin sincronizar creados antes del cambio llegan como Registros rechazados, para que su autor confirme la moneda.
- El cambio queda en la Actividad.
- Las monedas de fondos de ISO 4217 (por ejemplo CLF, COU o UYI) no se ofrecen en la interfaz.
- Los selectores de moneda muestran primero la Moneda preferida y las más usadas; debajo, las demás en orden alfabético.

## Personas

- Para Reclamar se usa el Enlace de invitación. Si el email de quien entra coincide con el de un Miembro pendiente, se le sugiere, pero nunca se vincula automáticamente.
- El Enlace de invitación de un Grupo se puede volver a mostrar cuantas veces haga falta; regenerarlo invalida el anterior (ver [ADR 0010](adr/0010-enlaces-de-invitacion-derivados.md)).
- Cualquier Miembro vinculado puede deshacer la vinculación de otro, sin importar cómo entró al Grupo; su Miembro vuelve a quedar pendiente, queda en la Actividad y ese Usuario deja de ver el Grupo. Puede volver a entrar con el Enlace de invitación.
- Al borrar su cuenta, los Miembros y Amigos de un Usuario vuelven a quedar pendientes con su nombre, así que los Saldos de los demás no cambian. De la cuenta se borran el email y el nombre; solo queda el nombre guardado en cada Miembro.
- Quien entra por primera vez con un código llega sin nombre; la aplicación le pide su nombre y su Moneda preferida, proponiendo la del idioma del navegador. Con Google, el nombre viene de la cuenta. Sin nombre no se pueden crear Grupos ni Amistades.
- Un código solo sirve en el navegador que lo pidió, y pedir otro desde ese navegador invalida el anterior.
- Después de enviar un código, el email espera un minuto para recibir otro. Un navegador en el que el Usuario ya entró espera por su cuenta. Si el correo no se pudo enviar, no hay espera.
- Un email admite 10 códigos equivocados en 24 horas desde navegadores en los que su Usuario nunca entró, sumando todos sus códigos. Al llegar a 10 no se aceptan ni se envían códigos a esos navegadores hasta que pasen 24 horas desde el primer fallo. Un navegador en el que el Usuario ya entró no cuenta en esos 10 y tiene otros 10 propios, así que nadie puede impedirle entrar ([ADR 0011](adr/0011-codigos-atados-al-navegador.md)). Google y las sesiones abiertas siguen funcionando.
- El email de un Miembro pendiente o de un Amigo pendiente cumple la misma regla que el del inicio de sesión: una dirección simple de 254 caracteres como máximo.
- Un Usuario puede estar en 200 Grupos y Amistades. Otra persona puede crear una Amistad con ese Usuario por encima de ese número.

## Gastos sin grupo y Amistades

- Siempre entre exactamente dos personas (ver [ADR 0004](adr/0004-gastos-sin-grupo-bilaterales.md)).
- Cada uno lleva su moneda; por defecto, la Moneda preferida del Usuario que lo crea.
- Los Saldos entre Amigos se muestran por moneda, sin conversión.
- Una Amistad con alguien que ya tiene cuenta solo se crea directamente si los dos comparten un Grupo; si no, se crea con un Amigo pendiente y su Enlace de invitación.
- Si ya existe una Amistad entre dos Usuarios, no se puede Reclamar otro Amigo pendiente que represente a la misma persona.

## Liquidaciones

- Solo se registran; la aplicación no mueve dinero real.
- Se permiten Liquidaciones parciales.
- Solo entre Miembros activos.
- Se pueden borrar y restaurar, pero no editar: para corregir una, se borra y se crea otra. Una Liquidación con un Antiguo Miembro no se puede borrar ni restaurar.

## Límites

- Un libro guarda 200.000 filas como máximo, contando Gastos, Aportes, Cuotas, Liquidaciones y Actividades.
- Un libro lleno solo admite borrar Gastos y Liquidaciones, salir, sacar a un Miembro, deshacer una Reclamación y borrar el Grupo. Un Registro sin sincronizar que no cabe queda como Registro rechazado.
- Ningún Saldo pasa de 2^53 − 1 unidades mínimas de su moneda, a favor ni en contra: 9.007.199.254.740.991, unos 90 billones en dólares, o algo más de nueve Gastos del importe máximo. La aplicación maneja los importes como números de JavaScript, que guardan exacto cualquier entero hasta ahí: por encima, el dispositivo leería los Saldos y las Deudas redondeados.
- Se rechaza cualquier cambio que llevaría un Saldo más allá de ese tope: crear, editar, borrar o restaurar un Gasto o una Liquidación, y cambiar la Moneda del grupo. Solo se miran los Saldos que el cambio mueve. Los formularios lo avisan antes de guardar, y un Registro sin sincronizar que no cabe queda como Registro rechazado.
- Un Usuario puede enviar 5 MiB de cambios por hora. Leer, cerrar sesión y cerrar otras sesiones no cuentan.

## Presentación

- Los importes se muestran con el formato numérico de la región del Usuario (en México, 7,589.67; en Colombia, 7.589,67).
- El total de Inicio suma, por moneda, los Saldos del Usuario en todos sus Grupos y Amistades. Puede pasar del tope de un Saldo, y se muestra exacto igualmente.
- En los listados se ocultan los decimales cuando son cero; en los formularios se muestran siempre.
- Los campos de importe muestran los separadores de miles mientras se escribe (al escribir 50000 en Colombia se ve 50.000).
- Al escribir un importe, si después del punto no vienen exactamente tres cifras, el punto se lee como separador decimal aunque en esa región separe los miles ("10.50" son 10,50 en Colombia).

## Avisos

- Con la aplicación abierta, lo que otro Usuario cambia en un Grupo o una Amistad aparece al momento.
- Las notificaciones push avisan de un Gasto nuevo en el que el Usuario participa y de una Liquidación que lo incluye. No avisan de lo que hace el propio Usuario.
- La notificación dice quién lo hizo, en qué Grupo o Amistad, qué Gasto es y cuánto le toca al Usuario.
- Una notificación espera 5 segundos por más registros del mismo Grupo o Amistad. Entre dos notificaciones del mismo Grupo o Amistad a un Usuario pasa al menos un minuto, y los registros de ese minuto llegan juntos.
- Un registro que se borra antes de que salga la notificación no aparece en ella. Un registro de hace más de 15 minutos no se notifica.
- Las notificaciones se activan en cada dispositivo, desde Cuenta. Cerrar sesión las desactiva en ese dispositivo ([ADR 0013](adr/0013-avisos-en-vivo-y-push.md)).
- En iPhone y iPad, las notificaciones solo llegan a Quitsie instalada en la pantalla de inicio, desde iOS 16.4.

## Sin conexión

- Se pueden crear Gastos y Liquidaciones; no Grupos, Miembros ni Amigos (ver [ADR 0001](adr/0001-offline-solo-creacion.md)).
- Un Registro rechazado nunca se descarta en silencio.
- Al cerrar sesión se borran del dispositivo la réplica y la cola. Si hay registros sin sincronizar o rechazados, la aplicación avisa antes y permite cancelar.
- Si la sesión termina sola y entra otro Usuario en el dispositivo, se borra la réplica, pero los registros sin sincronizar del anterior se guardan aparte, ocultos para el nuevo, y se envían cuando su dueño vuelva a entrar.
