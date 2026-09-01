## Resetting the db

Postgres data lives in a named docker volume. Compose prefixes it with the project
directory, so the name is `wavelen_wavelen-pgdata` (not the `wavelen-pgdata`).

```sh
docker volume ls | grep wavelen
```

### Reset

```sh
docker compose down -v # `-v` drops the volume.
docker compose up -d postgres
make migrate/up
```

Or

```sh
make dc/down # volume cannot be removed while a container uses it
docker volume rm wavelen_wavelen-pgdata
```

### Check where the schema is

```sh
make db/psql
```

```sql
SELECT version, dirty FROM schema_migrations;
\d users
```

Or

```sh
docker exec wavelen-postgres psql -U "$POSTGRES_USER" -d wavelen -c '\d users'
```
