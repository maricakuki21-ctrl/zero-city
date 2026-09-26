# Zero City decision packet: not frozen

**Status:** `not-frozen`
**Scope:** evidence packet only; none of these modules is runnable, authoritative, or production-complete.

## Ranking policy

- Question: contribution, answer quality, asset evidence and Tavern hosting may need separate scorecards.
- Counterargument: a single score encourages gaming and makes spend/rarity a shortcut.
- Required evidence before a freeze: independently replayable source events, published period, anti-abuse review and an appeal path.
- Implementation decision: no rank calculation, rank persistence or leaderboard write is included.

## Card economy

- Question: collectible cards may support identity and memory.
- Counterargument: trading, synthesis and scarcity couple identity presentation to economic pressure.
- Required evidence before a freeze: ownership policy, non-financial boundary, abuse analysis and reversible prototype.
- Implementation decision: no trade, synthesis, rarity economy, balance, privilege or recommendation input is included.

## AI room memory

- Question: a Tavern role might retain context across a room or between rooms.
- Counterargument: memory scope, consent, retention and model/billing ownership are not specified.
- Required evidence before a freeze: identity card, consent model, retention/deletion policy, canonical execution and billing trace.
- Implementation decision: no AI memory storage, retrieval, prompt state or background execution is included.

## Game rules and room policy

- Question: scripted play requires rules, roles, authority and rewards.
- Counterargument: inventing a state machine or reward loop would fabricate game authority and economic facts.
- Required evidence before a freeze: versioned manifest, server-authoritative event schema, replay schema, moderation policy and a separated reward policy.
- Implementation decision: this module only joins, completes, discovers and replays existing Tavern rooms; it creates no game rule or reward state.
