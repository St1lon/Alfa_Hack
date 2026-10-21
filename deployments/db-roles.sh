#!/bin/sh
# Включает вход для ролей сервисов и ставит пароли из окружения.
# Роли и права создаёт миграция 00004_service_roles; здесь — только то, чему
# не место в SQL репозитория: пароли. Запускается job'ом db-roles после миграций.
#
# Пароли передаются psql-переменными и подставляются через :'...' — psql сам
# экранирует их, кавычка в пароле не сломает и не подменит запрос.
set -eu

psql -v ON_ERROR_STOP=1 \
  -v ingest="${INGEST_DB_PASSWORD}" \
  -v gateway="${GATEWAY_DB_PASSWORD}" \
  -v api="${API_DB_PASSWORD}" \
  -v agent="${AGENT_DB_PASSWORD}" \
  -v ml="${PYML_DB_PASSWORD}" <<'SQL'
ALTER ROLE svc_ingest  LOGIN PASSWORD :'ingest';
ALTER ROLE svc_gateway LOGIN PASSWORD :'gateway';
ALTER ROLE svc_api     LOGIN PASSWORD :'api';
ALTER ROLE svc_agent   LOGIN PASSWORD :'agent';
ALTER ROLE svc_ml      LOGIN PASSWORD :'ml';
SQL

echo "db-roles: вход для svc_ingest, svc_gateway, svc_api, svc_agent, svc_ml включён"
