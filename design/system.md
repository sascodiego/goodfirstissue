# Sistema de diseño — GoodFirstIssue

Este documento **reemplaza** al sistema anterior. Aquel se había sincronizado
desde `internal/web/static/app.css`, que era la hoja del esqueleto «hola mundo»:
siete colores y tres saltos tipográficos pensados para una pantalla de bienvenida,
no para un dashboard de datos. Se descarta entero.

Acá hay **dos familias propuestas**, no una. Se elige una y recién entonces sus
tokens pasan a `internal/web/static/app.css`. Hasta ese momento son propuesta:
ningún valor de este documento está en el código.

Reglas que las dos comparten:

- **Sin decoración que no informe.** Nada de degradados de texto, glassmorphism,
  halos, sombras suaves por todas partes, ni violeta-azul genérico de plantilla.
- **La jerarquía se hace con tamaño, peso y hairlines**, no con color ni sombra.
- **Números tabulares** (`font-variant-numeric: tabular-nums`) en toda cifra:
  una columna de conteos que baila es un dashboard que no se puede leer.
- **Datos en monoespaciada** (números de issue, SHAs, etiquetas, fechas).
  Prosa en sans. Esa mezcla es lo que separa un instrumento de una tarjeta.
- **El color carga significado de estado**, nunca adorno. Un estado siempre
  lleva además una palabra: nada se comunica solo por color.
- **Foco visible real**: `outline: 2px solid var(--accent); outline-offset: 2px`.
- **`prefers-reduced-motion`** respetado; el movimiento por defecto es nulo:
  transiciones de 120–160 ms en bordes y fondos, nada que se mueva solo.
- Sin fuentes web. El binario sirve la app sin red.

---

## A · Instrumento

Para quien va a mirar esto durante horas. Casi negro, hairlines de 1 px, densidad
alta, un solo acento que aparece poco y por eso pesa. La referencia mental no es
un SaaS: es un instrumento de medición.

### Color

| Token | Valor | Uso |
|---|---|---|
| `--bg` | `#0a0b0d` | fondo de la aplicación, plano |
| `--surface` | `#101216` | paneles: rail, tabla, encabezado |
| `--surface-2` | `#16191e` | fila en hover, campo en foco |
| `--line` | `#1f2329` | hairline base |
| `--line-2` | `#2a2f37` | separador fuerte, borde de control |
| `--fg` | `#e7e9ec` | texto principal |
| `--fg-2` | `#99a1ac` | secundario: metadatos, encabezados de columna |
| `--fg-3` | `#656d78` | terciario: deshabilitado, marcas de tiempo lejanas |
| `--accent` | `#d8f26b` | acción primaria, anillo de foco, estado vivo |
| `--accent-ink` | `#0a0b0d` | texto sobre `--accent` |
| `--state-taken` | `#656d78` | ya tiene PR abierta |
| `--state-noise` | `#a78bfa` | no describe trabajo |
| `--state-blocked` | `#e3b341` | condicionada: falta el sí del mantenedor |
| `--state-live` | `var(--accent)` | candidata viva |

### Tipografía

| Rol | Tamaño | Detalle |
|---|---|---|
| Display | `clamp(1.75rem, 1.4rem + 1vw, 2.25rem)` | `letter-spacing: -0.03em`, peso 570 |
| Título de sección | `0.75rem` | mayúsculas, `letter-spacing: 0.1em`, `--fg-2`, peso 600 |
| Cuerpo / fila | `0.8125rem` | `line-height: 1.45` |
| Dato | `0.75rem` | mono, `tabular-nums`, `--fg-2` |

Familias: `ui-sans-serif, system-ui, -apple-system, "Segoe UI", Roboto, sans-serif`
y `ui-monospace, "SF Mono", Menlo, monospace`.

### Forma y densidad

