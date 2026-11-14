BIN := bin/gfi

.PHONY: build test lint verify run preflight design-check config-example clean

build:
	@mkdir -p bin
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o $(BIN) ./cmd/gfi

test:
	go test ./...

lint:
	go vet ./...
	golangci-lint run

verify: build test lint design-check

run:
	go run ./cmd/gfi

preflight:
	./scripts/preflight.sh

# El ejemplo se genera desde el código: un test falla si el archivo commiteado difiere.
config-example:
	go run ./cmd/gfi config example > config.example.json

# El CSS y el JS del diseño son los mismos que sirve el binario: si difieren,
# el build falla. Evita el drift entre design/ y internal/web/static/.
design-check:
	@test -d design || { echo "design/ no existe todavía"; exit 0; }; \
	fail=0; \
	for f in app.css app.js; do \
	  if [ -f "design/$$f" ] && ! cmp -s "design/$$f" "internal/web/static/$$f"; then \
	    echo "design/$$f y internal/web/static/$$f difieren"; fail=1; \
	  fi; \
	done; \
	exit $$fail

clean:
	rm -rf bin
