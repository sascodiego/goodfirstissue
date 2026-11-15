package web

import (
	"fmt"
	"html/template"
	"io/fs"
)

// screenDashboard es la pantalla del corpus. Cada pantalla es un archivo en
// templates/ y una ruta en routes.go: agregar una no obliga a editar el archivo
// de la de al lado, que es todo el punto de esta partición.
const screenDashboard = "dashboard"

// screenIssue es la pantalla de una issue del corpus.
const screenIssue = "issue"

// screenProfile es la pantalla del CV y el perfil que salió de él.
const screenProfile = "profile"

// screenNames son las pantallas que el binario sabe renderizar.
var screenNames = []string{screenDashboard, screenIssue, screenProfile}

// partialPattern son los bloques que cualquier pantalla puede enganchar.
const partialPattern = "templates/partials/*.html"

// parseScreens arma un set de plantillas por pantalla: el layout y los
// parciales se parsean una vez, y cada pantalla clona ese set y agrega su
// archivo.
//
// El espacio de nombres de html/template es plano: parsear todas las pantallas
// juntas hace que la segunda que defina "content" reemplace el cuerpo de la
// primera, sin error y sin que falle ningún test. Un set por pantalla lo vuelve
// imposible, que es para lo que existe Template.Clone.
func parseScreens(fsys fs.FS, names []string) (map[string]*template.Template, error) {
	base, err := template.ParseFS(fsys, "templates/layout.html", partialPattern)
	if err != nil {
		return nil, err
	}
	sets := make(map[string]*template.Template, len(names))
	for _, name := range names {
		tmpl, err := base.Clone()
		if err != nil {
			return nil, err
		}
		if _, err := tmpl.ParseFS(fsys, "templates/"+name+".html"); err != nil {
			return nil, err
		}
		// Sin cuerpo no hay pantalla. Fallar acá es fallar al arrancar, no en el
		// primer request con la cabecera ya escrita.
		if tmpl.Lookup("content") == nil {
			return nil, fmt.Errorf("la pantalla %q no define \"content\"", name)
		}
		sets[name] = tmpl
	}
	return sets, nil
}
