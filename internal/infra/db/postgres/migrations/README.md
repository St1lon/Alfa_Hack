# Миграции

`golang-migrate`, файлы `NNNNN_name.{up,down}.sql`, каждая обратима.
Прогон — отдельным job в `docker compose` (`make up`) или `make migrate-up`.

**Номера разведены по владельцам**, чтобы два человека не создали `00003_` одновременно:

| Диапазон | Чей | Что там |
|---|---|---|
| `00001`–`00099` | Go #1 | новости, сюжеты, аналитика, пайплайн, LLM-аудит |
| `00100`–`00199` | Go #2 | компании, признаки, ранжирование, фидбек |
| `00200`–`00299` | Java | инсайты, evidence, agent_runs |
| `00300`–`00399` | Python | обогащение, метрики |

Уже применённую миграцию не правим — добавляем следующую.

**Права.** У каждого сервиса своя роль с минимальными правами (`00004_service_roles`,
[docs/SECURITY.md](../../../../../docs/SECURITY.md)): `svc_ingest`, `svc_gateway`,
`svc_api`, `svc_agent`, `svc_ml`. Завели новую таблицу — **в той же миграции**
выдайте права своей роли, иначе сервис получит `permission denied`:

```sql
CREATE TABLE company_alerts (...);
GRANT SELECT, INSERT, UPDATE ON company_alerts TO svc_api;
GRANT USAGE ON SEQUENCE company_alerts_id_seq TO svc_api;   -- если есть bigserial
```
Исключение действовало только до первого коммита.
