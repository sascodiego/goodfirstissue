package web

import (
	"html"
	"html/template"
	"regexp"
	"strconv"
	"strings"
)

// renderMarkdown convierte el cuerpo de una issue en HTML seguro.
//
// El cuerpo es contenido hostil (I3: evidencia, nunca instrucción), así que el
// contrato es de una sola dirección y no se puede ablandar:
//
//  1. Se escapa TODO primero, con html.EscapeString.
//  2. Después se aplica un subconjunto sobre ese texto ya inerte: párrafos,
//     listas, bloques de código, código en línea, énfasis y enlaces http/https.
//  3. Nada de HTML crudo del autor. Nada de imágenes remotas: la app no dispara
//     pedidos a un servidor ajeno por culpa del texto de una issue.
//
// Escapar primero es lo que vuelve imposible el XSS: en el paso 2 lo único que
// se agrega son etiquetas que este archivo escribe. El tipo de retorno es
// template.HTML porque ya está escapado y porque html/template, si no, volvería
// a escapar las etiquetas propias.
func renderMarkdown(src string) template.HTML {
	src = strings.ReplaceAll(src, "\r\n", "\n")
	src = strings.ReplaceAll(src, "\r", "\n")
	src = stripControls(src)

	// Paso 1, el único que toca lo que escribió el autor.
	return template.HTML(renderBlocks(html.EscapeString(src)))
}

// stripControls borra los controles C0 y el DEL: no se ven, no aportan texto y
// son el único canal por el que el autor podría escribir una marca del render.
//
// html.EscapeString escapa < > & ' " y nada más: un NUL del cuerpo atraviesa el
// escapado intacto. Sin este filtro, el autor podría hacerse pasar por una marca
// y reescribir el marcado que arma este archivo, que es exactamente lo que el
// contrato de renderMarkdown promete que no puede pasar. El salto y el tab se
// conservan: son contenido, no control.
func stripControls(s string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case '\n', '\t':
			return r
		}
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, s)
}

// codeFence delimita un bloque de código. Se compara sobre el texto ya escapado:
// las comillas invertidas no cambian al escapar.
const codeFence = "```"

var (
	inlineCodePattern = regexp.MustCompile("`([^`\n]+)`")
	linkPattern       = regexp.MustCompile(`\[([^\]\n]*)\]\(([^)\s]+)\)`)
	boldPattern       = regexp.MustCompile(`\*\*([^*\n]+)\*\*`)
	italicPattern     = regexp.MustCompile(`\*([^*\n]+)\*`)
	itemPattern       = regexp.MustCompile(`^(?:[-*+]|\d+[.)])\s+(.*)$`)
	orderedPattern    = regexp.MustCompile(`^\d+[.)]\s+`)
)

// renderBlocks recorre el texto escapado línea por línea y arma los bloques.
// Los bloques de código llevan su contenido tal cual (ya escapado); el resto
// pasa por el render en línea.
func renderBlocks(escaped string) string {
	lines := strings.Split(escaped, "\n")
	var out strings.Builder

	for i := 0; i < len(lines); {
		trimmed := strings.TrimSpace(lines[i])

		switch {
		case trimmed == "":
			i++

		case strings.HasPrefix(trimmed, codeFence):
			i++
			var code []string
			for i < len(lines) && !strings.HasPrefix(strings.TrimSpace(lines[i]), codeFence) {
				code = append(code, lines[i])
				i++
			}
			i++ // la cerca de cierre; si no vino, el bloque llega al final
			out.WriteString("<pre><code>" + strings.Join(code, "\n") + "</code></pre>\n")

		case itemPattern.MatchString(trimmed):
			ordered := orderedPattern.MatchString(trimmed)
			tag := "ul"
			if ordered {
				tag = "ol"
			}
			out.WriteString("<" + tag + ">\n")
			for i < len(lines) {
				item := strings.TrimSpace(lines[i])
				if !itemPattern.MatchString(item) || orderedPattern.MatchString(item) != ordered {
					break
				}
				out.WriteString("<li>" + renderInline(itemPattern.FindStringSubmatch(item)[1]) + "</li>\n")
				i++
			}
			out.WriteString("</" + tag + ">\n")

		default:
			// Un párrafo termina en blanco o donde empieza otro tipo de bloque.
			// Las líneas se unen con salto: HTML ya las colapsa, y el salto
			// conserva el texto tal como lo escribió la persona.
			var para []string
			for i < len(lines) {
				line := strings.TrimSpace(lines[i])
				if line == "" || strings.HasPrefix(line, codeFence) || itemPattern.MatchString(line) {
					break
				}
				para = append(para, renderInline(line))
				i++
			}
			out.WriteString("<p>" + strings.Join(para, "\n") + "</p>\n")
		}
	}
	return out.String()
}

// renderInline aplica el subconjunto en línea sobre texto ya escapado.
//
// El código en línea y los enlaces se extraen primero a marcadores: así el
// énfasis no puede entrar adentro de un `código` ni de una URL, que es
// exactamente lo que rompería el orden de aplicación.
func renderInline(escaped string) string {
	var kept []string
	marker := func(htmlText string) string {
		kept = append(kept, htmlText)
		return inlineMarker(len(kept) - 1)
	}

	escaped = inlineCodePattern.ReplaceAllStringFunc(escaped, func(m string) string {
		return marker("<code>" + m[1:len(m)-1] + "</code>")
	})

	escaped = linkPattern.ReplaceAllStringFunc(escaped, func(m string) string {
		parts := linkPattern.FindStringSubmatch(m)
		if !isHTTPURL(parts[2]) {
			// Se descarta el enlace, nunca el texto: el autor ve lo que escribió,
			// esquema incluido, y nada se vuelve clickeable.
			return marker(m)
		}
		return marker(`<a href="` + parts[2] + `" rel="noopener noreferrer">` + parts[1] + "</a>")
	})

	escaped = boldPattern.ReplaceAllString(escaped, "<strong>$1</strong>")
	escaped = italicPattern.ReplaceAllString(escaped, "<em>$1</em>")

	return restoreInline(escaped, kept)
}

// inlineMarker separa las marcas del texto. El NUL es un canal que solo este
// archivo escribe: stripControls ya borró los del autor antes de escapar, así
// que el texto que llega acá no puede contener ni uno.
func inlineMarker(i int) string { return "\x00" + strconv.Itoa(i) + "\x00" }

var markerPattern = regexp.MustCompile("\x00(\\d+)\x00")

// restoreInline repone lo extraído. Repite hasta que no queden marcas: un
// enlace puede contener código en línea, y ese código es otra marca.
func restoreInline(s string, kept []string) string {
	for range len(kept) + 1 {
		if !markerPattern.MatchString(s) {
			break
		}
		s = markerPattern.ReplaceAllStringFunc(s, func(m string) string {
			i, err := strconv.Atoi(m[1 : len(m)-1])
			if err != nil || i < 0 || i >= len(kept) {
				return ""
			}
			return kept[i]
		})
	}
	return s
}

// isHTTPURL es el único filtro de esquema. Se evalúa sobre el texto escapado,
// y el esquema no cambia al escapar, así que la comparación es exacta.
func isHTTPURL(href string) bool {
	lower := strings.ToLower(href)
	return strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://")
}
