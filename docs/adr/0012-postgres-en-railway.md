# PostgreSQL en Railway

Reemplaza al [ADR 0006](0006-sqlite.md) y al [ADR 0007](0007-go-nativo-en-vm-gratuita.md). Los datos pasan de SQLite a PostgreSQL, y el binario Go corre en Railway en lugar de en la VM gratuita de Google Cloud. La base se elige con una sola variable, `QUITSIE_DATABASE_URL`, así que cambiar de proveedor de PostgreSQL (Railway, Neon, Supabase, RDS) es cambiar esa URL. Mantener la VM costaba trabajo de operación (túnel, systemd, Litestream, alertas, despliegue por SSH) que una plataforma gestionada ya hace, y su IP pública ya no era gratis.

Se pasa solo a PostgreSQL, sin seguir soportando SQLite: mantener los dos motores obliga a escribir cada consulta, cada migración y cada prueba dos veces. Como todavía no hay datos reales, no hay que copiar nada: las migraciones de SQLite se reemplazan por una migración inicial de PostgreSQL.

## Considered Options

- **SQLite con un volumen en Railway**: funciona y es lo más barato, pero ata la base al disco de una plataforma y cambiar de proveedor exige mover un archivo y montar respaldos aparte.
- **SQLite y PostgreSQL a la vez, elegidos por variable**: deja las dos puertas abiertas a cambio de duplicar las 95 consultas, el código generado y las pruebas.
- **Seguir en la VM con PostgreSQL instalado en ella**: añade a la operación de la VM la de un servidor de base de datos.

## Consequences

- Las lecturas de la sincronización corren en `REPEATABLE READ`. En el aislamiento por defecto de PostgreSQL cada consulta ve su propia foto, y un pull podría avanzar el cursor saltándose cambios. Las escrituras siguen ordenadas por el bloqueo de la fila de `version_counter`.
- Las pruebas necesitan un servidor PostgreSQL: en local con `docker compose`, en la CI como service container. Cada prueba crea su propia base.
- Los respaldos son los de Railway. Restaurar uno exige ejecutar `quitsie record-restore` antes de volver a abrir la aplicación, para que los dispositivos no mezclen versiones ([arquitectura](../arquitectura.md#restaurar-un-respaldo)).
- Desaparecen la VM, Litestream, R2, el túnel de Cloudflare, el despliegue por SSH y las alertas propias. Railway despliega cada push a `main` que pase la CI y avisa de los fallos del servicio.
- Una sola instancia sigue siendo suficiente, pero ya no es obligatoria para la base: lo que todavía la exige son los límites que se guardan en memoria.
