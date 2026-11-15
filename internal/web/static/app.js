// GoodFirstIssue — comportamiento del cliente. Sin dependencias, sin build.
//
// Todo lo de acá mejora algo que ya funciona sin JS: los filtros se envían con
// el formulario y la tabla se arma en el servidor. Nada de esto es requisito
// para ver la pantalla.
(function () {
  "use strict";

  // Los desplegables de filtro se envían al cambiar. Sin JS queda el botón
  // «Filtrar» del <noscript>, así que el filtro nunca depende de este archivo.
  Array.prototype.forEach.call(
    document.querySelectorAll("[data-autosubmit]"),
    function (el) {
      el.addEventListener("change", function () {
        if (el.form) el.form.submit();
      });
    }
  );

  var all = document.getElementById("sel-all");
  var count = document.getElementById("sel-count");
  var button = document.getElementById("analizar-btn");
  var tbody = document.getElementById("filas");
  var rows = [];

  function bindRows() {
    rows = Array.prototype.slice.call(document.querySelectorAll(".row-sel"));
    rows.forEach(function (r) {
      r.addEventListener("change", sync);
    });
  }

  function pickedNumbers() {
    return rows
      .filter(function (r) {
        return r.checked;
      })
      .map(function (r) {
        return r.value;
      });
  }

  // El check del encabezado selecciona lo que se está viendo, no el corpus:
  // el filtro define el universo de la acción.
  function sync() {
    var picked = pickedNumbers();

    if (count) count.textContent = picked.length;
    if (all) {
      all.checked = picked.length > 0 && picked.length === rows.length;
      all.indeterminate = picked.length > 0 && picked.length < rows.length;
    }
    // El botón se habilita solo con algo seleccionado: apagado sin selección es
    // una instrucción, apagado con selección era el defecto que esto arregla.
    if (button) button.disabled = picked.length === 0;

    rows.forEach(function (r) {
      var tr = r.closest("tr");
      if (tr) tr.classList.toggle("is-selected", r.checked);
    });

    schedule();
  }

  if (all) {
    all.addEventListener("change", function () {
      rows.forEach(function (r) {
        r.checked = all.checked;
      });
      sync();
    });
  }

  bindRows();

  // La cola vive en el servidor, así que el navegador no la monitorea: pide la tabla otra
  // vez mientras algo esté en curso y cambia el <tbody>. Sin JS, el <noscript> del layout
  // vuelve a pedir la página cada cinco segundos y se ve lo mismo, con un parpadeo.
  var timer = null;

  function schedule() {
    if (timer !== null || !tbody) return;
    if (!tbody.querySelector("[data-live]")) return;
    timer = window.setTimeout(refresh, 5000);
  }

  function refresh() {
    timer = null;
    var picked = pickedNumbers();
    fetch("/explore/rows" + window.location.search, {
      headers: { Accept: "text/html" },
    })
      .then(function (res) {
        return res.ok ? res.text() : null;
      })
      .then(function (html) {
        if (html === null || !tbody) return;
        tbody.innerHTML = html;
        bindRows();
        // La selección es del usuario, no del servidor: se repone después de cambiar filas.
        picked.forEach(function (value) {
          var el = tbody.querySelector('.row-sel[value="' + value + '"]');
          if (el) el.checked = true;
        });
        sync();
      })
      .catch(function () {
        // Un corte de red no rompe la página: se reintenta en el próximo ciclo.
        schedule();
      });
  }

  sync();

  // Un POST tarda segundos y la página espera: el formulario se apaga al enviarse y dice
  // qué está pasando. Sin este archivo el envío funciona igual, solo que mudo.
  Array.prototype.forEach.call(
    document.querySelectorAll("form[data-busy]"),
    function (form) {
      form.addEventListener("submit", function () {
        var submit = form.querySelector("button[type=submit]");
        if (submit) {
          submit.disabled = true;
          submit.setAttribute("aria-busy", "true");
        }
        Array.prototype.forEach.call(
          form.querySelectorAll(".busy"),
          function (note) {
            note.hidden = false;
          }
        );
      });
    }
  );
})();
