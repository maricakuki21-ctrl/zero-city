# Zero City bounded-journey provenance

- Source workspace (read-only): `D:\BizDecipher-NewSystem-20260819`
- Source Git commit: `969eed3e3f52d0764c23d32f21100552d6806d9e`
- Adopted interfaces:
  - `frontend/src/features/bizdecipher/api/community.ts` supplies durable discussion discovery, replies and optional room-to-post conversion.
  - `frontend/src/features/bizdecipher/api/bizdecipher.ts` supplies Tavern room lifecycle, public identity history and capability-asset lookup.
  - `frontend/src/features/bizdecipher/views/user/{CommunityView,TavernView,ZeroCityProfileView,CapabilityAssetsView}.vue` remains the current product surface; this module is its bounded journey seam and route-wiring handoff.
- Rejected: source `data/zeroCityRankings.ts`, `data/zeroCityModules.ts`, `data/communityPreview.ts`, and every card/ranking/economy runtime rule. They do not enter this implementation.

This module deliberately creates no Sub2 health, usage, throughput, scheduler, gateway, price, ledger, balance, or media-task fact. Relay and Workbench activation is represented only by an explicit user navigation descriptor; Task 17 owns its final route wiring and execution.
