# Business Capability Specification, Config Propagator

## 1) Purpose and Value
Provide a standardized, low‑risk way to distribute and keep application configuration in sync across many Kubernetes namespaces, using a single source of truth. This reduces configuration drift, accelerates change rollout, and simplifies onboarding or offboarding of environments selected by labels. Business impact includes faster time to market for configuration changes, fewer outages from misconfiguration, and improved auditability of change activity.

## 2) Business Outcomes and Success Metrics
**Target outcomes**
- Configuration consistency across selected namespaces, with drift visible and actionable
- Predictable, low‑blast‑radius updates via controlled rollout
- Clear operational insights, status, and eventing, enabling fast troubleshooting
- Safe deprovisioning when configurations are no longer needed

**Example KPIs**
- 95th percentile end‑to‑end propagation time from source update to target update, for example, ≤ 60 seconds in the same cluster
- Reduction in configuration‑related incidents, for example, 50 percent in six months
- Drift detection mean time to detect, for example, ≤ 1 minute from modification
- Change success rate, for example, ≥ 99 percent of namespace updates complete without retry
- Cleanup completeness after deprecation, for example, 100 percent of deselected targets pruned within five minutes

## 3) Scope
**In scope**
- Propagate a single source ConfigMap into one or many target namespaces selected by labels
- Keep targets synchronized as the source changes, or as namespace labels change
- Apply controlled update strategies, rolling or immediate
- Provide status, events, and metrics for observability
- Safe deletion and finalization of managed copies

**Out of scope**
- Secrets management, cross‑cluster synchronization, propagation of resources other than ConfigMaps

## 4) Primary Users and Stakeholders
- Application platform teams, define policy, guardrails, and global configuration
- Application teams, manage app‑specific ConfigMaps and target selection via labels
- SRE and operations, monitor propagation status, investigate drift or failures
- Security and compliance, rely on auditable events, no handling of Secrets

## 5) Core Capabilities
1) **Single source of truth synchronization**
   - Maintain copies of a source ConfigMap across selected namespaces, optionally copy only specified keys
2) **Target selection by labels**
   - Dynamically include or exclude namespaces based on label selectors, responds to label changes
3) **Controlled rollout of changes**
   - Support rolling strategy that updates a small number of namespaces at a time, default batch size 5, or immediate strategy when required
4) **Drift detection and conflict handling**
   - Detect content drift using a recorded hash
   - Policy options, overwrite to enforce source truth, or skip to preserve local changes while recording drift
5) **Safe deprovisioning and lifecycle controls**
   - Prune managed targets when deselected or when the propagation resource is deleted, controlled by policy
   - Finalizers ensure clean handoff and prevent orphaned resources
6) **Operational visibility**
   - Status conditions, synced and out‑of‑sync namespaces, last sync time, and events for key actions and errors
   - Metrics for propagation activity and errors, enable dashboards and alerts
7) **Resilience and failure handling**
   - Partial progress tolerated, retries with backoff, RBAC gaps isolated to affected namespaces, large payloads flagged for manual action

## 6) Business Processes and Typical Flows
**Enable configuration standardization**
- Create a propagation record pointing to the authoritative ConfigMap, select namespaces by label such as team or environment
- As teams add labels to namespaces, copies appear automatically, updates flow without additional effort

**Roll out a configuration change safely**
- Update the source ConfigMap
- Operator rolls out updates to target namespaces using the selected update strategy, events and metrics validate success

**Offboard or deprecate**
- Remove namespaces from selection labels or delete the propagation record
- Operator prunes or detaches managed copies based on policy, leaving a clean state

## 7) Policies and Guardrails
- Default update strategy is rolling to limit blast radius
- Default conflict policy is overwrite to preserve a single source of truth
- Default prune policy is true to prevent configuration sprawl
- Operator touches only resources it manages, identified by its annotations, avoids user‑owned resources by default

