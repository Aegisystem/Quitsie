# Grupos y Amistades comparten el mismo libro

Internamente, los Gastos y Liquidaciones viven en un libro (`Ledger`). Cada Grupo tiene uno, y cada Amistad también, como un libro de exactamente dos Miembros. Así Reclamar, Saldos, Actividad, permisos y sincronización usan el mismo código para los dos casos, en lugar de duplicarlo para los Gastos sin grupo. El libro es un detalle de implementación: el Usuario nunca ve esa palabra y el glosario sigue hablando de Grupos y Amistades.

## Consequences

- En una Amistad, el Amigo es un `Member` del libro, y un Amigo pendiente es un `Member` sin Usuario vinculado.
- Un libro de Amistad no tiene Moneda del grupo: cada Gasto lleva la suya.
- Las reglas propias de cada tipo (por ejemplo, que una Amistad tiene siempre dos Miembros) se validan según el tipo del libro.
