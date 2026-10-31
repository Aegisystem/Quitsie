# Reglas de negocio

Reglas acordadas durante el diseño. El vocabulario sigue [CONTEXT.md](../CONTEXT.md).

## Gastos

- Un Gasto tiene descripción, fecha (por defecto hoy, editable), un total mayor que cero y una Categoría opcional.
- Las Categorías son: comida, supermercado, transporte, alojamiento, vivienda, servicios, ocio, compras, salud, regalos y otros.
- Tiene al menos un Aporte y al menos una Cuota; los Aportes suman el total y las Cuotas también.
- Los Aportes son mayores que cero. Una Cuota puede quedar en cero por redondeo (1 céntimo al 50 %), y se guarda igualmente para no perder el reparto al reeditar.
- Ningún importe supera 10^15 unidades mínimas de su moneda, para que las sumas de Saldos nunca desborden.
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

- Sin Simplificación de deudas, las Deudas se calculan Gasto a Gasto: dentro de cada Gasto, quienes deben les deben a quienes pusieron de más, en proporción a lo que cada uno puso de más, con el sobrante repartido de forma determinista. Luego se compensan las Deudas en sentidos opuestos y se restan las Liquidaciones.
- Con Simplificación de deudas, se parte de los Saldos y quien más debe le paga a quien más se le debe, hasta saldar todo. Deja como mucho un pago menos que el número de personas con Saldo distinto de cero; es una aproximación al mínimo.

## Grupos

- No hay roles: todos los Miembros vinculados pueden lo mismo (añadir Miembros, editar Gastos, cambiar la Simplificación de deudas, regenerar el Enlace de invitación).
- La Simplificación de deudas viene activada por defecto.
- Nadie puede salir ni ser sacado de un Grupo con Saldo distinto de cero; al salir pasa a ser Antiguo Miembro y pierde el acceso al Grupo, aunque su nombre sigue en los Gastos pasados.
- Cualquier Miembro vinculado puede sacar a otro Miembro, pendiente o vinculado, si su Saldo es cero.
- Quien vuelve a un Grupo del que salió recupera su Antiguo Miembro, con todo su historial; no se crea uno nuevo.
- El nombre y el email de un Miembro pendiente se pueden editar.
- Un Usuario ocupa como máximo un Miembro por Grupo.
- Un Grupo solo se puede borrar cuando todos los Saldos están en cero.

## Moneda del grupo

- Se puede cambiar en cualquier momento. El cambio reetiqueta todos los Gastos y Liquidaciones del Grupo, también los borrados (mismos números, otra moneda), y nunca aplica tipos de cambio.
- Si la moneda nueva tiene menos decimales (USD → CLP), el cambio solo se permite cuando ningún importe perdería céntimos, incluidos los de Gastos y Liquidaciones borrados, porque se pueden restaurar; si no, la aplicación indica qué corregir.
- Los Registros sin sincronizar creados antes del cambio llegan como Registros rechazados, para que su autor confirme la moneda.
- El cambio queda en la Actividad.
- Las monedas de fondos de ISO 4217 (por ejemplo CLF, COU o UYI) no se ofrecen en la interfaz.

## Personas

- Para Reclamar se usa el Enlace de invitación. Si el email de quien entra coincide con el de un Miembro pendiente, se le sugiere, pero nunca se vincula automáticamente.
- El Enlace de invitación de un Grupo se puede volver a mostrar cuantas veces haga falta; regenerarlo invalida el anterior (ver [ADR 0010](adr/0010-enlaces-de-invitacion-derivados.md)).
- Cualquier Miembro puede deshacer una Reclamación hecha por error; queda en la Actividad, y el Usuario desvinculado deja de ver el Grupo.
- Al borrar su cuenta, los Miembros y Amigos de un Usuario vuelven a quedar pendientes con su nombre, así que los Saldos de los demás no cambian.
- Quien entra por primera vez con un código llega sin nombre; la aplicación le pide su nombre y su Moneda preferida, proponiendo la del idioma del navegador. Con Google, el nombre viene de la cuenta.
- Pedir un código nuevo invalida el anterior.

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

## Presentación

- Los importes se muestran con el formato numérico de la región del Usuario (en México, 7,589.67; en Colombia, 7.589,67).
- En los listados se ocultan los decimales cuando son cero; en los formularios se muestran siempre.
- Al escribir un importe, si después del punto no vienen exactamente tres cifras, el punto se lee como separador decimal aunque en esa región separe los miles ("10.50" son 10,50 en Colombia).

## Sin conexión

- Se pueden crear Gastos y Liquidaciones; no Grupos, Miembros ni Amigos (ver [ADR 0001](adr/0001-offline-solo-creacion.md)).
- Un Registro rechazado nunca se descarta en silencio.
