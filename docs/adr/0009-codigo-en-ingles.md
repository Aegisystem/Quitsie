# El código está en inglés; el glosario, en español

El dominio se define y se discute en español ([CONTEXT.md](../../CONTEXT.md)), pero todo el código (tipos, tablas, rutas de la API, carpetas) está en inglés. Para que la traducción no se desvíe con el tiempo, cada término del glosario declara su único nombre en código (Cuota = `OwedShare`, Aporte = `PaidShare`, Liquidación = `Settlement`…), y ese nombre es el que se usa siempre.

## Considered Options

- **Términos del dominio en español dentro del código** (`Gasto`, `Cuota`) y lo técnico en inglés: evita traducir, pero mezcla idiomas en cada línea.
- **Todo en español**: choca con las palabras clave, las librerías y la documentación, que están en inglés.

## Consequences

Añadir un término al glosario implica fijar también su nombre en inglés. Los textos que ve el Usuario siguen en español a través de i18n.
