# Reglas de negocio

Reglas acordadas durante el diseño. El vocabulario sigue [CONTEXT.md](../CONTEXT.md).

## Gastos

- Un Gasto tiene descripción, fecha (por defecto hoy, editable), un total mayor que cero y una Categoría opcional.
- Tiene al menos un Aporte y al menos una Cuota; los Aportes suman el total y las Cuotas también.
- Con Porcentajes, cada porcentaje admite hasta dos decimales y entre todos suman exactamente 100.
- Con Proporciones, los pesos son enteros positivos; una proporción como 1,5:1 se escribe 3:2.
- Si el reparto no es exacto (100 entre 3), el sobrante se asigna de forma determinista, se ve en la previsualización y las Cuotas se guardan ya calculadas: nunca se recalculan.
- Los importes respetan los decimales de su moneda.
- Puede editar o borrar un Gasto de Grupo cualquier Miembro vinculado; uno sin grupo, cualquiera de sus dos Participantes.
- Un Gasto borrado se puede restaurar sin límite de tiempo.
- Si dos personas editan el mismo Gasto a la vez, la segunda en guardar recibe un aviso y ve la versión nueva; nunca se sobrescribe en silencio.

## Grupos

- No hay roles: todos los Miembros vinculados pueden lo mismo (añadir Miembros, editar Gastos, cambiar la Simplificación de deudas, regenerar el Enlace de invitación).
- La Simplificación de deudas viene activada por defecto.
- Nadie puede salir ni ser sacado de un Grupo con Saldo distinto de cero; al salir pasa a ser Antiguo Miembro y pierde el acceso al Grupo, aunque su nombre sigue en los Gastos pasados.
- Un Miembro pendiente con Saldo cero se puede quitar del Grupo.
- Un Usuario ocupa como máximo un Miembro por Grupo.
- Un Grupo solo se puede borrar cuando todos los Saldos están en cero.

## Moneda del grupo

- Se puede cambiar en cualquier momento. El cambio reetiqueta todos los Gastos y Liquidaciones del Grupo (mismos números, otra moneda) y nunca aplica tipos de cambio.
- Si la moneda nueva tiene menos decimales (USD → CLP), el cambio solo se permite cuando ningún importe perdería céntimos; si no, la aplicación indica qué Gastos corregir.
- Los Registros sin sincronizar creados antes del cambio llegan como Registros rechazados, para que su autor confirme la moneda.
- El cambio queda en la Actividad.

## Personas

- Para Reclamar se usa el Enlace de invitación. Si el email de quien entra coincide con el de un Miembro pendiente, se le sugiere, pero nunca se vincula automáticamente.
- Cualquier Miembro puede deshacer una Reclamación hecha por error; queda en la Actividad.
- Al borrar su cuenta, los Miembros y Amigos de un Usuario vuelven a quedar pendientes con su nombre, así que los Saldos de los demás no cambian.

## Gastos sin grupo

- Siempre entre exactamente dos personas (ver [ADR 0004](adr/0004-gastos-sin-grupo-bilaterales.md)).
- Cada uno lleva su moneda; por defecto, la Moneda preferida del Usuario que lo crea.
- Los Saldos entre Amigos se muestran por moneda, sin conversión.

## Liquidaciones

- Solo se registran; la aplicación no mueve dinero real.
- Se permiten Liquidaciones parciales.

## Sin conexión

- Se pueden crear Gastos y Liquidaciones; no Grupos, Miembros ni Amigos (ver [ADR 0001](adr/0001-offline-solo-creacion.md)).
- Un Registro rechazado nunca se descarta en silencio.
