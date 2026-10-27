# Product

<!-- impeccable:product-schema 1 -->

Registro de producto para el trabajo de interfaz. El vocabulario manda en [CONTEXT.md](../../../CONTEXT.md); las reglas, en [docs/reglas-de-negocio.md](../../../docs/reglas-de-negocio.md).

## Platform

web

## Users

Personas que comparten gastos, en dos escenas que pesan lo mismo en la v1:

- **Viaje entre amigos**: un Grupo temporal; los gastos se apuntan en la calle o en la mesa del restaurante, con una mano y a veces sin señal, y se liquidan al volver.
- **Casa compartida**: compañeros de piso o pareja; gastos recurrentes (mercado, servicios, arriendo) y una mirada ocasional a los Saldos.

Público hispanohablante internacional. Los textos van en español neutro, sin regionalismos, tuteando. Los datos de ejemplo mezclan monedas (EUR, USD y monedas latinoamericanas, incluidas las que no tienen decimales).

## Product Purpose

Registrar gastos compartidos y saber en todo momento quién le debe cuánto a quién. El nombre viene de "we're quits": el objetivo de cada Grupo y cada Amistad es quedar a mano. El éxito es que cualquiera llegue a Saldo cero sabiendo exactamente qué pago lo consigue.

## Positioning

- Se puede apuntar un Gasto o una Liquidación sin conexión; se sincroniza al volver la señal y nada se pierde en silencio (ADR 0001).
- No hace falta que todos tengan cuenta: un Miembro pendiente acumula su historial y la persona lo Reclama después con un Enlace de invitación.
- Cinco Métodos de reparto con previsualización de Cuotas mientras se escribe; el sobrante de un reparto no exacto se ve antes de guardar.
- Los Saldos se muestran por moneda y nunca se convierten.
- La aplicación no mueve dinero real: una Liquidación solo registra que alguien devolvió dinero.
- Sin contraseñas: código de 6 dígitos por email o Google.

## Operating Context

- Teléfono primero, como PWA instalada o en el navegador; también escritorio.
- En iOS la PWA instalada no comparte sesión con Safari, por eso el inicio de sesión es con código y no con enlace (ADR 0005).
- La interfaz lee siempre de la réplica local, así que se comporta igual con o sin conexión; sin conexión no se crean ni editan Grupos, Miembros ni Amigos.

## Capabilities and Constraints

- Términos y nombres en código: CONTEXT.md (ADR 0009). En los textos se dice "Saldo", nunca "balance"; "Liquidación", nunca "pago" ni "reembolso".
- Importes en enteros de la unidad mínima de su moneda (ISO 4217) y con sus decimales; nunca coma flotante.
- Un Gasto sin grupo es siempre entre dos personas (ADR 0004).
- La Simplificación de deudas viene activada por defecto y solo la calcula el servidor; sin conexión se muestran Saldos provisionales.
- v1 solo en español, con i18n preparada para más idiomas.
- Sin decidir: precio o modelo de negocio, dominio, iconografía y paleta definitivas.

## Brand Commitments

- El nombre, Quitsie, es lo único fijo. El icono, el color turquesa y el lema del esqueleto son provisionales.
- Debe transmitir confianza con el dinero sin ponerse serio.

## Evidence on Hand

No hay usuarios, testimonios, cifras ni capturas reales. Todo dato de las pantallas de la v1 es de prueba y no debe presentarse como real.

## Product Principles

1. El Saldo es la verdad visible: siempre claro quién debe a quién, cuánto y en qué moneda.
2. Apuntar un Gasto cuesta segundos, con o sin señal.
3. Nada se pierde en silencio: los Registros rechazados esperan al Usuario y los Gastos borrados se pueden restaurar.
4. Precisión con el dinero, ligereza en el tono.
5. Las personas antes que las cuentas: se puede compartir con quien todavía no usa la aplicación.
