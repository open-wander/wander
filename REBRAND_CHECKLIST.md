# Wander Rebrand Checklist

This document tracks all items that need to be changed as part of the rebrand from HashiCorp Nomad to open-wander/wander.
Domain openwander.org
Consider Docusoaurus for document handling

## Status Legend
- [ ] Not started
- [x] Completed
- [~] Partially complete

---

## 1. Branding Assets

### Icons and Logos
- [ ] `ui/public/images/icons/nomad-logo-n.svg` - Main UI logo
- [ ] `ui/public/favicon.ico` - Browser tab icon
- [ ] `website/public/img/logo-hashicorp.svg` - Full HashiCorp logo
- [ ] `website/public/img/logo-text.svg` - Nomad text logo
- [ ] `website/public/img/favicons/*` - All favicon sizes
- [ ] `website/public/img/og-image.png` - Social media preview image

### UI Icon System
- [ ] Review `ui/app/styles/core/icon.scss` for hardcoded references
- [ ] Review `ui/stories/theme/icons.stories.js` for branding examples

---

## 2. Code References

### Module and Import Paths
- [x] Go module name in `go.mod` (changed to `github.com/open-wander/wander`)
- [~] Import statements across codebase (partially done in #2)
- [ ] Verify all `github.com/hashicorp/nomad` -> `github.com/open-wander/wander`

### Binary and Command Names
- [ ] `GNUmakefile` - Binary name references (`nomad` -> `wander`)
- [ ] `main.go` - Application name
- [ ] All Makefile targets referencing `nomad` binary
- [ ] Command help text and descriptions in `command/` directory

### Configuration and Defaults
- [ ] Default config file names (e.g., `nomad.hcl` -> `wander.hcl`)
- [ ] Environment variable prefixes (`NOMAD_*` -> `WANDER_*`)
- [ ] Default data directories and paths
- [ ] Service/daemon names in systemd/init scripts
- [ ] Agent names and identifiers

---

## 3. Documentation

### Root Level Docs
- [ ] `README.md` - Update all Nomad/HashiCorp references
  - Project name and description
  - Links to nomadproject.io
  - HashiCorp branding mentions
  - Discussion forum links
  - Tutorial/learning links
- [ ] `CHANGELOG.md` - Consider migration notes
- [ ] `CHANGELOG-unsupported.md` - Header updates

### Contributing Documentation
- [ ] `contributing/README.md` - Update project references
- [ ] All files in `contributing/` directory
- [ ] Code of conduct (if exists)
- [ ] Security policy (if exists)

### UI Documentation
- [ ] `ui/README.md`
- [ ] `ui/DEVELOPMENT_MODE.md`
- [ ] UI component documentation

### Website Content
- [ ] `website/` - All content files
- [ ] Documentation pages in `website/content/docs/`
- [ ] Tutorial content
- [ ] API documentation
- [ ] Blog posts (if any)

---

## 4. GitHub and Repository

### GitHub Configuration
- [ ] Repository description
- [ ] Repository topics/tags
- [ ] Issue templates in `.github/ISSUE_TEMPLATE/`
- [ ] Pull request templates
- [ ] Contributing guidelines
- [ ] GitHub Actions workflow names and descriptions

### Issue Templates
- [ ] Bug report template - Update links and email addresses
- [ ] Feature request template
- [ ] Other templates in `.github/`

### GitHub Actions
- [~] Workflow files (partially done in #1, #3)
- [ ] Action names and descriptions
- [ ] Artifact names
- [ ] Release naming

---

## 5. URLs and External Links

### Update All References To:
- [ ] `nomadproject.io` domain
- [ ] `developer.hashicorp.com/nomad`
- [ ] `learn.hashicorp.com/nomad`
- [ ] `discuss.hashicorp.com/c/nomad`
- [ ] HashiCorp documentation sites

### Email Addresses
- [ ] `nomad-oss-debug@hashicorp.com` (in issue templates)
- [ ] Any support/contact emails

---

## 6. UI and Frontend

### UI Text and Strings
- [ ] Application title in `ui/app/index.html`
- [ ] Page titles and meta tags
- [ ] Error messages mentioning "Nomad"
- [ ] Help text and tooltips
- [ ] Navigation labels

### UI Configuration
- [ ] `ui/config/environment.js`
- [ ] `ui/package.json` - Package name and description
- [ ] Build configuration in `ui/ember-cli-build.js`

---

## 7. Test Data and Fixtures

### Test Files
- [ ] Test fixture data mentioning Nomad/HashiCorp
- [ ] Mock data in `ui/mirage/`
- [ ] Test certificates (already updated in #5)
- [ ] Example job files and configurations
- [ ] Test scripts in `e2e/`

### Example Configurations
- [ ] Example HCL files in `command/testdata/`
- [ ] Terraform examples in `terraform/`
- [ ] Vagrant configuration

---

## 8. Build and Release

### Build Configuration
- [ ] Package names in build scripts
- [ ] Release artifact naming
- [ ] Docker image names and tags
- [ ] Binary signing configuration

### Scripts
- [ ] `scripts/` directory - all scripts
- [ ] `dev/` directory scripts
- [ ] Release automation scripts

---

## 9. Dependencies and Third-Party

### Package Metadata
- [ ] `package.json` files (root and subdirectories)
- [ ] Go module metadata
- [ ] License file headers
- [ ] Copyright notices

### Third-Party Integrations
- [ ] Consul integration references
- [ ] Vault integration references
- [ ] Cloud provider configurations
- [ ] Monitoring/metrics configurations

---

## 10. Legal and Compliance

- [ ] License file (MPL 2.0) - Update copyright holder if needed
- [ ] Copyright notices in source files
- [ ] Trademark notices
- [ ] Attribution requirements

---

## Search Commands for Finding References

Use these commands to find remaining references:

```bash
# Find "Nomad" references (case-sensitive, code files)
grep -r "Nomad" --include="*.go" --include="*.js" --include="*.hcl" .

# Find "nomad" references (case-insensitive, all files)
grep -ri "nomad" --exclude-dir=".git" --exclude-dir="vendor" --exclude-dir="node_modules" .

# Find HashiCorp references
grep -ri "hashicorp" --exclude-dir=".git" --exclude-dir="vendor" --exclude-dir="node_modules" .

# Find nomadproject.io URLs
grep -r "nomadproject\.io" .

# Find environment variable references
grep -r "NOMAD_" --include="*.go" --include="*.sh" .
```

---

## Priority Order

1. **High Priority** - User-facing elements
   - Branding assets (logos, icons, favicons)
   - UI text and strings
   - Command names and help text
   - Main documentation (README)

2. **Medium Priority** - Developer-facing elements
   - Code comments and documentation
   - Test data and fixtures
   - Contributing guides
   - Build scripts

3. **Low Priority** - Historical/archive
   - Changelog entries (historical context)
   - Old release notes
   - Archived documentation

---

## Notes

- Some references to "Nomad" may be intentional (e.g., in changelogs for historical context)
- Consider keeping compatibility shims for configuration migration
- May need migration guide for existing users
- Consider deprecation warnings for old environment variables/paths

---

## Completed Work

- [x] Import paths updated from hashicorp/nomad to open-wander/wander (#2)
- [x] GitHub Actions cleaned up (#1, #3)
- [x] Test certificates updated (#5)