| Token | Valor |
|---|---|
| `--r-sm` | `3px` (chip, campo) |
| `--r-md` | `6px` (panel) |
| `--r-pill` | `999px` (solo el punto de estado) |
| `--row-h` | `2.75rem` (44 px) |
| sombra | ninguna, en ningún nivel |

---

## B · Editorial

Para leer un veredicto y entenderlo. Cálido, con serif en los títulos, aire
generoso y una terracota que se usa con cuentagotas. Se lee como un informe
bien impreso, no como un panel de control.

### Color

| Token | Valor | Uso |
|---|---|---|
| `--bg` | `#12100e` | fondo cálido, casi negro |
| `--surface` | `#191613` | paneles |
| `--surface-2` | `#201b17` | hover, foco |
| `--line` | `#2a2420` | hairline |
| `--line-2` | `#3a322c` | separador fuerte |
| `--fg` | `#f2ece4` | texto principal, blanco cálido (nunca `#fff`) |
| `--fg-2` | `#aea396` | secundario |
| `--fg-3` | `#7b7167` | terciario |
| `--accent` | `#d98b5f` | acción primaria, foco, estado vivo |
| `--accent-ink` | `#12100e` | texto sobre `--accent` |
| `--state-taken` | `#7b7167` | ya tiene PR abierta |
| `--state-noise` | `#b79be0` | no describe trabajo |
| `--state-blocked` | `#d9a441` | condicionada |
| `--state-live` | `var(--accent)` | candidata viva |

### Tipografía

| Rol | Tamaño | Detalle |
|---|---|---|
| Display | `clamp(2rem, 1.6rem + 1.4vw, 2.75rem)` | **serif**, `letter-spacing: -0.015em` |
| Título de sección | `0.7rem` | mayúsculas, `letter-spacing: 0.16em`, `--fg-3` |
| Cuerpo / fila | `0.875rem` | `line-height: 1.55` |
| Dato | `0.78rem` | mono, `tabular-nums` |

Familias: display `ui-serif, Georgia, "Iowan Old Style", "Palatino Linotype", serif`;
texto `ui-sans-serif, system-ui, -apple-system, "Segoe UI", Roboto, sans-serif`;
datos `ui-monospace, "SF Mono", Menlo, monospace`.

El serif es la decisión de riesgo de esta familia: es lo que la aleja de cualquier
plantilla y lo que la vuelve legible como documento. Si se cae, se cae por eso.

### Forma y densidad

| Token | Valor |
|---|---|
| `--r-sm` | `2px` |
| `--r-md` | `4px` |
| `--r-pill` | `999px` (solo el punto de estado) |
| `--row-h` | `3rem` (48 px) |
| sombra | ninguna |

---

## Lo que falta en las dos, y se propone igual

| Token | A · Instrumento | B · Editorial | Significado |
|---|---|---|---|
| `--danger` | `#f0716a` | `#d96c5f` | fallo de red, `undecided` |
| `--focus` | `--accent` | `--accent` | anillo de foco |

## Estados del producto

Cuatro desenlaces, iguales en las dos familias (`--state-*` de arriba):
`taken`, `noise`, `blocked`, `live`.

No existe un quinto estado para «descartada». **I10** prohíbe descartar por
evidencia insuficiente: ese caso aparece como `uncertain` dentro de la etapa
viva, con su probabilidad. Las duplicadas tampoco se descartan (**I12**). En el
MVP la unidad de evaluación es la issue sola: agrupar quedó fuera por tiempo
(`LIMITES.md` §1). Cuando exista, corre después del triaje y la evaluación se
hace sobre el grupo.

## Cómo se elige

1. Se mira `design/system.html`, que pone las dos familias lado a lado con los
   mismos componentes.
2. Se mira `design/dashboard/01-instrumento.html` y
   `design/dashboard/02-editorial.html`, que son la misma pantalla en cada familia.
3. La elegida se lleva a `internal/web/static/app.css`, se sincroniza este
   documento a lo que quedó implementado, y este archivo deja de decir «propuesta».
