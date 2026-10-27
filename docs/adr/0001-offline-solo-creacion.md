# Sin conexión solo se crean Gastos y Liquidaciones

La PWA permite consultar sin conexión todos los datos del Usuario y crear Gastos y Liquidaciones nuevos, que quedan en cola y se envían al recuperar la conexión. Editar, borrar o crear Grupos, Miembros y Amigos exige estar en línea. Crear un Gasto o una Liquidación solo añade un registro, así que no hay ediciones concurrentes que reconciliar, y eso cubre el caso principal (apuntar un gasto, o un pago en efectivo, en un viaje sin señal) sin el costo de un modelo offline completo.

## Considered Options

- **Solo instalable** (la interfaz carga sin red, los datos no): no resuelve el caso del viaje sin señal.
- **Solo lectura sin conexión**: tampoco permite apuntar el gasto en el momento.
- **Offline completo** (crear, editar y borrar sin conexión): obliga a resolver conflictos como "Ana editó el Gasto mientras Beto lo borraba". Se descartó por complejidad; se puede revisar más adelante.

## Consequences

El cliente genera el identificador de cada registro, así que reenviar la cola nunca duplica nada. Si el servidor rechaza un Registro sin sincronizar (por ejemplo, porque sacaron al Usuario del Grupo mientras estaba sin señal), queda como Registro rechazado a la vista del Usuario; nunca se descarta en silencio.