## 8) Service Levels and Operational Targets
- Availability, aligns with control plane uptime, targets 99.9 percent for reconciliation service
- Performance, 95 percent of updates applied within 60 seconds in cluster, configurable via batch size and refresh interval
- Scalability, thousands of target namespaces per source, rolling batches control load
- Supportability, clear events, conditions, and metrics enable first‑line diagnosis within 15 minutes

## 9) Reporting and Observability
- **Dashboards**, total propagations, targets per propagation, error rates, out‑of‑sync counts, age of last sync
- **Alerts**, source missing, high out‑of‑sync rate, repeated update failures, payload too large
- **Audit**, event logs for create, update, prune, conflict decisions, and finalization

## 10) Risk Register and Mitigations
- **Mass rollout incident risk**, use rolling strategy, small batch size, and quick rollback by reverting the source
- **RBAC gaps**, clearly surfaced in out‑of‑sync lists, continue healthy namespaces, alert owners
- **Oversized payloads**, detect, warn, and block, require operator guidance or payload reduction
- **Unintended target selection**, standardized label taxonomies, change review for selectors, dry‑run in lower environments
- **Drift due to local edits**, enforce overwrite unless explicitly using skip, surface drift in status and dashboards

## 11) Dependencies and Constraints
- Kubernetes cluster RBAC that permits the operator to read the source namespace and manage ConfigMaps in target namespaces
- Label governance for namespaces, agreed naming and lifecycle for labels that drive selection
- ConfigMap size limits apply, typical kube limits for object size impact payload feasibility
- Single‑cluster scope, no cross‑cluster operation

## 12) Compliance and Security Considerations
- No Secrets managed, reduces exposure of sensitive data
- Least privilege RBAC recommended for the operator
- Change transparency via events and conditions supports audit and compliance reporting

## 13) Acceptance Criteria, Business Level
- When a propagation is created for a valid source, copies appear in all matching namespaces within the SLA window, with status showing Ready and synced namespaces
- When the source changes, selected targets update per rollout policy, with observable events and metrics
- When a namespace is labeled into selection, a copy appears automatically, when it is unlabeled, the copy is pruned or detached per policy
- If a target cannot be updated due to RBAC, that namespace is listed as out of sync without blocking others, and an alert is raised
- If a user edits a managed target and conflict policy is skip, drift is recorded and the target is not overwritten
- Deleting the propagation cleans up managed copies per prune policy and removes finalizers

## 14) Deliverables for Productization
- Service catalog entry that describes capability, policies, and how to request onboarding
- Runbook, common failure modes, RBAC errors, payload limits, and rollback steps
- Grafana or equivalent dashboard templates mapped to the provided metrics
- Quick start guide, how to configure labels, select namespaces, and choose strategies
- Governance doc for label taxonomy used for targeting

## 15) Phased Rollout Plan
- Pilot, small set of teams and namespaces, validate label taxonomy and SLAs
- Scale up, expand to critical apps, tune batch size and alert thresholds
- Organization wide adoption, enforce default policies, publish dashboards and runbooks

## 16) Glossary
- **ConfigMap**, non‑secret key value configuration for Kubernetes workloads
- **Selector**, label based query that identifies target namespaces
- **Rolling update**, update a limited number of targets at a time to limit blast radius
- **Prune**, remove managed copies that are no longer selected or on deletion
- **Drift**, mismatch between target and source configuration

---

### Traceability, Technical to Business Mapping

| Technical Feature | Business Capability Enabled |
|---|---|
| Label‑based target selection | Dynamic onboarding and offboarding of namespaces without per‑team tickets |
| Rolling or immediate strategy | Risk‑controlled or fast‑path rollouts tailored to change criticality |
| Conflict policy, overwrite or skip | Enforce source of truth or preserve local exceptions with visibility |
| Finalizers and prune | Clean decommissioning, prevent configuration sprawl |
| Status conditions and events | Operational transparency, faster incident triage |
| Metrics, totals and errors | Executive and SRE dashboards, SLO tracking |
| Backoff and partial retries | Resilience, progress despite localized failures |
| Hash based drift detection | Early detection of unauthorized or accidental changes |

