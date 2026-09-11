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

The retention worker is deliberately separate from the domain and service
logic. It periodically removes old snapshots while retaining the latest
snapshot for each service/environment scope. It never edits state outside the
store transaction.
