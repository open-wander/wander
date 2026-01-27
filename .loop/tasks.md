# Project: UI Feature Parity - API Integration

> Wire up HTMX UI to real Nomad API, replacing mock data with live cluster data

Started: 2026-01-26
Status: in-progress

---

## Tasks

### Phase 1: ACL & Authentication (Foundation)

- [x] 1. Create ACL middleware to validate X-Nomad-Token header on all requests
- [x] 2. Integrate token validation with Nomad ACL API (client.ACLTokens().Self())
- [x] 3. Store user permissions in request context for downstream handlers
- [x] 4. Add per-handler permission checks with proper 403 Forbidden responses
- [x] 5. Implement token expiration handling and refresh logic
- [x] 6. Wire up Settings page to real token management (store/clear token)

### Phase 2: Detail Page API Integration

- [x] 7. Wire Jobs list to real API: client.Jobs().List() (needs: 1)
- [x] 8. Wire Job detail to real API: client.Jobs().Info() (needs: 7)
- [x] 9. Wire Job versions tab to real API: client.Jobs().Versions() (needs: 8)
- [x] 10. Wire Job allocations tab to real API: client.Jobs().Allocations() (needs: 8)
- [x] 11. Wire Job deployments tab to real API: client.Jobs().Deployments() (needs: 8)
- [x] 12. Wire Job evaluations tab to real API: client.Jobs().Evaluations() (needs: 8)
- [x] 13. Wire Allocations list to real API: client.Allocations().List() (needs: 1)
- [x] 14. Wire Allocation detail to real API: client.Allocations().Info() (needs: 13)
- [x] 15. Wire Allocation file browser to real API: client.AllocFS().List() (needs: 14)
- [x] 16. Wire Allocation file content to real API: client.AllocFS().Cat()/Stat() (needs: 15)
- [x] 17. Wire Allocation log streaming to real API: client.AllocFS().Logs() via SSE (needs: 14)
- [ ] 18. Wire Clients list to real API: client.Nodes().List() (needs: 1)
- [ ] 19. Wire Client detail to real API: client.Nodes().Info() (needs: 18)
- [ ] 20. Wire Client allocations to real API: client.Nodes().Allocations() (needs: 19)
- [ ] 21. Wire Client stats to real API: client.Nodes().Stats() (needs: 19)
- [ ] 22. Wire Servers list to real API: client.Agent().Members() (needs: 1)
- [ ] 23. Add Server detail page and handler with client.Agent().Self() (needs: 22)
- [ ] 24. Wire Evaluations list to real API: client.Evaluations().List() (needs: 1)
- [ ] 25. Add Evaluation detail page with client.Evaluations().Info() (needs: 24)
- [ ] 26. Wire Namespaces list to real API: client.Namespaces().List() (needs: 1)

### Phase 3: Write Operations (Actions)

- [ ] 27. Implement job plan: client.Jobs().Plan() with diff display (needs: 8)
- [ ] 28. Implement job submit/run: client.Jobs().Register() (needs: 27)
- [ ] 29. Implement job stop: client.Jobs().Deregister() (needs: 8)
- [ ] 30. Implement force periodic job: client.Jobs().PeriodicForce() (needs: 8)
- [ ] 31. Implement dispatch parameterized job: client.Jobs().Dispatch() (needs: 8)
- [ ] 32. Implement job version revert: client.Jobs().Revert() (needs: 9)
- [ ] 33. Implement task group scaling: client.Jobs().Scale() (needs: 8)
- [ ] 34. Implement allocation restart: client.Allocations().Restart() (needs: 14)
- [ ] 35. Implement allocation stop: client.Allocations().Stop() (needs: 14)
- [ ] 36. Implement allocation exec: WebSocket to /v1/client/allocation/:id/exec (needs: 14)
- [ ] 37. Implement client drain: client.Nodes().UpdateDrain() (needs: 19)
- [ ] 38. Implement client eligibility toggle: client.Nodes().ToggleEligibility() (needs: 19)
- [ ] 39. Wire Variables list to real API: client.Variables().List() (needs: 1)
- [ ] 40. Wire Variable read to real API: client.Variables().Read() (needs: 39)
- [ ] 41. Implement Variable create: client.Variables().Create() (needs: 40)
- [ ] 42. Implement Variable update: client.Variables().Update() (needs: 40)
- [ ] 43. Implement Variable delete: client.Variables().Delete() (needs: 40)
- [ ] 44. Wire Policies list to real API: client.ACLPolicies().List() (needs: 1)
- [ ] 45. Wire Policy read to real API: client.ACLPolicies().Info() (needs: 44)
- [ ] 46. Implement Policy create/update: client.ACLPolicies().Upsert() (needs: 45)
- [ ] 47. Implement Policy delete: client.ACLPolicies().Delete() (needs: 45)

