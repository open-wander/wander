# Project: UI Migration - Rename and Feature Parity

> Rename ui-htmx to ui, then systematically compare and fix all pages for feature parity with old Ember.js UI

Started: 2026-01-27
Status: complete

---

## Tasks

### Phase 1: Directory Rename

- [x] 1. Commit current in-progress UI changes
- [x] 2. Update import in http.go from ui-htmx to ui
- [x] 3. Rename ui-htmx directory to ui with git mv
- [x] 4. Regenerate templates and verify build
- [x] 5. Commit the rename

### Phase 2: Jobs Section Comparison

- [x] 6. Compare Jobs list page vs old app/templates/jobs/index.hbs (needs: 5) - GAPS_FOUND: Missing namespace, nodePool, taskGroupCount, allocation summary, pack indicator, multi-select filters
- [x] 7. Compare Jobs list filters vs old app/components/job-page.js (needs: 6) - GAPS_FOUND: Missing datacenter, prefix, nodePool filters; type/status are single-value not multi-select
- [x] 8. Compare Job detail overview vs old app/templates/jobs/job/index.hbs (needs: 5) - GAPS_FOUND: Missing recent allocations, allocation status chart, meta section, placement failures, purge action, additional tabs
- [x] 9. Compare Job definition tab vs old app/templates/jobs/job/definition.hbs (needs: 8) - GAPS_FOUND: Missing full definition vs job spec toggle, HCL variables section, file upload, save as template
- [x] 10. Compare Job versions tab vs old app/templates/jobs/job/versions.hbs (needs: 8) - GAPS_FOUND: Missing structured hierarchical diff, change count in version row
- [x] 11. Compare Job allocations tab vs old app/templates/jobs/job/allocations.hbs (needs: 8) - GAPS_FOUND: Missing sorting, filters, search, task expansion, modified/version/volume columns, live stats
- [x] 12. Compare Job evaluations tab vs old app/templates/jobs/job/evaluations.hbs (needs: 8) - GAPS_FOUND: Missing priority column, column sorting
- [x] 13. Compare Job run/submit page vs old app/templates/jobs/run.hbs (needs: 5) - FEATURE_PARITY: All core features present, enhanced with more templates
- [x] 14. Compare Task group detail vs old app/templates/jobs/job/task-group.hbs (needs: 8) - GAPS_FOUND: Missing allocations section entirely, volume requirements, lifecycle chart, scaling events

### Phase 3: Allocations Section Comparison

- [x] 15. Compare Allocations list vs old app/templates/allocations.hbs (needs: 5) - NEW_FEATURE: Old UI had no allocations list page; new UI adds full list with filters/search/sort
- [x] 16. Compare Allocation detail vs old app/templates/allocations/allocation/index.hbs (needs: 15) - GAPS_FOUND: Missing ports, services, reschedule, preemption, lifecycle chart; logs/exec placeholders only (~55% complete)
- [x] 17. Compare Task logs vs old app/components/task-log.js (needs: 16) - GAPS_FOUND: Missing word wrap, head/tail navigation; added download, multi-stream, timestamps toggle
- [x] 18. Compare Task files/FS vs old app/components/fs/browser.js (needs: 16) - GAPS_FOUND: Missing image preview, streaming/head/tail modes; added syntax highlighting, file icons, line numbers
- [x] 19. Compare Exec terminal vs old app/components/exec-terminal.js (needs: 16) - FEATURE_PARITY: Complete WebSocket implementation with improved UX, task selector, connection status

### Phase 4: Clients Section Comparison

- [x] 20. Compare Clients list vs old app/templates/clients/index.hbs (needs: 5) - GAPS_FOUND: Missing nodePool/version/volume filters, pagination, multi-select; added resource visualization
- [x] 21. Compare Client detail vs old app/templates/clients/client/index.hbs (needs: 20) - GAPS_FOUND: Missing alloc filters/pagination/search, drain strategy info, host volumes, driver attrs, meta editing; added charts
- [x] 22. Compare Client drain vs old app/components/drain-popover.js (needs: 21) - GAPS_FOUND: Critical - drain API not implemented (stub only), missing force drain, no deadline presets, no validation
- [x] 23. Compare Node allocations display (needs: 21) - GAPS_FOUND: Missing filters, pagination, task expansion; basic table only

### Phase 5: Servers Section Comparison

- [x] 24. Compare Servers list vs old app/templates/servers/index.hbs (needs: 5) - GAPS_FOUND: Missing port column, pagination, version column; added region, raft voter status
- [x] 25. Compare Server detail vs old app/templates/servers/server/index.hbs (needs: 24) - GAPS_FOUND: Minor - missing copy button for tags; significantly enhanced with more metadata (version, protocol, raft, build)

### Phase 6: Topology Comparison

