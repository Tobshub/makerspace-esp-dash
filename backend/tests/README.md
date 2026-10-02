# Backend tests

Unit tests sit next to the package they cover (`internal/.../*_test.go`).

`internal/server/registry_test.go` signs in two users, checks project isolation, and checks that a device secret is returned once and stored only as a hash. It skips when PostgreSQL is not reachable.
