# Project: UI Migration - Rename and Feature Parity

> Rename ui-htmx to ui, then systematically compare and fix all pages for feature parity with old Ember.js UI

Started: 2026-01-27
Status: in-progress

---

## Tasks

### Phase 1: Directory Rename

- [x] 1. Commit current in-progress UI changes
- [x] 2. Update import in http.go from ui-htmx to ui
- [x] 3. Rename ui-htmx directory to ui with git mv
- [x] 4. Regenerate templates and verify build
- [ ] 5. Commit the rename

### Phase 2: Jobs Section Comparison

- [ ] 6. Compare Jobs list page vs old app/templates/jobs/index.hbs (needs: 5)
- [ ] 7. Compare Jobs list filters vs old app/components/job-page.js (needs: 6)
- [ ] 8. Compare Job detail overview vs old app/templates/jobs/job/index.hbs (needs: 5)
- [ ] 9. Compare Job definition tab vs old app/templates/jobs/job/definition.hbs (needs: 8)
- [ ] 10. Compare Job versions tab vs old app/templates/jobs/job/versions.hbs (needs: 8)
- [ ] 11. Compare Job allocations tab vs old app/templates/jobs/job/allocations.hbs (needs: 8)
- [ ] 12. Compare Job evaluations tab vs old app/templates/jobs/job/evaluations.hbs (needs: 8)
- [ ] 13. Compare Job run/submit page vs old app/templates/jobs/run.hbs (needs: 5)
- [ ] 14. Compare Task group detail vs old app/templates/jobs/job/task-group.hbs (needs: 8)

### Phase 3: Allocations Section Comparison

- [ ] 15. Compare Allocations list vs old app/templates/allocations.hbs (needs: 5)
- [ ] 16. Compare Allocation detail vs old app/templates/allocations/allocation/index.hbs (needs: 15)
- [ ] 17. Compare Task logs vs old app/components/task-log.js (needs: 16)
- [ ] 18. Compare Task files/FS vs old app/components/fs/browser.js (needs: 16)
- [ ] 19. Compare Exec terminal vs old app/components/exec-terminal.js (needs: 16)

### Phase 4: Clients Section Comparison

- [ ] 20. Compare Clients list vs old app/templates/clients/index.hbs (needs: 5)
- [ ] 21. Compare Client detail vs old app/templates/clients/client/index.hbs (needs: 20)
- [ ] 22. Compare Client drain vs old app/components/drain-popover.js (needs: 21)
- [ ] 23. Compare Node allocations display (needs: 21)

### Phase 5: Servers Section Comparison

- [ ] 24. Compare Servers list vs old app/templates/servers/index.hbs (needs: 5)
- [ ] 25. Compare Server detail vs old app/templates/servers/server/index.hbs (needs: 24)

### Phase 6: Topology Comparison

- [ ] 26. Compare Topology viz vs old app/components/topo-viz.js (needs: 5)
- [ ] 27. Compare Datacenter view vs old app/components/topo-viz/datacenter.js (needs: 26)

### Phase 7: Variables Section Comparison

- [ ] 28. Compare Variables list vs old app/templates/variables/index.hbs (needs: 5)
- [ ] 29. Compare Variable detail vs old app/templates/variables/variable/index.hbs (needs: 28)
- [ ] 30. Compare Variable create/edit vs old app/components/variable-form.js (needs: 28)

### Phase 8: Policies Section Comparison

- [ ] 31. Compare Policies list vs old app/templates/policies/index.hbs (needs: 5)
- [ ] 32. Compare Policy detail vs old app/templates/policies/policy.hbs (needs: 31)
- [ ] 33. Compare Policy editor vs old app/components/policy-editor.js (needs: 32)

### Phase 9: CSI Section Comparison

- [ ] 34. Compare CSI Volumes list vs old app/templates/csi/volumes/index.hbs (needs: 5)
- [ ] 35. Compare CSI Volume detail vs old app/templates/csi/volumes/volume/index.hbs (needs: 34)
- [ ] 36. Compare CSI Plugins list vs old app/templates/csi/plugins/index.hbs (needs: 34)
- [ ] 37. Compare CSI Plugin detail vs old app/templates/csi/plugins/plugin/index.hbs (needs: 36)

### Phase 10: Settings & Global Components

- [ ] 38. Compare Token management vs old app/templates/settings/tokens.hbs (needs: 5)
- [ ] 39. Compare Sidebar/Nav vs old app/components/gutter-menu.js (needs: 5)
- [ ] 40. Compare Header vs old app/components/global-header.js (needs: 5)
- [ ] 41. Compare Global search vs old app/components/global-search.js (needs: 5)
- [ ] 42. Compare Namespace switcher (needs: 39)
- [ ] 43. Compare Region switcher vs old app/components/region-switcher.js (needs: 40)
- [ ] 44. Compare Breadcrumbs vs old app/components/app-breadcrumbs.js (needs: 5)

### Phase 11: Final Verification

- [ ] 45. Run go build and go test for full verification (needs: 44)
- [ ] 46. Runtime testing with wander agent -dev (needs: 45)

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

