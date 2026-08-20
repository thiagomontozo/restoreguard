# Validation engine

Checks are typed modules: `FILE_EXISTS`, `FILE_SIZE`, `SHA256`, `POSTGRES_CONNECTIVITY`, safe profiled PostgreSQL table/row rules, and `HTTP_HEALTH`. Every check has a bounded timeout, required flag, status, summary, metrics, and optional evidence reference.

Ordinary users cannot submit arbitrary SQL. PostgreSQL validators build known parameterized queries from validated identifiers/profiles. HTTP health is restricted to explicitly authorized private/loopback sandbox destinations, disables redirects, and uses short timeouts. A future elevated custom-query feature would require a separate permission and policy review.

Required failure prevents `VERIFIED`. Optional limitation can yield `PARTIALLY_VERIFIED`/`MEDIUM`. Cancellation, timeout, missing capability, and insufficient observations produce `INCONCLUSIVE` rather than success.