### Phase 4: Missing Pages

- [ ] 48. Create Job submission page with CodeMirror editor (needs: 27, 28)
- [ ] 49. Add job template library to submission page (needs: 48)
- [ ] 50. Create Deployments list handler and page (needs: 11)
- [ ] 51. Create Deployment detail page with task group status (needs: 50)
- [ ] 52. Implement deployment promote action (needs: 51)
- [ ] 53. Implement deployment fail action (needs: 51)
- [ ] 54. Wire CSI Volumes list to real API: client.CSIVolumes().List() (needs: 1)
- [ ] 55. Wire CSI Volume detail to real API: client.CSIVolumes().Info() (needs: 54)
- [ ] 56. Wire CSI Plugins list to real API: client.CSIPlugins().List() (needs: 1)
- [ ] 57. Wire Topology page to real API: build from Nodes + Allocations (needs: 18, 13)

### Phase 5: Real-Time Updates

- [ ] 58. Add HTMX polling to allocation status on job detail (needs: 10)
- [ ] 59. Implement SSE endpoint for real log streaming from Nomad (needs: 17)
- [ ] 60. Add HTMX polling for deployment progress (needs: 51)
- [ ] 61. Add HTMX polling for evaluation status (needs: 25)
- [ ] 62. Add connection status indicator with reconnection logic (needs: 59)

### Phase 6: UX Enhancements

- [ ] 63. Implement keyboard shortcuts (g+j=Jobs, g+c=Clients, etc.) (needs: 7)
- [ ] 64. Implement global search with /v1/search/fuzzy endpoint (needs: 1)
- [ ] 65. Create 403 Forbidden error page with ACL message (needs: 4)
- [ ] 66. Create 404 Not Found error page (needs: 1)
- [ ] 67. Create No Leader error page for cluster unavailable state (needs: 1)
- [ ] 68. Verify breadcrumb navigation completeness across all pages (needs: 57)
- [ ] 69. Add region switcher to header with API region parameter (needs: 1)
- [ ] 70. Verify version diff viewer working with real data (needs: 9, 32)

---

## Notes

### Reference Codebase (IMPORTANT)
The old Ember.js UI is available at `../wander-old/ui/` for feature parity reference:
```
../wander-old/ui/
├── app/
│   ├── routes/          # Route logic (jobs/, clients/, allocations/, etc.)
│   ├── controllers/     # Controller logic with query params, actions
│   ├── components/      # 123 components with behaviors
│   ├── templates/       # HBS templates showing UI structure
│   ├── models/          # Data models (55 models)
│   ├── adapters/        # API adapters
│   └── services/        # Services (token, system, etc.)
```

**When implementing a feature, check the old codebase:**
1. `../wander-old/ui/app/routes/{feature}/` - Route logic
2. `../wander-old/ui/app/controllers/{feature}.js` - Actions, query params
3. `../wander-old/ui/app/templates/{feature}/` - UI structure
4. `../wander-old/ui/app/components/` - Reusable behaviors

**Key files for common features:**
- Jobs: `routes/jobs/`, `controllers/jobs/`, `templates/jobs/`
- Allocations: `routes/allocations/`, `templates/allocations/`
- Clients: `routes/clients/`, `templates/clients/`
- ACL/Token: `services/token.js`, `routes/settings.js`

### API Client Access
The Nomad API client is available via the Handler struct. Each handler receives access through dependency injection:
```go
type Handler struct {
    client *api.Client
    // ...
}
```

### ACL Flow
1. Token comes from `X-Nomad-Token` header or cookie
2. Middleware validates token via `client.ACLTokens().Self()`
3. Permissions stored in context
4. Handlers check permissions before operations

### Error Handling
- Invalid token: 401 Unauthorized
- Missing permission: 403 Forbidden
- Resource not found: 404 Not Found
- Cluster unavailable: 503 Service Unavailable

### Namespace Handling
Many API calls require namespace parameter. Get from:
1. Query parameter `?namespace=foo`
2. Header `X-Nomad-Namespace`
3. Default to "default" namespace

### Real-Time Patterns
- Log streaming: SSE with client.AllocFS().Logs()
- Exec: WebSocket proxying to Nomad agent
- Status updates: HTMX polling with `hx-trigger="every 5s"`

### Testing Strategy
After each task:
1. Start dev cluster: `nomad agent -dev`
2. Test with real data
3. Test with/without ACL token
4. Verify error handling

### Agent Recommendations
- Phase 1 (ACL): security-auditor + golang-pro
- Phase 2-3 (API): golang-pro
- Phase 4 (Pages): golang-pro + frontend-design
- Phase 5-6 (Polish): golang-pro + javascript-pro
