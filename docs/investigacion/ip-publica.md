# IP pública de la VM (decisión 5)

Investigación del 2026-11-01 para la decisión 5 de `docs/pendientes.md`. Parte de lo que hacen hoy `deploy/first-deploy-wizard.sh` y `deploy/provision.sh`: VM `e2-micro` en `us-east1-b`, red `default`, IPv4 externa efímera, sin puertos de entrada, administración por IAP y entrada pública por Cloudflare Tunnel.

Cada dato lleva una marca:

- **Verificado:** leído hoy en la fuente oficial (página, código fuente de la documentación o ayuda de `gcloud` 570.0.0).
- **Extracto:** la página oficial está bloqueada en esta sesión. El dato viene del extracto del buscador, con enlace a la página.
- **Sin verificar:** no encontré fuente fiable.

Límite de esta sesión: no pude consultar registros AAAA. El resolvedor DNS del contenedor está filtrado (devuelve vacío incluso para `pypi.org`) y los servicios DNS sobre HTTPS (`cloudflare-dns.com`, `dns.google`) están bloqueados. Tampoco se abren `docs.cloud.google.com`, `developers.cloudflare.com` (lo leí en su fuente de GitHub), `docs.oracle.com` ni `hetzner.com`.

## Resumen

| Opción | Costo al mes | Qué rompe | Estado |
| --- | --- | --- | --- |
| A. Pagar la IPv4 externa (como ahora) | 3,65 USD | Nada | Verificado |
| B. VM dual-stack: IPv4 solo interna + IPv6 externa | 0 USD | GitHub (sin IPv6). R2 y Resend, si no tienen IPv6 | Viable en papel, sin probar |
| C. VM solo IPv6 | 0 USD | Lo mismo que B, más el SSH por IAP | Peor que B |
| D. Sin IP externa + Cloud NAT | ~4,67 USD + 0,045 USD/GiB | Nada | Más caro que A |
| E. Sin IP externa + Private Google Access | 0 USD | El túnel, R2, Resend, Debian y GitHub | No sirve |
| F. Otro proveedor | 0 USD (Oracle) o 2–6 USD | Rehacer el despliegue; riesgos propios | Ver sección 4 |

## 1. Precio de la IPv4 externa en GCE

