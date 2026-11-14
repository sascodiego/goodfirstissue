# design/ — maquetas de la interfaz

Acá se diseña la UI **antes** de que exista el template. HTML, CSS y JS puros: sin
build, sin frameworks, sin `node_modules`. Un archivo se abre con doble clic y se ve.

## Regla contra el drift

El riesgo de maquetar y después "portar" es tener dos copias que se separan. Se
evita no portando todo:

| Pieza | Qué pasa |
|---|---|
| **`app.css`, `app.js`** | **no se portan.** Son los mismos archivos que sirve el binario, en `internal/web/static/`. `make design-check` falla el build si difieren. |
| **HTML** | sí se porta: se parte en `layout` + parciales y lo repetido pasa a `{{range}}`. Es la única pieza que cambia de forma. |
| **`fixtures.json`** | datos falsos al lado de la maqueta, para que se vea real sin servidor. |

Cuando una maqueta ya está implementada, **no se edita más**: si el diseño cambia, se
cambia en `internal/web/` y se actualiza la maqueta o se borra. Una maqueta vieja que
nadie mira es peor que ninguna.

## Convención

```
design/<slug>/NN-nombre.html     maqueta, numerada en orden de flujo
design/<slug>/app.css            hoja, idéntica a internal/web/static/app.css
design/<slug>/app.js             comportamiento, idéntico a internal/web/static/app.js
design/<slug>/fixtures.json      datos de mentira
```

Slugs del proyecto: `repo` (embudo), `issue` (evidencia), `veredicto` (encaje).

## Flujo de trabajo

1. Se escribe la maqueta en `design/`.
2. Se mira en el navegador, se itera ahí. Nada de recompilar Go para mover un margen.
3. Cuando convence, `app.css` y `app.js` se copian a `internal/web/static/` —o se mueven,
   si es la primera vez— y `make design-check` confirma que no hay deriva.
4. El HTML se convierte en `internal/web/templates/layout.html` + parciales.
5. `make verify` (build + test + lint + design-check) antes de commitear.

## Lo que **no** se hace

- Meter lógica de negocio en las maquetas: los datos salen de `fixtures.json` y ya.
- Copiar CSS de terceros sin adaptarlo a los tokens de `:root`.
- Fuentes web: el binario tiene que servir la app sin red. Solo familias del sistema.
