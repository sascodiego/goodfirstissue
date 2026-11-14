package web

import (
	"strings"
	"testing"
)

// El cuerpo de una issue es contenido hostil (I3): evidencia, nunca instrucción.
// Estos tests fijan el contrato del render: se escapa todo primero y después se
// aplica un subconjunto. Si el orden se invierte, el primero de acá falla.

// Un cuerpo con etiquetas no ejecuta nada: sale escapado, visible, inerte.
func TestMarkdownEscapesAuthorHTML(t *testing.T) {
	got := string(renderMarkdown("Fix this:\n\n<script>alert(1)</script>\n<img src=x onerror=\"alert(2)\">\n"))

	for _, forbidden := range []string{"<script", "<img", "onerror=\"alert"} {
		if strings.Contains(got, forbidden) {
			t.Errorf("el render dejó pasar %q:\n%s", forbidden, got)
		}
	}
	for _, want := range []string{
		"&lt;script&gt;alert(1)&lt;/script&gt;",
		"&lt;img src=x onerror=&#34;alert(2)&#34;&gt;",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("falta %q (el cuerpo tiene que verse, escapado):\n%s", want, got)
		}
	}
}

// Un enlace del autor solo se convierte en ancla si el esquema es http o https.
// El texto sigue visible: se descarta el enlace, no lo que la persona escribió.
func TestMarkdownLinksOnlyHTTPAndHTTPS(t *testing.T) {
	got := string(renderMarkdown("[click](javascript:alert(1))"))

	if strings.Contains(got, "<a ") {
		t.Errorf("un enlace javascript: se convirtió en ancla:\n%s", got)
	}
	for _, want := range []string{"click", "javascript:alert(1)"} {
		if !strings.Contains(got, want) {
			t.Errorf("falta %q: se descartó el texto, no el enlace:\n%s", want, got)
		}
	}

	got = string(renderMarkdown("[ok](https://example.com/a?b=1&c=2)"))
	want := `<a href="https://example.com/a?b=1&amp;c=2" rel="noopener noreferrer">ok</a>`
	if !strings.Contains(got, want) {
		t.Errorf("el enlace https no salió como %q:\n%s", want, got)
	}
}

// Nada de imágenes remotas: la app no dispara pedidos a un servidor ajeno por
// culpa del texto de una issue.
func TestMarkdownDropsRemoteImages(t *testing.T) {
	got := string(renderMarkdown("![captura](https://example.com/x.png)"))

	if strings.Contains(got, "<img") {
		t.Errorf("el render armó un <img>:\n%s", got)
	}
	if !strings.Contains(got, "captura") {
		t.Errorf("se perdió el texto de la imagen:\n%s", got)
	}
}

// El subconjunto que sí se renderiza: listas, bloque de código, código en línea
// y énfasis. Un título markdown no está en el subconjunto y queda literal.
func TestMarkdownRendersTheSubset(t *testing.T) {
	in := strings.Join([]string{
		"# paso a paso",
		"",
		"- uno",
		"- dos",
		"",
		"1. tres",
		"",
		"```",
		"<script>x</script>",
		"```",
		"",
		"Usá `make test` y **no** lo rompas.",
	}, "\n")

	got := string(renderMarkdown(in))

	for _, want := range []string{
		"# paso a paso", // fuera del subconjunto: literal, nunca <h1>
		"<li>uno</li>",
		"<li>dos</li>",
		"<li>tres</li>",
		"<pre><code>&lt;script&gt;x&lt;/script&gt;</code></pre>", // el código también se escapa
		"<code>make test</code>",
		"<strong>no</strong>",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("falta %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "<h1") {
		t.Errorf("un título markdown se convirtió en <h1>:\n%s", got)
	}
	if !strings.Contains(got, "<ul>") || !strings.Contains(got, "<ol>") {
		t.Errorf("no se distinguieron la lista suelta de la numerada:\n%s", got)
	}
}

// El código en línea gana sobre el énfasis: lo que está entre backticks se
// muestra tal cual, aunque adentro haya marcas.
func TestMarkdownInlineCodeIsNotEmphasis(t *testing.T) {
	got := string(renderMarkdown("Escribí `**no**` en el commit."))

	if !strings.Contains(got, "<code>**no**</code>") {
		t.Errorf("el código en línea no quedó literal:\n%s", got)
	}
	if strings.Contains(got, "<strong>") {
		t.Errorf("se aplicó énfasis adentro del código en línea:\n%s", got)
	}
}
