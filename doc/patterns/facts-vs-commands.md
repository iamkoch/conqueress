# Facts versus commands

A guidance note on when to event-source an inbound write and when to treat it
as a fact that needs idempotent storage instead.

## TL;DR

Not every inbound write should land as a command on an aggregate.

A **command** is a request to make a decision: "create the order", "cancel
the booking", "approve the application". The domain can reject it; the
aggregate exists to protect the invariants that justify the rejection.

A **fact** is an observation that has already happened: a sensor reading, a
payment received, a webhook from an external system, a packet from a device.
The domain has no decision to make. The fact is either recorded or, due to
retry, recorded twice and deduplicated; in neither case is the fact rejected.

Routing facts through commands and aggregates is the wrong mechanism. At low
volume it works and you might not notice. At high volume the impedance
mismatch surfaces as duplicate-key errors, optimistic concurrency contention,
and aggregates that bloat to tens of thousands of events without protecting
any invariant.

## The shape of the problem

Suppose a service accepts biometric samples from a wearable. The natural
modelling impulse, once you have a CQRS/ES skeleton, is:

1. Sample arrives at `POST /samples`.
2. Controller issues a `RecordSampleCommand`.
3. Handler loads the `Session` aggregate, applies the command, emits a
   `SampleRecorded` event.
4. Event store persists the event with optimistic concurrency on the
   aggregate version.
5. Subscribers project the event into read models.

This is the orthodox path. It also explodes when two samples for the same
session arrive concurrently. Both handlers load the session at version `N`,
both try to append `SampleRecorded` at version `N+1`, one wins, the other
retries. Under a burst (a strap drains 50,000 historical packets in one
sync; a fleet of devices opens streams at the same minute) the retries
cascade and the duplicate-key errors propagate to the caller.

The clue that something is off: the aggregate's `Apply(SampleRecorded)`
method has no meaningful effect on aggregate state. It increments a counter
and stops. The aggregate is being used as a queue for facts, not as a
decision boundary.

## When a write is a fact

A write is a fact when all of these hold:

1. **The producer is not asking permission.** The data already exists in
   the real world (a measurement, a payment, a packet). Rejecting it has no
   meaning; the fact is true regardless of what the service thinks.
2. **There is a natural deduplication key.** Timestamps, content hashes,
   external IDs. Two arrivals with the same key are the same fact.
3. **The aggregate the orthodox path would route through enforces no
   invariant on the fact.** Its `Apply` method just records the fact and
   updates a counter.
4. **The volume is non-trivial.** Either steady (a stream) or bursty (a
   periodic sync of accumulated history).

If all four hold, the orthodox path is overhead with no benefit.

## What to do instead

Route facts to **idempotent durable storage** directly. Do not route them
through commands and aggregates. The ingestion path becomes:

1. Fact arrives at the controller.
2. Controller writes it (or them, in a batch) to a fact store via an
   idempotent upsert on the natural key.
3. Done. No aggregate load, no version, no retry loop.

The fact store is the source of truth for that data, not a projection. Read
models that depend on the facts query the fact store (or a derived
projection of it) rather than replaying the event stream.

Decisions stay event-sourced. In the wearable example, opening and closing
a session is a decision (it transitions the session's state, and competing
opens / closes need to be serialised); the samples accumulated within an
open session are facts. The same service uses both patterns side by side.

## Practical guidance

- **Pick the natural key carefully.** For a wearable, `(owner_id,
  sample_timestamp)` is enough if timestamps have sufficient resolution.
  For external webhooks, the upstream's event ID is usually right. For
  packet streams, a content hash is robust against accidental re-sends.
- **Batch the writes.** A controller receiving 1,000 samples should do one
  `UpsertBatch`, not 1,000 individual upserts. Most databases have an
  efficient idiom for this: Postgres `INSERT ... ON CONFLICT`, MongoDB
  `BulkWrite` with upsert, etc.
- **Index for the read patterns up front.** Fact stores are often hot on
  range scans by time. Get the indexes right at creation time; backfilling
  them later on a large table is expensive.
- **Don't replay the fact log into the event log.** That defeats the
  purpose. The fact store is its own source of truth.
- **Keep decision aggregates small.** Once facts are out, the aggregate's
  event stream is the lifecycle events (opened, configured, closed) and
  nothing else. It rehydrates quickly and snapshots become unnecessary.

## What you give up

- **Cross-aggregate transactional consistency** between a fact and its
  containing decision. A sample can land for a session that has just been
  closed. Treat the closed session as a soft signal: store the sample
  anyway (it really happened), surface a warning in observability, decide
  in the projection layer whether to include it.
- **A single audit log of everything.** Facts live in the fact store; the
  event stream has only the decisions. Reconstructing "what happened on
  date X" needs to query both. In practice the fact store is the source
  most queries want.
- **The conceptual simplicity of "everything is an event".** You now have
  two mechanisms in the same service. Document the rule (an ADR is the
  natural home) so that contributors know which path to use for new
  writes.

## Anti-patterns

- **Inventing a fake aggregate to hold facts.** A "SampleStream" aggregate
  whose only job is to accumulate samples is the orthodox path in disguise.
  It still serialises on its version. Skip the aggregate entirely.
- **Using event sourcing to retrofit dedup.** Some teams add an idempotency
  cache in front of the command handler to absorb duplicate facts. This
  works but is the wrong fix: the dedup is incidental to the model. Move
  the facts out and the dedup becomes a property of the fact store's
  natural key.
- **Reading raw facts directly from every endpoint.** The fact store is
  fine as a source of truth, but heavy reads should still go through a
  projection (or a view cache for expensive aggregations). The fact store
  is for ingest correctness; projections are for read performance.

## Further reading

- Vaughn Vernon, *Effective Aggregate Design*. Small aggregates exist to
  protect true invariants, not to accumulate facts.
- Mathias Verraes, *Messaging Flavours*. Distinguishes informational
  messages (observations, e.g. "the temperature was measured") from
  imperative messages (commands, e.g. "measure the temperature").
- Microsoft, *Azure Architecture Center, Event Sourcing pattern*. "Event
  sourcing doesn't have to be an all-or-nothing decision; apply it
  selectively."
- Greg Young, *CQRS Documents*. The original split between writes that
  need decision-handling and reads that need projection-handling, with
  facts naturally sitting on the read side of the line.

## When to revisit

This guidance applies when you have a write that meets all four criteria
above. If the producer is asking permission, or there is no natural
deduplication key, or the aggregate genuinely protects an invariant on the
write, stay on the orthodox path. The pattern is selective, not universal.
