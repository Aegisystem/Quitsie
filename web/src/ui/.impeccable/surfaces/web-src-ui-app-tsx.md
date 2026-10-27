---
version: 1
slug: "web-src-ui-app-tsx"
primary_target: "web/src/ui/App.tsx"
related_targets: []
---

# Superficie: pantallas de la v1 de Quitsie (web/src/ui)

Modo: Operate. Mobile-first, también escritorio.

Tarea: ver de un vistazo quién le debe cuánto a quién en cada moneda, apuntar un Gasto en segundos (con o sin señal), registrar Liquidaciones, Reclamar Miembros pendientes y no perder nunca un Registro rechazado.

Pantallas: inicio de sesión (código de 6 dígitos y Google), inicio, detalle de Grupo (Gastos, Saldos, Deudas sugeridas, Actividad), nuevo Gasto, Liquidación, invitación y Reclamar, sin conexión y bandeja de registros, invitación a instalar.

Decisiones del usuario (2026-10-27): dirección Isotype; azul a favor, rojo en contra, verde para «a mano»; tipografía geométrica (Jost); tono cercano con un guiño. Público hispanohablante internacional, tuteo; solo el nombre Quitsie es fijo.

Restricciones: datos de prueba dentro de web/src/ui; sin tocar web/src/domain, api, local ni sync.

## Direction contract

THESIS: Las Deudas se cuentan, no se leen. Cada Saldo es una fila de fichas iguales con su leyenda, a la manera de Isotype, y quedar a mano es ver la fila vaciarse. Rechaza el estándar de la categoría: lista blanca con verde/naranja, avatares de iniciales y botón flotante.

OWN-WORLD: color plano de imprenta sin sombras ni degradados: tinta #1F2328, azul a favor, rojo en contra, verde a mano y ocre para lo pendiente, sobre un fondo neutro. Jost con cifras tabulares, títulos en mayúsculas espaciadas solo para secciones. Personas como figuras de silueta: llenas si están vinculadas, huecas si están pendientes, grises si salieron. Categorías como pictogramas de trazo uniforme.

STORY: el Usuario entiende en un segundo cuánto le deben o debe en cada moneda, ve por qué (las cifras llevan su prueba), apunta el Gasto, ve las Cuotas cerrar justo antes de guardar y confía en que nada se pierde sin conexión.

FIRST VIEWPORT: barra de tinta con el nombre Quitsie y el estado de conexión. Debajo, una banda de color pleno por moneda (azul o rojo) con «Te deben» / «Debes», el importe a tamaño titular y la fila de fichas con su leyenda. Luego las secciones Grupos y Amigos. La acción principal, «Añadir gasto», va abajo, al alcance del pulgar, sobre la barra de pestañas.

FORM: Isotype (Neurath y Arntz), candidato 7 de 7 de mi lista, asignado por sorteo; clave c32facdf. Aportes de los descartados: jerarquía solo por escala; cada cifra con su prueba; un foco a la vez en el reparto; color pleno en campos enteros; estado en una celda fija (llena, hueca, tachada); el reparto tiene que cerrar justo. Movimiento propio: la fila de fichas se llena o se vacía cuando cambia un Saldo.

FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance
