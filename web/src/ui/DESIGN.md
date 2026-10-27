---
name: Quitsie
description: Gastos compartidos que se cuentan con fichas, a la manera de Isotype.
colors:
  paper: "#f4f5f2"
  surface: "#ffffff"
  sunken: "#e9ebe6"
  ink: "#1f2328"
  ink-2: "#5b616b"
  ink-3: "#8a9098"
  rule: "#dcdfd8"
  rule-strong: "#bcc0b8"
  favor: "#2f5da8"
  against: "#c03a25"
  even: "#2e7a4b"
  pending: "#e2a72e"
  pending-ink: "#8a5d00"
  bar: "#1f2328"
  on-bar: "#f4f5f2"
  on-bar-2: "#b6bbc2"
  on-color: "#ffffff"
  paper-dark: "#121417"
  surface-dark: "#1b1e22"
  sunken-dark: "#24282d"
  ink-dark: "#ecedea"
  ink-2-dark: "#a2a7ae"
  ink-3-dark: "#737981"
  rule-dark: "#2b2f35"
  rule-strong-dark: "#41464e"
  favor-text-dark: "#86a8ea"
  against-text-dark: "#f08a73"
  even-text-dark: "#6fc08d"
  pending-ink-dark: "#e9b44c"
  bar-dark: "#0a0b0d"
typography:
  display:
    fontFamily: "'Jost Variable', Futura, 'Century Gothic', 'Avenir Next', system-ui, sans-serif"
    fontSize: "clamp(2.75rem, 13vw, 4rem)"
    fontWeight: 500
    lineHeight: 1
    letterSpacing: "-0.025em"
    fontFeature: "'tnum', 'lnum'"
  display-compact:
    fontFamily: "'Jost Variable', Futura, 'Century Gothic', 'Avenir Next', system-ui, sans-serif"
    fontSize: "clamp(2rem, 8.5vw, 2.625rem)"
    fontWeight: 500
    lineHeight: 1
    letterSpacing: "-0.025em"
    fontFeature: "'tnum', 'lnum'"
  amount-entry:
    fontFamily: "'Jost Variable', Futura, 'Century Gothic', 'Avenir Next', system-ui, sans-serif"
    fontSize: "clamp(2.5rem, 12vw, 3.25rem)"
    fontWeight: 500
    letterSpacing: "-0.02em"
    fontFeature: "'tnum', 'lnum'"
  headline:
    fontFamily: "'Jost Variable', Futura, 'Century Gothic', 'Avenir Next', system-ui, sans-serif"
    fontSize: "1.75rem"
    fontWeight: 600
    lineHeight: 1.15
    letterSpacing: "-0.015em"
  title:
    fontFamily: "'Jost Variable', Futura, 'Century Gothic', 'Avenir Next', system-ui, sans-serif"
    fontSize: "1.125rem"
    fontWeight: 600
  row-title:
    fontFamily: "'Jost Variable', Futura, 'Century Gothic', 'Avenir Next', system-ui, sans-serif"
    fontSize: "1.0625rem"
    fontWeight: 500
  body:
    fontFamily: "'Jost Variable', Futura, 'Century Gothic', 'Avenir Next', system-ui, sans-serif"
    fontSize: "1rem"
    fontWeight: 400
    lineHeight: 1.45
  small:
    fontFamily: "'Jost Variable', Futura, 'Century Gothic', 'Avenir Next', system-ui, sans-serif"
    fontSize: "0.875rem"
    fontWeight: 400
  field-label:
    fontFamily: "'Jost Variable', Futura, 'Century Gothic', 'Avenir Next', system-ui, sans-serif"
    fontSize: "0.875rem"
    fontWeight: 600
  section-label:
    fontFamily: "'Jost Variable', Futura, 'Century Gothic', 'Avenir Next', system-ui, sans-serif"
    fontSize: "0.8125rem"
    fontWeight: 600
    letterSpacing: "0.08em"
  legend:
    fontFamily: "'Jost Variable', Futura, 'Century Gothic', 'Avenir Next', system-ui, sans-serif"
    fontSize: "0.8125rem"
    fontWeight: 400
rounded:
  base: "6px"
  check: "4px"
  pill: "999px"
  token: "50%"
spacing:
  gutter: "16px"
  gutter-wide: "24px"
  stack-sm: "8px"
  stack-md: "16px"
  form: "22px"
  section: "32px"
  token: "14px"
  token-band: "16px"
  token-chart: "13px"
  token-debt: "10px"
  column: "42rem"
  column-wide: "72rem"
  bar: "56px"
  tabbar: "64px"
components:
  button-primary:
    backgroundColor: "{colors.ink}"
    textColor: "{colors.paper}"
    typography: "{typography.body}"
    rounded: "{rounded.base}"
    padding: "0 20px"
    height: "48px"
  button-secondary:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.ink}"
    rounded: "{rounded.base}"
    padding: "0 20px"
    height: "48px"
  button-secondary-hover:
    backgroundColor: "{colors.sunken}"
  button-quiet:
    textColor: "{colors.ink}"
    rounded: "{rounded.base}"
    padding: "0 10px"
    height: "48px"
  button-quiet-hover:
    backgroundColor: "{colors.sunken}"
  button-small:
    rounded: "{rounded.base}"
    padding: "0 12px"
    height: "36px"
  button-light:
    backgroundColor: "{colors.on-color}"
    textColor: "{colors.ink}"
    rounded: "{rounded.base}"
    height: "48px"
  input:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.ink}"
    typography: "{typography.body}"
    rounded: "{rounded.base}"
    padding: "0 12px"
    height: "48px"
  chip:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.ink}"
    rounded: "{rounded.pill}"
    padding: "0 14px"
    height: "40px"
  chip-selected:
    backgroundColor: "{colors.ink}"
    textColor: "{colors.paper}"
  band-favor:
    backgroundColor: "{colors.favor}"
    textColor: "{colors.on-color}"
    typography: "{typography.display}"
    padding: "20px 16px 22px"
  band-against:
    backgroundColor: "{colors.against}"
    textColor: "{colors.on-color}"
    typography: "{typography.display}"
    padding: "20px 16px 22px"
  band-even:
    backgroundColor: "{colors.even}"
    textColor: "{colors.on-color}"
    typography: "{typography.display-compact}"
    padding: "20px 16px 22px"
  topbar:
    backgroundColor: "{colors.bar}"
    textColor: "{colors.on-bar}"
    height: "56px"
  tabbar:
    backgroundColor: "{colors.bar}"
    textColor: "{colors.on-bar-2}"
    height: "64px"
  offline-banner:
    backgroundColor: "{colors.pending}"
    textColor: "{colors.ink}"
    padding: "10px 16px"
  tag-pending:
    textColor: "{colors.pending-ink}"
    rounded: "{rounded.pill}"
    padding: "0 8px"
  notice:
    backgroundColor: "{colors.sunken}"
    rounded: "{rounded.base}"
    padding: "12px 14px"
  toast:
    backgroundColor: "{colors.ink}"
    textColor: "{colors.paper}"
    rounded: "{rounded.base}"
    padding: "12px 16px"
  code-cell:
    backgroundColor: "{colors.surface}"
    rounded: "{rounded.base}"
    height: "60px"
---

# Design System: Quitsie

## Overview

**Creative North Star: "La cuenta en fichas"**

Quitsie no se lee, se cuenta. Cada Saldo es una banda de color pleno con su importe a tamaño titular y, debajo, una fila de fichas iguales con su leyenda, como en los gráficos Isotype de Neurath y Arntz: cada ficha vale lo mismo, la fracción se corta y nunca se encoge, y quedar a mano es ver la fila vaciarse. El resto de la pantalla es tinta sobre un fondo neutro, impresa en plano: sin sombras, sin degradados que mezclen color, sin avatares de iniciales ni botón flotante.

El color nunca decora; es un dato. Azul es lo que te deben, rojo es solo «debes», verde es «a mano», ocre es lo pendiente o sin sincronizar. Un problema (registro rechazado, campo inválido, reparto que no cierra) no se pinta de rojo: se marca en tinta con la celda tachada. Las personas son figuras de silueta y su estado se lee en la propia figura (llena, hueca, apagada); las categorías son pictogramas de silueta llena de la misma familia.

La densidad es la de una herramienta de bolsillo: filas de 64px, botones de 48px, una sola columna de 42rem, la acción principal abajo al alcance del pulgar. La jerarquía sale de la escala y del peso, no de cajas ni elevación. El tono es cercano con un guiño («Así da gusto.», «El primero siempre es el café.») y preciso con el dinero.

**Key Characteristics:**
- Bandas de color pleno, una por moneda; los Saldos de monedas distintas nunca se suman.
- Fichas Isotype iguales con leyenda «cada ● = importe»; la última se corta.
- Tinta #1F2328 como único color de acción y de problema.
- Jost con cifras tabulares y alineadas en todo importe.
- Dos familias de iconos que no se mezclan: trazo de 2px para controles, silueta llena para contenido.
- Plano: profundidad por superficie (paper, surface, sunken) y por reglas de tinta, nunca por sombra.

## Colors

Color plano de imprenta: cuatro tintas con significado sobre una escala neutra ligeramente verdosa.

### Primary
- **Tinta de imprenta** (ink): todo el texto, la acción principal (fondo del botón primario), el estado seleccionado de chips y casillas, la regla de 2-3px que cierra cabeceras de día, el par de una Liquidación y la pestaña activa. También es el color de los problemas.
- **Barra de tinta** (bar): barra superior, barra de pestañas, cabecera del inicio de sesión e invitación a instalar. En oscuro baja casi a negro (bar-dark) para separarse del fondo.

### Secondary
- **Azul a favor** (favor): banda «Te deben», fichas y cifras de lo que se le debe al Usuario, lado derecho del gráfico de Saldos. Es también el color del anillo de foco.
- **Rojo en contra** (against): banda «Debes», fichas y cifras de lo que el Usuario debe, lado izquierdo del gráfico de Saldos. Nada más.

### Tertiary
- **Verde a mano** (even): banda «Estás a mano», etiqueta «a mano», el punto de equilibrio sobre el eje, el medidor del reparto cuando cierra justo y el fondo de la marca Quitsie (dos barras iguales).
- **Ocre de espera** (pending): banda sin conexión, chip de estado sin conexión y contador de registros. Siempre con texto en tinta encima.
- **Ocre de texto** (pending-ink): versión legible del ocre para texto y trazos sobre fondo claro: etiqueta «pendiente» discontinua, anillo de registro sin sincronizar, sobrante del reparto, coincidencia al Reclamar.

### Neutral
- **Papel** (paper): fondo de la aplicación, del dock y de las pestañas fijas.
- **Superficie** (surface): campos, chips, tarjetas de Reclamar, celdas del código.
- **Hundido** (sunken): avisos, caja del enlace, hover de filas y botones discretos.
- **Tinta 2** (ink-2): texto secundario, etiquetas de campo y de sección, leyendas.
- **Tinta 3** (ink-3): lo que salió o se borró (antiguo miembro, gasto borrado, participante fuera del reparto) y las fichas grises de las Deudas.
- **Regla** (rule) y **regla fuerte** (rule-strong): separadores de fila y bordes de campo; la fuerte también es el punto de un registro sincronizado.
- **Sobre color** (on-color): blanco puro para todo texto, figura y ficha dentro de una banda.

En modo oscuro las neutras se invierten (los tokens `-dark`) y las tintas de significado conservan el tono pero suben de luminosidad solo cuando son texto o ficha sobre el fondo (`favor-text-dark`, `against-text-dark`, `even-text-dark`, `pending-ink-dark`). Las bandas mantienen favor, against y even plenos con texto blanco en ambos modos. El tema sigue `prefers-color-scheme` y admite forzarse con `data-theme`.

### Named Rules
**The Rojo-Solo-Debes Rule.** El rojo significa únicamente «debes». Errores, rechazos, acciones destructivas y repartos que no cierran van en tinta con la marca tachada; el botón de peligro es tinta, igual que el primario.

**The Color-Es-Dato Rule.** Si un color no responde a «¿a favor, en contra, a mano o pendiente?», es tinta o neutro. Ninguna superficie lleva color de marca por decoración.

**The Ocre-Con-Tinta Rule.** El ocre pleno nunca lleva texto blanco ni texto ocre encima; sobre ocre se escribe en tinta, y el ocre como texto es siempre pending-ink.

## Typography

**Display Font:** Jost Variable (con Futura, Century Gothic, Avenir Next, system-ui)
**Body Font:** Jost Variable (la misma familia)

**Character:** Una sola geométrica de la tradición de Futura, la letra de los carteles Isotype; amable en los textos, exacta en las cifras. Todo importe usa cifras tabulares y alineadas para que las columnas de dinero cuadren.

### Hierarchy
- **Display** (500, clamp(2.75rem, 13vw, 4rem), 1, -0.025em): el importe de una banda cuando hay una sola moneda.
- **Display compacto** (500, clamp(2rem, 8.5vw, 2.625rem)): importes de banda cuando hay varias monedas y el título «A mano con todos.»; en escritorio sube a clamp(2.5rem, 3.6vw, 3.25rem).
- **Entrada de importe** (500, clamp(2.5rem, 12vw, 3.25rem), -0.02em): el campo del importe de un Gasto, sobre una regla de tinta de 3px.
- **Headline** (600, 1.75rem, 1.15, -0.015em): títulos de página; 1.375rem cuando es la leyenda de un formulario. Excepciones de marca: wordmark del inicio de sesión (2.75rem) y nombre del grupo al invitar (2.25rem).
- **Title** (600, 1.125rem): título en la barra superior; 1.25rem en el perfil y en la invitación a instalar.
- **Título de fila** (500, 1.0625rem): nombre de Grupo, amigo o Gasto; el importe al final de la fila usa el mismo tamaño.
- **Body** (400, 1rem, 1.45): texto corrido; la entradilla sube a 1.0625rem con un máximo de 34em.
- **Small** (400, 0.875rem): subtítulos de fila, pistas (máx. 40em), notas del dock.
- **Etiqueta de campo** (600, 0.875rem, ink-2).
- **Etiqueta de sección** (600, 0.8125rem, 0.08em, MAYÚSCULAS, ink-2): solo nombra secciones reales (Grupos, Amigos, Deudas sugeridas).
- **Leyenda** (400, 0.8125rem, ink-2): «cada ● = 10 €» y metadatos.

### Named Rules
**The Cifra-Tabular Rule.** Todo importe, contador, fecha de campo y celda de código lleva `font-variant-numeric: tabular-nums lining-nums`. Un importe nunca se parte en dos líneas.

**The Mayúsculas-Solo-Para-Secciones Rule.** Las mayúsculas espaciadas nombran una sección que existe y nada más; no se usan para antetítulos, categorías ni adornos sobre un título.

**The Jerarquía-Por-Escala Rule.** La jerarquía se marca con tamaño y peso (400, 500, 600). No se usan cajas, fondos ni color para subir el rango de un texto.

## Layout

Mobile-first, una columna. El contenido vive en una columna de 42rem centrada con un margen lateral de 16px (24px desde 900px). Las bandas de Saldo y la invitación a instalar rompen la columna a todo el ancho; la barra superior alinea su contenido a 72rem.

- **Barra superior** fija de 56px (más el área segura). **Barra de pestañas** fija abajo de 64px con Inicio, Actividad y Cuenta.
- **Dock**: la acción principal («Añadir gasto», «Liquidar» + «Añadir gasto», «Guardar») va en un dock fijo sobre la barra de pestañas, a lo ancho y al alcance del pulgar. El contenido reserva su altura en el padding inferior.
- **Ritmo vertical**: 32px entre secciones, 22px entre campos de formulario, 8px entre controles agrupados, filas de 64px de alto mínimo separadas por una regla de 1px.
- **Desde 900px**: la barra de pestañas desaparece y su navegación pasa a la barra superior; Inicio se ensancha a 72rem, las bandas se ponen en una fila (rejilla `auto-fit` de mínimo 16rem con 4px entre ellas), Grupos y Amigos van en dos columnas con 56px de calle, y «Añadir gasto» se mueve del dock a la derecha bajo las bandas. Las demás pantallas conservan la columna de 42rem y el dock se alinea a la derecha con un máximo de 22rem.
- **Inicio de sesión**: columna de 28rem bajo una cabecera de tinta.
- Las pestañas del Grupo (Gastos, Saldos, Deudas, Actividad) quedan fijas bajo la barra superior al desplazarse.

## Elevation & Depth

El sistema es plano. No hay `box-shadow` de elevación en ningún elemento. La profundidad se expresa con tres superficies (paper, surface, sunken), con reglas (1px rule para separar filas, 2-3px de tinta para cerrar un bloque o marcar lo activo) y con capas de tinta (barras superior e inferior). Los únicos `box-shadow` del código son anillos interiores (`inset 0 0 0 1-2px`) que dibujan bordes: el chip de estado, el registro sin sincronizar, la tarjeta de Reclamar elegida, la celda activa del código, el punto de equilibrio.

### Named Rules
**The Imprenta-Plana Rule.** Nada flota. Si algo necesita separarse, cambia de superficie o recibe una regla de tinta; nunca una sombra difusa ni un degradado que mezcle dos colores.

**The Corte-Duro Rule.** Los únicos `linear-gradient` permitidos son de corte duro: el relleno del medidor del reparto, el rayado de exceso y la flecha de los selectores. Ninguno mezcla tonos.

## Shapes

Un solo radio suave (6px) para botones, campos, avisos, tarjetas, toasts y celdas; píldora (999px) para chips, etiquetas, interruptor y chip de estado; círculo perfecto para fichas, marcas de sincronización y el punto de equilibrio; 4px para la casilla del reparto. Las bandas de Saldo no tienen radio: son campos de color a sangre, separados por 3px de papel (en fila, 4px de calle).

### Fichas
La ficha es un círculo lleno del color del texto (`currentColor`), 14px por defecto: 16px en bandas, 13px en el gráfico de Saldos, 10px en Deudas, 18px en el inicio de sesión. Separación de 4px. La unidad se elige de 1, 2 o 5 por potencia de diez para que no haya más de 10 fichas y nunca vale menos de una unidad entera de la moneda.

### Iconos: dos familias
- **Controles**: trazo de 2px en una rejilla de 24, extremos y uniones redondeados, `fill: none` (volver, añadir, cerrar, compartir, copiar, bandeja, nube en línea, nube tachada, flechas de sincronizar, casa, actividad, cuenta, flecha, chevron, restaurar, descargar). La pestaña activa engrosa el trazo a 2.5.
- **Contenido**: silueta llena en rejilla de 24 (persona, Grupo de tres figuras, pictogramas de categoría: cubiertos, bolsa, tranvía, cama, casa, rayo, entrada, cuatro cuadros; flecha de Liquidación con su retorno al 55 %). Estado de la enmienda del 2026-10-27, **pendiente de confirmación del usuario**: el contrato original pedía trazo uniforme para las categorías; lo construido son siluetas llenas de la familia de las figuras.

### Figuras de persona
Llena si el Miembro está vinculado a un Usuario; hueca (trazo de 1.75) si está pendiente; apagada a ink-3 si es antiguo miembro. El estado vive en la figura, no en una etiqueta aparte.

### Marca
Cuadrado de 32 con radio 7, verde a mano, y dos barras blancas iguales de 18 × 4.5 con extremos redondos: «we're quits». El favicon y los PNG de la PWA salen de la misma geometría.

## Components

### Buttons
Firmes y planos; la acción se distingue por el relleno de tinta, no por color.
- **Shape:** radio suave (6px), alto 48px, padding horizontal 20px, peso 600; icono de 24px con 8px de separación.
- **Primary:** fondo tinta, texto papel. Hover: tinta con un 14 % de azul. Active: baja 1px. Es también el botón de peligro.
- **Secondary:** fondo surface con borde de tinta de 1px; hover a sunken. Acompaña al primario en el dock («Liquidar»).
- **Quiet:** sin fondo, texto subrayado (offset 3px), peso 500; hover a sunken. Para acciones de sección («Nuevo grupo»).
- **Small:** 36px, 0.875rem. **Light / Ghost light:** blanco con texto tinta, o contorno blanco al 45 %, solo sobre la barra de tinta.
- **Disabled:** opacidad 0.5, sin desplazamiento; las acciones que necesitan conexión se deshabilitan sin conexión y lo explican en `title`.

### Chips
- **Style:** píldora de 40px, surface con borde rule-strong, peso 500; el input real queda invisible encima.
- **State:** seleccionado en tinta llena con texto papel; hover oscurece el borde a ink-2; foco con el anillo azul alrededor del chip. En móvil la fila de chips se desplaza en horizontal a sangre.

### Cards / Containers
- **Corner Style:** 6px.
- **Background:** sunken para avisos y la caja del enlace; surface con borde rule-strong para tarjetas de Reclamar y selector de persona.
- **Shadow Strategy:** ninguna (ver Elevation & Depth); la tarjeta elegida suma un anillo interior de tinta.
- **Internal Padding:** 12-14px.
- La mayoría del contenido no va en tarjetas sino en filas separadas por reglas.

### Inputs / Fields
- **Style:** 48px, surface, borde rule-strong de 1px, radio 6px, etiqueta encima en 0.875rem/600 ink-2.
- **Focus:** borde de tinta más anillo azul de 2px separado 1px.
- **Error:** borde de tinta de 2px y mensaje en tinta 600 precedido de la marca tachada; nunca rojo.
- **Disabled:** opacidad 0.45.
- **Importe:** sin caja, cifra grande sobre una regla de tinta de 3px que pasa a azul con el foco; moneda a la derecha en un selector compacto de 40px.
- **Código de 6 dígitos:** seis celdas de 60px; la actual se marca con borde de tinta doble y el grupo con el anillo de foco.

### Navigation
- **Barra superior:** tinta, 56px. Wordmark (marca + «Quitsie», 600) o botón de volver y título de pantalla. A la derecha, siempre, el estado de conexión: chip con nube y texto («En línea»), flechas mientras envía, nube tachada sobre ocre sin conexión; junto a un título se reduce al icono. A su lado la bandeja de Registros con contador ocre (blanco si hay rechazados).
- **Barra de pestañas (móvil):** tinta, tres destinos con icono y etiqueta de 0.75rem; activo en blanco con trazo 2.5.
- **Escritorio:** la misma navegación en la barra superior, activo con fondo blanco al 10 %.
- **Pestañas del Grupo:** texto ink-2 500; activa en tinta con subrayado de 3px.

### Banda de Saldo (componente de firma)
Un campo de color pleno por moneda: «Te deben» o «Debes» (1.0625rem, blanco al 88 %), el importe en Display y la fila de fichas blancas con su leyenda. Varias monedas apiladas con 3px de papel y tipografía compacta; en escritorio, en una fila. A mano: banda verde con dos figuras y un «=» entre ellas. En el Grupo lleva al pie el enlace «De dónde sale».

### Gráfico de Saldos
Leyenda arriba («A la izquierda, lo que se debe; a la derecha, lo que se le debe a cada uno» y «cada ● = importe»). Por Miembro: figura, nombre, etiqueta y cifra; debajo, un eje vertical de tinta de 2px con fichas rojas hacia la izquierda (alineadas al eje, la fracción cortada por su lado exterior) o azules hacia la derecha, y un anillo verde sobre el eje si está a mano. Cada cifra despliega su prueba: la lista de Gastos y Liquidaciones que la forman.

### Reparto
Filas de participante con casilla de tinta, figura, valor y Cuota tabular. Mientras se escribe un valor, las demás filas bajan al 45 % (un foco a la vez). Debajo, un medidor de 10 celdas que se llena en tinta, se raya si se pasa y se vuelve verde cuando cierra justo, con el texto de cierre en tinta tachada o en verde con check. El sobrante de céntimos aparece en ocre sobre la Cuota.

### Marcas de estado de registro
Celda fija de 10px al final de la fila: punto rule-strong si está sincronizado, anillo ocre si no, círculo tachado en tinta si fue rechazado.

### Movimiento
Una sola curva de salida, `cubic-bezier(0.16, 1, 0.3, 1)`. Las fichas entran escalando de 0.3 a 1 en 0.5s con 40ms de escalonado; el toast sube 12px en 0.45s; el interruptor se desliza en 0.3s; los botones cambian de fondo en 0.15s. Con `prefers-reduced-motion` se apagan las animaciones de fichas y toast y las transiciones del interruptor y del reparto.

### Voz
Español neutro, tuteo, frases cortas. Vocabulario fijo: «Saldo», nunca «balance»; «Liquidación», nunca «pago» ni «reembolso». Un guiño ligero en estados vacíos y en el a mano («Así da gusto.», «Cuentas claras, amistades largas.»); precisión seca en errores y dinero.

## Do's and Don'ts

### Do:
- **Do** muestra cada Saldo como banda de color pleno por moneda con importe en Display y fila de fichas con leyenda «cada ● = importe».
- **Do** corta la última ficha para representar una fracción; todas las fichas de un gráfico miden lo mismo.
- **Do** pon una leyenda en cada gráfico de fichas, visible y también para lectores de pantalla.
- **Do** marca cualquier problema en tinta con la marca de círculo tachado (12px) delante del texto.
- **Do** dibuja a las personas con la figura de silueta: llena vinculada, hueca pendiente, ink-3 antigua.
- **Do** usa trazo de 2px en rejilla de 24 para controles y silueta llena para personas, grupos y categorías.
- **Do** escribe todo importe con cifras tabulares y alineadas, sin saltos de línea.
- **Do** separa con superficies y reglas de tinta (1px, 2px, 3px), nunca con sombra.
- **Do** deja la acción principal en el dock inferior en móvil y respeta el área segura.
- **Do** escribe texto en tinta sobre el ocre pleno.

### Don't:
- **Don't** uses rojo para errores, rechazos, borrar o validar; el rojo es solo «debes».
- **Don't** escales una ficha para mostrar una fracción ni mezcles tamaños de ficha dentro de un mismo gráfico.
- **Don't** sumes ni conviertas Saldos de monedas distintas en una sola banda o cifra.
- **Don't** uses avatares de iniciales, fotos de perfil ni botón flotante.
- **Don't** añadas sombras de elevación, degradados que mezclen color ni fondos de color decorativo.
- **Don't** mezcles las dos familias de iconos: ni controles rellenos ni pictogramas de trazo.
- **Don't** pongas antetítulos ni etiquetas en mayúsculas espaciadas fuera de los nombres de sección.
- **Don't** uses un punto de color para el estado de conexión; el estado es un icono de nube para que no se confunda con una ficha.
