# Backend en Go, frontend en TypeScript

El servidor se escribe en Go y el cliente PWA en TypeScript. Se consideró usar TypeScript en ambos lados para compartir tipos y lógica de cálculo, pero se eligió Go por interés en el lenguaje.

## Consequences

Los tipos se comparten mediante un contrato de API, no importando código. La lógica que necesiten ambos lados (por ejemplo, previsualizar repartos o saldos sin conexión) no se puede reutilizar directamente: hay que decidir explícitamente en qué lado vive cada cálculo.
