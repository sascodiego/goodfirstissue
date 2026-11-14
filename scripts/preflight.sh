#!/usr/bin/env bash
# Preflight: verifica los prerequisitos de PROYECTO.md §17 antes de escribir código.
# Uso: ./scripts/preflight.sh
set -uo pipefail

fail=0
warn=0
rows=()

ok()   { rows+=("PASS|$1|$2"); }
bad()  { rows+=("FAIL|$1|$2"); fail=$((fail + 1)); }
note() { rows+=("WARN|$1|$2"); warn=$((warn + 1)); }

need_ver() { # need_ver <bin> <min> <etiqueta> <cómo obtener la versión>
  local bin=$1 min=$2 label=$3 getter=$4 have
  command -v "$bin" >/dev/null 2>&1 || { bad "$label" "no está en PATH"; return; }
  have=$(eval "$getter" 2>/dev/null | head -1)
  [[ -z $have ]] && { bad "$label" "no pude leer la versión"; return; }
  if [[ $(printf '%s\n%s\n' "$min" "$have" | sort -V | head -1) == "$min" ]]; then
    ok "$label" "$have"
  else
    bad "$label" "$have (mínimo $min)"
  fi
}

# --- Go -------------------------------------------------------------------
need_ver go 1.22 "Go >= 1.22" 'go env GOVERSION | sed "s/^go//"'

# --- codegraph (dependencia de runtime, con fallback textual) -------------
need_ver codegraph 1.6 "codegraph >= 1.6" 'codegraph -V'

# --- git ------------------------------------------------------------------
need_ver git 2.30 "git >= 2.30" 'git --version | awk "{print \$3}"'

# --- gh (solo para construir el corpus; no se usa en runtime) -------------
if command -v gh >/dev/null 2>&1; then
  if gh auth status >/dev/null 2>&1; then ok "gh autenticado" "$(gh --version | head -1 | awk '{print $3}')"
  else note "gh autenticado" "instalado pero sin sesión: solo afecta la construcción del corpus"; fi
else
  note "gh autenticado" "no instalado: solo afecta la construcción del corpus"
fi

# --- jq (fixtures y depuración) -------------------------------------------
command -v jq >/dev/null 2>&1 && ok "jq" "$(jq --version)" || note "jq" "no instalado (opcional)"

# --- clave de NaN: BLOQUEANTE --------------------------------------------
if [[ -n ${NAN_KEY:-} ]]; then
  ok "NAN_KEY" "definida (${#NAN_KEY} caracteres)"
else
  bad "NAN_KEY" "sin definir: sin clave no se puede llamar a Clef ni al modelo de chat"
fi

# --- espacio en disco -----------------------------------------------------
free_gb=$(df -Pk . | awk 'NR==2 {printf "%d", $4/1048576}')
if (( free_gb >= 2 )); then ok "disco libre" "${free_gb} GB (mínimo 2)"; else bad "disco libre" "${free_gb} GB (mínimo 2)"; fi

# --- red hacia la API de NaN ---------------------------------------------
code=$(curl -s -o /dev/null -w '%{http_code}' --max-time 8 https://api.nan.builders/v1/models 2>/dev/null || echo 000)
case $code in
  000)     bad  "api.nan.builders" "sin respuesta" ;;
  401|200) ok   "api.nan.builders" "alcanzable (HTTP $code)" ;;
  *)       note "api.nan.builders" "HTTP $code" ;;
esac

# --- informe --------------------------------------------------------------
printf '%s\n' "${rows[@]}" | column -t -s'|' 2>/dev/null || printf '%s\n' "${rows[@]}" | tr '|' '\t'

echo
if (( fail > 0 )); then
  echo "RESULTADO: $fail bloqueante(s), $warn aviso(s). No se puede empezar."
  exit 1
fi
echo "RESULTADO: prerequisitos completos, $warn aviso(s)."
