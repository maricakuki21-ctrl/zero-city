# BizDecipher Feature Module

This directory is the enterprise feature boundary for the BizDecipher / Zero City product layer.

The repository still contains inherited gateway infrastructure in generic folders such as `api`, `views`, `components`, and `composables`. New BizDecipher product work should live here first, then expose compatibility shims only when older routes or imports still depend on legacy paths.

## Structure

```text
features/bizdecipher/
  api/                  Typed API clients for `/biz` and `/admin/biz` surfaces
  components/           Product-owned UI components
    common/             Shared BizDecipher visual primitives
    layout/             Product layout/entry widgets
    shared-pool/        Account Square / shared-pool cards
  constants/            Product constants and mascot catalog
  views/
    admin/              Admin governance/review pages
    landing/            Public BizDecipher positioning pages
    user/               User-facing Zero City, tavern, pool, and asset pages
```

## Product Boundary

BizDecipher is not the inherited gateway itself. The gateway is the traffic and billing substrate. This module owns the product strategy layers:

- Account Square and shared pools as supply network
- Community and Zero City as relationship/trust layer
- Capability assets as trust proof
- Tavern/runtime flows as early workbench surface
- Promo/check-in/card incentives as supply selection mechanisms

## Migration Rule

Prefer imports from this module when editing moved product code:

```ts
import { listSharedPools } from '@/features/bizdecipher/api/bizdecipher'
import { zeroCityMascots } from '@/features/bizdecipher/constants/zeroCityMascots'
```

Legacy paths under `src/api`, `src/views`, `src/components`, and `src/constants` may remain as compatibility wrappers until callers are migrated.
