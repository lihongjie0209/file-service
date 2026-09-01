# Migrations

Database-specific migrations live under `mysql`, `postgres`, and `kingbase`. Set `migration.path` to the matching directory, for example `migrations/postgres`. Review indexes, collation and online-DDL impact against production data before deployment.

Application scoping uses an expand/backfill/contract deployment. Migration `000006` adds a nullable scope and application-aware index. Before `000007` is applied to an existing database, backfill every file with its authoritative application ID; the contract migration intentionally fails while unknown scopes remain.
