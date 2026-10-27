# Quitsie

Aplicación para registrar gastos compartidos entre personas y saber en todo momento quién le debe cuánto a quién.

Los términos están en español; el código usa el nombre en inglés indicado en cada uno (ver [ADR 0009](docs/adr/0009-codigo-en-ingles.md)).

## Personas

**Usuario**:
Persona con cuenta en la aplicación. Tiene una **Moneda preferida**, que se propone por defecto en sus Gastos sin grupo.
_En código_: `User`, `PreferredCurrency`
_Evitar_: cuenta, perfil

**Miembro**:
El lugar que ocupa una persona dentro de un Grupo. Puede estar vinculado a un Usuario o estar pendiente.
_En código_: `Member`
_Evitar_: integrante

**Miembro pendiente**:
Miembro que todavía no está vinculado a ningún Usuario; solo tiene un nombre y, opcionalmente, un email. Cuando un Usuario borra su cuenta, sus Miembros vuelven a quedar pendientes.
_En código_: pending member (un `Member` sin `User` vinculado)
_Evitar_: invitado, usuario fantasma

**Antiguo Miembro**:
Miembro que salió del Grupo. Sigue apareciendo en los Gastos pasados, pero ya no participa en nuevos ni ve el Grupo.
_En código_: former member
_Evitar_: ex-miembro, miembro inactivo

**Amigo**:
Otra persona con la que un Usuario comparte Gastos sin grupo. Puede estar vinculada a un Usuario o estar pendiente.
_En código_: `Friend`
_Evitar_: contacto

**Amigo pendiente**:
Amigo que todavía no está vinculado a ningún Usuario, en el mismo sentido que un Miembro pendiente.
_En código_: pending friend

**Amistad**:
Relación entre un Usuario y uno de sus Amigos, en la que viven sus Gastos sin grupo y sus Liquidaciones.
_En código_: `Friendship`

**Reclamar**:
Acción por la que un Usuario se vincula a un Miembro pendiente o a un Amigo pendiente y hereda todo su historial.
_En código_: `Claim`
_Evitar_: fusionar, aceptar invitación

**Enlace de invitación**:
Enlace revocable con el que una persona entra a través de otra. El de un Grupo permite Reclamar uno de sus Miembros pendientes o unirse como Miembro nuevo; el de un Amigo pendiente solo permite Reclamarlo.
_En código_: `InviteLink`

**Participante**:
Persona que, en un Gasto concreto, pagó algo, entra en el reparto, o ambas cosas; su Cuota puede quedar en cero por redondeo. En un Gasto de un Grupo es un Miembro; en un Gasto sin grupo, el Usuario o su Amigo.
_En código_: `Participant`

## Gastos

**Grupo**:
Conjunto de Miembros que comparten gastos. Todos sus Gastos se registran en una única moneda, la **Moneda del grupo**. Cambiarla reetiqueta los importes (mismos números, otra moneda); nunca los convierte.
_En código_: `Group`, `Group.Currency`, relabel

**Gasto**:
Registro de algo que pagaron uno o varios Participantes y que se reparte entre uno o varios Participantes.
_En código_: `Expense`
_Evitar_: transacción, movimiento

**Gasto sin grupo**:
Gasto que no pertenece a ningún Grupo. Siempre es entre exactamente dos personas, un Usuario y uno de sus Amigos, y lleva su propia moneda.
_En código_: un `Expense` de una `Friendship`
_Evitar_: gasto individual, gasto personal

**Aporte**:
Cantidad que un Participante pagó de un Gasto. Los Aportes de un Gasto suman su total.
_En código_: `PaidShare`
_Evitar_: pago, contribución

**Cuota**:
Cantidad que le corresponde pagar a un Participante de un Gasto. Las Cuotas de un Gasto suman su total.
_En código_: `OwedShare`
_Evitar_: parte, porción, deuda

**Método de reparto**:
Regla con la que se calculan las Cuotas de un Gasto: Partes iguales, Montos exactos, Porcentajes, Proporciones o Ajustes.
_En código_: `SplitMethod` (`equal`, `exact`, `percentage`, `weights`, `adjustment`)

**Proporciones**:
Método de reparto en el que cada Participante tiene un peso (por ejemplo 2:1:1) y su Cuota es proporcional a él.
_En código_: `weights`
_Evitar_: partes, acciones

**Ajustes**:
Método de reparto a partes iguales en el que a algunos Participantes se les suma o resta una cantidad fija.
_En código_: `adjustment`

**Categoría**:
Clasificación de un Gasto según en qué se gastó: comida, supermercado, transporte, alojamiento, vivienda, servicios, ocio, compras, salud, regalos u otros.
_En código_: `Category`

## Saldos

**Saldo**:
Lo que una persona tiene a favor (positivo) o en contra (negativo) en un Grupo o con un Amigo, en una moneda concreta. Nunca se convierte entre monedas.
_En código_: `Balance`
_Evitar_: balance (en los textos en español)

**Deuda**:
Cantidad que una persona le debe a otra persona concreta.
_En código_: `Debt`

**Simplificación de deudas**:
Reorganización de las Deudas de un Grupo para saldarlas con el menor número de pagos sin cambiar ningún Saldo. Se activa o desactiva por Grupo.
_En código_: debt simplification (`Group.SimplifyDebts`)

**Liquidación**:
Registro de que una persona le devolvió dinero a otra. Modifica los Saldos pero no es un Gasto; la aplicación no mueve dinero real.
_En código_: `Settlement`
_Evitar_: pago, reembolso

## Historial

**Actividad**:
Registro de un cambio (quién hizo qué y cuándo) en un Grupo o entre Amigos.
_En código_: `Activity`
_Evitar_: log, evento, notificación

**Gasto borrado**:
Gasto que ya no cuenta en los Saldos pero que puede restaurarse.
_En código_: deleted expense (`Expense.DeletedAt`)
_Evitar_: gasto eliminado

## Sin conexión

**Registro sin sincronizar**:
Gasto o Liquidación creado sin conexión que todavía no ha llegado al servidor.
_En código_: `UnsyncedRecord`
_Evitar_: gasto pendiente (se confunde con Miembro pendiente)

**Registro rechazado**:
Registro sin sincronizar que el servidor no aceptó y que espera a que el Usuario lo corrija o lo descarte.
_En código_: `RejectedRecord`