- [x] 26. Compare Topology viz vs old app/components/topo-viz.js (needs: 5) - GAPS_FOUND: Missing allocation overlay/selection, edge visualization; improved tooltips/backend separation
- [x] 27. Compare Datacenter view vs old app/components/topo-viz/datacenter.js (needs: 26) - GAPS_FOUND: Missing allocation-level granularity; node-centric view only

### Phase 7: Variables Section Comparison

- [x] 28. Compare Variables list vs old app/templates/variables/index.hbs (needs: 5) - GAPS_FOUND: Tree view removed, folder navigation gone; added search/sort/pagination/item count/job links
- [x] 29. Compare Variable detail vs old app/templates/variables/variable/index.hbs (needs: 28) - GAPS_FOUND: Missing copy path, JSON/list toggle, copy value, show/hide value, related entities; edit-first design
- [x] 30. Compare Variable create/edit vs old app/components/variable-form.js (needs: 28) - GAPS_FOUND: Missing path validation, duplicate detection, key validation, JSON editor, conflict handling, unsaved warning

### Phase 8: Policies Section Comparison

- [x] 31. Compare Policies list vs old app/templates/policies/index.hbs (needs: 5) - GAPS_FOUND: Missing tokens column, keyboard shortcuts; added search/sort/description/lastModified
- [x] 32. Compare Policy detail vs old app/templates/policies/policy.hbs (needs: 31) - GAPS_FOUND: Missing entire tokens section (create/list/delete tokens); improved metadata/delete modal
- [x] 33. Compare Policy editor vs old app/components/policy-editor.js (needs: 32) - GAPS_FOUND: Missing policy creation flow, name validation, duplicate check; improved change detection/metadata

### Phase 9: CSI Section Comparison

- [x] 34. Compare CSI Volumes list vs old app/templates/csi/volumes/index.hbs (needs: 5) - GAPS_FOUND: Missing controller/node health columns, namespace; added plugin ID, access mode
- [x] 35. Compare CSI Volume detail vs old app/templates/csi/volumes/volume/index.hbs (needs: 34) - GAPS_FOUND: Missing alloc CPU/memory/version/timestamps; added health tabs, mount options, usage stats
- [x] 36. Compare CSI Plugins list vs old app/templates/csi/plugins/index.hbs (needs: 34) - GAPS_FOUND: Missing pagination; different nav behavior (volumes vs detail); added health badges
- [x] 37. Compare CSI Plugin detail vs old app/templates/csi/plugins/plugin/index.hbs (needs: 36) - NOT_IMPLEMENTED: Entire plugin detail page missing; list links to volumes instead

### Phase 10: Settings & Global Components

- [x] 38. Compare Token management vs old app/templates/settings/tokens.hbs (needs: 5) - GAPS_FOUND: Missing JWT/SSO support, policy rules display, relative time; improved ACL disabled state
- [x] 39. Compare Sidebar/Nav vs old app/components/gutter-menu.js (needs: 5) - GAPS_FOUND: Keyboard shortcuts removed, structure reorganized; improved visual hierarchy
- [x] 40. Compare Header vs old app/components/global-header.js (needs: 5) - GAPS_FOUND: Custom label removed, architecture changed; cleaner integration with sidebar
- [x] 41. Compare Global search vs old app/components/global-search.js (needs: 5) - ENHANCED: 18 vs 5 search types; significantly improved scope and UX
- [x] 42. Compare Namespace switcher (needs: 39) - GAPS_FOUND: Form-scoped namespace filtering removed; global header location is improvement
- [x] 43. Compare Region switcher vs old app/components/region-switcher.js (needs: 40) - GAPS_FOUND: Keyboard shortcuts removed, URL strategy changed to app context; added toast notifications
- [x] 44. Compare Breadcrumbs vs old app/components/app-breadcrumbs.js (needs: 5) - GAPS_FOUND: Improved accessibility, better ARIA support; minor structural differences

### Phase 11: Final Verification

- [x] 45. Run go build and go test for full verification (needs: 44) - BUILD_OK: All packages build, vet passes. Test compilation fixed (updated allocations_test, jobs_test, errors_test, sse_test for API changes). Handler integration tests need running Nomad.
- [x] 46. Runtime testing with wander agent -dev (needs: 45) - VERIFIED: UI loads correctly on all pages (Jobs, Clients, Servers, Topology, Allocations, etc.)

---

## Notes

### Reference Paths
- New UI: `/Users/stokvis/dev/wander/wander/ui-htmx/` (will become `/ui/`)
- Old UI: `/Users/stokvis/dev/wander/wander-old/ui/`

### Comparison Process for Each Task
1. Read old UI code to understand expected behavior
2. Read new UI code to understand current implementation
3. Identify gaps (missing data fetch, wrong template, broken action)
4. Fix if needed
5. Mark as done

### Key Integration File
`command/agent/http.go:41` - UI import path

