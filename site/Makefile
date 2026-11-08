.PHONY: all build generate css render check test serve clean

TAILWIND := ../ui/node_modules/.bin/tailwindcss

all: build

build: generate css render

generate:
	@templ generate -path templates

css:
	@test -x $(TAILWIND) || (echo "Tailwind is missing; run make -C ../ui install-tailwind"; exit 1)
	@$(TAILWIND) --config tailwind.config.js -i static/css/input.css -o static/css/output.css --minify

render:
	@go run ./cmd/site-build

test: generate
	@go test ./cmd/site-build ./templates

check: build test

serve: build
	@echo "Wander site: http://127.0.0.1:8080"
	@cd dist && python3 -m http.server 8080 --bind 127.0.0.1

clean:
	@rm -rf dist
	@rm -f static/css/output.css templates/*_templ.go
