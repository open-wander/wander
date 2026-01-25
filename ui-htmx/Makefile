.PHONY: help install-tailwind build-css watch-css clean ui-generate ui-watch

TAILWIND_BIN := ./bin/tailwindcss
INPUT_CSS := static/css/input.css
OUTPUT_CSS := static/css/output.css
TEMPL_BIN := templ

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-20s %s\n", $$1, $$2}'

install-tailwind: ## Install Tailwind CSS CLI
	@echo "Installing Tailwind CSS CLI..."
	@./install-tailwind.sh

build-css: ## Build CSS for production (minified)
	@echo "Building production CSS..."
	@$(TAILWIND_BIN) -i $(INPUT_CSS) -o $(OUTPUT_CSS) --minify
	@echo "✓ CSS built successfully: $(OUTPUT_CSS)"

build-css-dev: ## Build CSS for development (not minified)
	@echo "Building development CSS..."
	@$(TAILWIND_BIN) -i $(INPUT_CSS) -o $(OUTPUT_CSS)
	@echo "✓ CSS built successfully: $(OUTPUT_CSS)"

watch-css: ## Watch CSS files and rebuild on changes
	@echo "Watching CSS files for changes..."
	@$(TAILWIND_BIN) -i $(INPUT_CSS) -o $(OUTPUT_CSS) --watch

clean: ## Remove generated CSS
	@echo "Cleaning generated files..."
	@rm -f $(OUTPUT_CSS)
	@echo "✓ Cleaned"

verify-config: ## Verify Tailwind config is valid
	@echo "Verifying Tailwind configuration..."
	@$(TAILWIND_BIN) -i $(INPUT_CSS) -o /dev/null --config tailwind.config.js
	@echo "✓ Configuration is valid"

check-deps: ## Check if Tailwind plugins are installed
	@echo "Checking Tailwind plugin dependencies..."
	@if [ ! -d "node_modules/@tailwindcss/forms" ] || [ ! -d "node_modules/@tailwindcss/typography" ]; then \
		echo "⚠ Tailwind plugins not installed. Run: npm install @tailwindcss/forms @tailwindcss/typography"; \
		exit 1; \
	else \
		echo "✓ All plugin dependencies installed"; \
	fi

ui-generate: ## Generate Go code from Templ templates
	@echo "Generating Go code from Templ templates..."
	@$(TEMPL_BIN) generate
	@echo "✓ Templates generated successfully"

ui-watch: ## Watch Templ templates and regenerate on changes
	@echo "Watching Templ templates for changes..."
	@$(TEMPL_BIN) generate --watch

setup: install-tailwind check-deps build-css-dev ## Complete setup: install CLI, check deps, build CSS
	@echo ""
	@echo "✓ Setup complete! You can now run 'make watch-css' for development."
