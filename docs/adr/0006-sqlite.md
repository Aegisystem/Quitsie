# SQLite como base de datos

Por ahora los datos viven en SQLite, en el mismo servidor que la API. Se prefirió a PostgreSQL, que era la recomendación inicial para un producto público, porque es más simple de operar: un archivo, sin servicio aparte y sin costo.

## Consequences

- Solo una instancia de la API escribe en la base de datos: se escala con una máquina más grande, no con más máquinas.
- El servidor necesita disco persistente, lo que descarta plataformas efímeras sin volumen.
- Los respaldos hay que montarlos aparte.
- Para no cerrar la puerta a PostgreSQL, el SQL evita características exclusivas de SQLite.
