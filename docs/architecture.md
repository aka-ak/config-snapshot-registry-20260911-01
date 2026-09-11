# Architecture

The service stores immutable configuration snapshots keyed by an application,
environment, and caller-provided snapshot ID. A snapshot contains arbitrary
nested JSON values and a list of dependent services. The current version keeps
the ingestion path small so a later module can consume the same records without
changing the write contract.

All mutations go through a store revision. The memory store is used by tests;
the file store persists a complete JSON state through a temporary file and
rename. The service retries short optimistic-concurrency conflicts and appends
an audit event in the same committed state as each mutation.

The engine flattens nested objects into stable dotted paths. Scalar values and
arrays are compared using canonical JSON bytes, so map key order does not alter
the result. It emits added, removed, and modified changes in path order.

The impact module builds on the engine to assess the blast radius of a change.
Given a service, an environment, and two snapshot IDs, it diffs the snapshots
and performs a reverse-dependency lookup over the latest snapshot of every
scope in that environment: any scope whose newest snapshot depends on the
changed service is reported as potentially affected, with the snapshot ID as
evidence. The assessment refuses to guess — both snapshots must belong to the
requested scope, and every dependency the changed service declares must have
at least one snapshot in the same environment, otherwise the module returns an
explicit scope-mismatch or incomplete-dependencies error instead of a partial
answer. The module only reads state; the write contract is unchanged.

The retention worker is deliberately separate from the domain and service
logic. It periodically removes old snapshots while retaining the latest
snapshot for each service/environment scope. It never edits state outside the
store transaction.