Fuente: [precios de VPC](https://cloud.google.com/vpc/network-pricing), sección «External IP address pricing». **Verificado.**

| Concepto | Precio |
| --- | --- |
| IPv4 estática o efímera en uso en una VM estándar | 0,005 USD/hora |
| La misma en una VM Spot o interrumpible | 0,0025 USD/hora |
| IPv4 usada por Cloud NAT | 0,005 USD/hora |
| IPv4 estática reservada sin usar | 0,01 USD/hora |
| IPv6 externa de una VM o de una subred | Sin cargo |

- La capa gratuita de la IPv4 es **una hora al mes por cuenta**: «This free usage is limited to one hour per month per account». No cubre la IP de la `e2-micro`.
- Con 730 horas al mes son 3,65 USD (3,645 USD restando la hora gratis). En un mes de 31 días son unos 3,72 USD.
- La IP efímera solo se cobra mientras la VM está encendida. La estática se cobra también con la VM parada.
- La página de la capa gratuita dice solo «1 e2-micro instance per month» ([cloud.google.com/free](https://cloud.google.com/free), **verificado**). La página con el detalle ([free-cloud-features](https://docs.cloud.google.com/free/docs/free-cloud-features)) no se pudo abrir.
- Una VM Spot abarata la IP a la mitad, pero la `e2-micro` gratuita es una VM estándar, y una Spot puede pararse en cualquier momento. No sirve.

**Salida de datos.** El nivel Premium es el predeterminado y el que usa el asistente. Da 1 GiB gratis al mes hacia Norteamérica y después cobra 0,12 USD/GiB. El nivel Standard da 200 GiB gratis al mes, pero solo existe para IPv4: la IPv6 externa solo admite Premium (`gcloud compute instances create --help`, `--ipv6-network-tier`: «only one value is supported: PREMIUM»). **Verificado.** Es decir, pasar a IPv6 no empeora la salida respecto a hoy, y pagar la IPv4 permitiría además pasar a Standard si la salida crece. No comprobé que Standard esté disponible en `us-east1`.

## 2. IPv6 en GCE

### 2.1 Qué permite Google

| Hecho | Fuente | Estado |
| --- | --- | --- |
| Una interfaz puede ser `IPV4_ONLY`, `IPV4_IPV6` (dual-stack) o `IPV6_ONLY` | `gcloud compute instances create --help`, `--stack-type` | Verificado |
| Una subred puede ser `IPV4_IPV6` o `IPV6_ONLY`, con IPv6 `EXTERNAL` o `INTERNAL` | `gcloud compute networks subnets create --help` | Verificado |
| La IPv6 externa no se cobra | [precios de VPC](https://cloud.google.com/vpc/network-pricing) | Verificado |
| Las subredes con IPv6 solo existen en redes en modo personalizado. La red `default` es automática, así que el asistente no puede usarla | [Subnets](https://docs.cloud.google.com/vpc/docs/subnets) | Extracto |
| Solo IPv6 funciona con imágenes Debian, Red Hat y Ubuntu. No dice qué versiones de Debian | [Create an instance that uses IPv6](https://docs.cloud.google.com/compute/docs/instances/create-ipv6-instance) | Extracto |
| En una VM solo IPv6, el servidor de metadatos está en `fd20:ce::254` | [Query metadata](https://docs.cloud.google.com/compute/docs/metadata/querying-metadata), [troubleshooting](https://docs.cloud.google.com/compute/docs/troubleshooting/troubleshoot-metadata-server) | Extracto |
| Una VM dual-stack puede crearse con «External IPv4 address: None» y su IPv6 externa | [Create an instance that uses IPv6](https://docs.cloud.google.com/compute/docs/instances/create-ipv6-instance) | Extracto. En `gcloud` existen `--no-address` y `--ipv6-network-tier`, pero no probé la combinación |
| IAP TCP (el SSH del asistente) no llega a una VM solo IPv6: necesita una IP interna IPv4 | [respuesta en el foro de Google](https://discuss.google.dev/t/failed-to-lookup-instance-when-using-iap-ssh-on-ipv6-only-vm/190749); la [documentación de IAP](https://docs.cloud.google.com/iap/docs/tcp-forwarding-overview) dice que usa IP internas | Comunidad, sin confirmación oficial |

Sobre el agente invitado y OS Login en una VM solo IPv6 no encontré documentación específica. Dependen del servidor de metadatos, que sí responde por IPv6, pero está **sin verificar**.

Conclusión: la variante útil es la **dual-stack** (opción B). Conserva la IPv4 interna, así que IAP, el servidor de metadatos en `169.254.169.254`, el DNS y la hora siguen como hoy. Solo cambia que la salida a Internet va por IPv6. La variante solo IPv6 (C) además rompe el acceso de emergencia y la copia de `deploy/` del asistente, que van por IAP.

### 2.2 Cada dependencia por IPv6

| Dependencia | Host | ¿IPv6? | Estado y fuente | Qué hacer |
| --- | --- | --- | --- | --- |
| Cloudflare Tunnel | `region1.v2.argotunnel.com`, `region2.v2.argotunnel.com`, puerto 7844 | Sí: `2606:4700:a0::1`–`::10` y `2606:4700:a8::1`–`::10` | Verificado: [tunnel with firewall](https://developers.cloudflare.com/cloudflare-one/networks/connectors/cloudflare-tunnel/configure-tunnels/tunnel-with-firewall/) ([fuente](https://github.com/cloudflare/cloudflare-docs/blob/production/src/content/partials/cloudflare-one/tunnel/firewall/required-ports.mdx)) | `cloudflared` conecta por IPv4 por defecto. Añadir `TUNNEL_EDGE_IP_VERSION=6` a `/etc/quitsie/cloudflared.env` ([edge-ip-version](https://developers.cloudflare.com/cloudflare-one/networks/connectors/cloudflare-tunnel/configure-tunnels/run-parameters/#edge-ip-version), valores `auto`, `4`, `6`) |
| Actualizaciones de `cloudflared` | `update.argotunnel.com`, GitHub | No hace falta | La unidad usa `--no-autoupdate`; se actualiza por apt | Nada |
| Cloudflare R2 | `<ACCOUNT_ID>.r2.cloudflarestorage.com` | **Sin verificar** | No encontré documentación ni registro | Comprobar antes de elegir B. Sin IPv6, Litestream no respalda: bloquea la opción |
| Resend | `api.resend.com` | **Sin verificar** | No encontré documentación ni registro | Comprobar antes de elegir B. Sin IPv6 no salen los emails. Salida: un Worker de Cloudflare como relé (plan gratis: [100.000 peticiones al día](https://developers.cloudflare.com/workers/platform/limits/#daily-requests), verificado), que es código nuevo |
| Google OAuth | `accounts.google.com`, `oauth2.googleapis.com`, `www.googleapis.com` | Casi seguro sí | Extracto: Google publica rangos IPv6 para sus APIs (`2600:2d00:0002:1000::/56`, `2001:4860:8040::/42`) en [Private Google Access](https://docs.cloud.google.com/vpc/docs/configure-private-google-access). Sin `dig` | Comprobar con `dig` |
| apt de Debian | `deb.debian.org` (Fastly) | Probable sí | `debian.map.fastly.net` tenía AAAA en 2021 (hilo de NANOG: [1](https://seclists.org/nanog/2021/Oct/612), [2](https://seclists.org/nanog/2021/Oct/614)); `debian.org` tenía AAAA de Fastly en junio de 2026 ([WireWiki](https://www.wirewiki.com/domains/debian.org/dns-records/aaaa/2026-06-01)). El viejo problema de los servidores DNS de Fastly solo IPv4 no afecta aquí: resuelve el resolvedor de Google, no la VM (deducción) | Comprobar con `dig` |
| apt de Google y de Cloudflare | `packages.cloud.google.com`, `pkg.cloudflare.com` | **Sin verificar** | Son de Google y Cloudflare, que publican IPv6 en casi todo | Comprobar con `dig` |
| GitHub (Litestream) | `github.com/.../releases/download/...` | **No** | `github.com` seguía sin AAAA en 2026 ([blog, febrero de 2026](https://b.j4.lc/2026/02/11/github-and-ipv6-in-2026/); [Hacker News, 2026](https://news.ycombinator.com/item?id=47789617); petición abierta en [community #10539](https://github.com/orgs/community/discussions/10539)). La descarga empieza en `github.com`, así que falla aunque `*.githubusercontent.com` tuviera IPv6 (eso está sin verificar) | Descargar el `.deb` en la máquina del dueño y copiarlo con `deploy/` por IAP. El SHA-256 fijado en `provision.sh` lo sigue protegiendo. Exige cambiar `provision.sh` y el asistente |
| Despliegue desde GitHub Actions | Runner → Cloudflare Access → túnel | No depende de la VM | El runner entra por Cloudflare; el túnel ya lo abrió la VM | Nada |
| SSH de emergencia | IAP, `35.235.240.0/20` → IPv4 interna | Sí en dual-stack, no en solo IPv6 | Ver 2.1 | Nada en B |

Cualquier servicio que se añada en el futuro y solo tenga IPv4 tampoco funcionará en B. Se arregla añadiendo la IPv4 externa a la VM, y entonces se paga.

## 3. Sin IP externa

**Cloud NAT.** Cobra 0,0014 USD/hora por VM (1,02 USD al mes), 0,005 USD/hora por su IPv4 (3,65 USD al mes) y 0,045 USD por GiB procesado, más la salida normal. Para una VM son unos 4,67 USD al mes más los datos: más que la IPv4 directa ([precios de Cloud NAT](https://cloud.google.com/vpc/network-pricing), **verificado**). El NAT64 para VM solo IPv6 usa la misma pasarela y sus IPv4, así que cuesta lo mismo ([DNS64 y NAT64](https://docs.cloud.google.com/vpc/docs/connect-ipv6-to-ipv4), extracto). Solo tiene sentido con muchas VM.

**Private Google Access.** Deja a una VM sin IP externa llegar a las APIs de Google, como `*.googleapis.com`. En `accounts.google.com` solo cubre las rutas de OAuth de cuentas de servicio ([Private Google Access](https://docs.cloud.google.com/vpc/docs/private-google-access), [configuración](https://docs.cloud.google.com/vpc/docs/configure-private-google-access), extracto). No llega a Cloudflare, R2, Resend, Debian ni GitHub. Sin salida a Cloudflare el túnel no arranca y la aplicación queda inaccesible. No sirve sola.

## 4. Otros proveedores

| Proveedor y plan | Precio al mes | IPv4 | Problemas | Fuente |
| --- | --- | --- | --- | --- |
| Oracle Always Free, `VM.Standard.E2.1.Micro` (AMD x86-64, 1/8 de OCPU, 1 GB; hasta 2) | 0 | Una IP pública por VNIC; 50 Mbps | Oracle reclama instancias ociosas: 7 días con CPU p95, red y (solo en A1) memoria por debajo del 20 %. Quitsie casi siempre está ociosa. Las fuentes no coinciden en si pasar a pago lo evita. Cuentas inactivas 30 días pueden suspenderse. A veces no hay capacidad en la región propia («out of host capacity») | [Always Free](https://docs.oracle.com/en-us/iaas/Content/FreeTier/freetier_topic-Always_Free_Resources.htm) (extracto); [oracle.com/cloud/free](https://www.oracle.com/cloud/free/) (verificado: suspensión a los 30 días) |
| Oracle Always Free, Ampere A1 | 0 | Sí | Bajó de 4 OCPU y 24 GB a 2 OCPU y 12 GB el 15 de junio de 2026, sin aviso previo. Es ARM: `provision.sh` exige x86-64 y el binario y Litestream se construyen para amd64. Mismas reglas de inactividad | [InfoQ, julio de 2026](https://www.infoq.com/news/2026/07/oracle-cloud-free-tier-limits/); [Linuxiac](https://linuxiac.com/oracle-quietly-cuts-free-tier-ampere-a1-resources-in-half/) |
| Hetzner CX23, Alemania o Finlandia | 5,49 € + 0,50 € de IPv4 ≈ 5,99 € (sin IVA) | 0,50 € al mes | Subió de 3,99 € el 15 de junio de 2026. Está en Europa. En EE. UU. el plan más barato listado (CPX11) cuesta 20,49 USD sin IPv4 | [ajuste de precios](https://docs.hetzner.com/general/infrastructure-and-availability/price-adjustment/), [precio de IPv4](https://docs.hetzner.com/general/infrastructure-and-availability/ipv4-pricing/) (extracto) |
| IONOS VPS S+ (EE. UU.; 2 vCore, 2 GB) | 2 USD los 3 primeros meses con contrato de un año; después 5 USD | Una incluida | Precio de entrada temporal | [ionos.com/servers/vps](https://www.ionos.com/servers/vps) (extracto) |
| RackNerd 1 GB; UpCloud Starter 1 GB | 1,83 USD (pago anual); 3,50 USD | Incluida | Solo fuentes secundarias de comparación | [servercalc, 21-10-2026](https://servercalc.com/cheap-vps/); [Virtarix](https://virtarix.com/blog/vps-guides/cheap-cloud-vps/) |

Cualquier cambio de proveedor necesita un ADR nuevo. Además, el asistente y `provision.sh` dependen de `gcloud` e IAP para el acceso de emergencia y la copia de `deploy/`. Los precios de VPS de 2026 ya no están en 1–4 USD salvo promociones o planes anuales.

## 5. Cómo comprobar lo que falta

Desde cualquier máquina con `dig`, antes de decidir:

```bash
dig AAAA api.resend.com +short
dig AAAA <ACCOUNT_ID>.r2.cloudflarestorage.com +short   # el Account ID real de R2
dig AAAA accounts.google.com +short
dig AAAA oauth2.googleapis.com +short
dig AAAA www.googleapis.com +short
dig AAAA deb.debian.org +short
dig AAAA packages.cloud.google.com +short
dig AAAA pkg.cloudflare.com +short
dig AAAA github.com +short   # se espera vacío
```

Si se elige B, este sería el cambio en la etapa 5 del asistente. **Sin probar.** En una red personalizada no existen las reglas `default-allow-*`, así que solo hace falta la de IAP.

```bash
gcloud compute networks create quitsie-net --subnet-mode=custom
gcloud compute networks subnets create quitsie-us-east1 --network=quitsie-net --region=us-east1 \
  --range=10.10.0.0/24 --stack-type=IPV4_IPV6 --ipv6-access-type=EXTERNAL
gcloud compute firewall-rules create allow-iap-ssh --network=quitsie-net --direction=INGRESS \
  --action=ALLOW --rules=tcp:22 --source-ranges=35.235.240.0/20 --target-tags=quitsie
gcloud compute instances create quitsie --zone=us-east1-b --machine-type=e2-micro \
  --image-family=debian-13 --image-project=debian-cloud \
  --boot-disk-size=30GB --boot-disk-type=pd-standard \
  --no-service-account --no-scopes --tags=quitsie \
  --subnet=quitsie-us-east1 --stack-type=IPV4_IPV6 --ipv6-network-tier=PREMIUM --no-address
```

Después, en la VM, `curl -6 -sS -o /dev/null -w '%{http_code}\n' https://api.resend.com/` y lo mismo con el host de R2 confirman la salida real. Si algo falla, `gcloud compute instances add-access-config quitsie --zone=us-east1-b` le da una IPv4 efímera y se vuelve a la opción A.

## 6. Recomendación

Primero, comprobar con `dig AAAA` que `api.resend.com` y el host de R2 de la cuenta tienen IPv6: son los dos datos que no pude verificar y deciden todo. Si los dos responden, la VM dual-stack sin IPv4 externa (B) cumple los 0 USD del ADR 0007. Conserva IAP, el servidor de metadatos y el resto del diseño, y se puede deshacer con un comando. Cuesta trabajo y riesgo: una red propia en el asistente, `TUNNEL_EDGE_IP_VERSION=6`, traer el `.deb` de Litestream desde fuera porque GitHub no tiene IPv6, una configuración que nadie ha probado, y que cualquier servicio futuro solo IPv4 falle. Si alguno no tiene IPv6, o no se quiere ese trabajo, lo más barato y simple es pagar los 3,65 USD al mes: Cloud NAT cuesta más, Oracle gratis puede reclamar una VM ociosa y cambia sus condiciones sin aviso, y los VPS cuestan lo mismo o más pasada la promoción. En cualquier caso, el ADR 0007 y la tabla de costos de `docs/arquitectura.md` deben decir el costo real.
