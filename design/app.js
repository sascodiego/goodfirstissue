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
  var rows = Array.prototype.slice.call(document.querySelectorAll(".row-sel"));
  if (!all || !rows.length) return;

  // El check del encabezado selecciona lo que se está viendo, no el corpus:
  // el filtro define el universo de la acción.
  function sync() {
    var picked = rows.filter(function (r) {
      return r.checked;
    });

    if (count) count.textContent = picked.length;
    all.checked = picked.length > 0 && picked.length === rows.length;
    all.indeterminate = picked.length > 0 && picked.length < rows.length;

    rows.forEach(function (r) {
      var tr = r.closest("tr");
      if (tr) tr.classList.toggle("is-selected", r.checked);
    });

    // El botón de analizar sigue deshabilitado a propósito: el endpoint no
    // existe todavía. Habilitarlo sería ofrecer algo que no va a pasar.
  }

  all.addEventListener("change", function () {
    rows.forEach(function (r) {
      r.checked = all.checked;
    });
    sync();
  });

  rows.forEach(function (r) {
    r.addEventListener("change", sync);
  });

  sync();

  // Un fetch real tarda segundos y el POST espera: el formulario se apaga al enviarse y
  // dice qué está pasando. Sin este archivo el envío funciona igual, solo que mudo.
  Array.prototype.forEach.call(
    document.querySelectorAll("form[data-busy]"),
    function (form) {
      form.addEventListener("submit", function () {
        var button = form.querySelector("button[type=submit]");
        if (button) {
          button.disabled = true;
          button.setAttribute("aria-busy", "true");
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
