# El cliente calcula las Cuotas; el servidor solo valida

El cliente aplica el Método de reparto y envía las Cuotas ya calculadas. El servidor no reimplementa los métodos de reparto: comprueba que Aportes y Cuotas sumen el total y que los Participantes sean válidos, y guarda el Método de reparto solo para poder reeditar el Gasto. La Simplificación de deudas se calcula solo en el servidor; sin conexión, el cliente muestra Saldos provisionales, que son simples sumas. Así la lógica no se duplica entre Go y TypeScript (ver ADR 0002).

No abre un hueco de seguridad: cualquier Miembro ya puede fijar Cuotas arbitrarias desde la interfaz usando Montos exactos.

## Considered Options

- **Duplicar la lógica en Go y TypeScript** con casos de prueba compartidos: dos implementaciones que mantener sincronizadas.
- **Compilar Go a WebAssembly** para el cliente: unos 2 MB más de descarga y una cadena de compilación más compleja.
