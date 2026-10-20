#!/usr/bin/env bash
# Скачивание материалов кейса из Google Drive (ссылки — из ТЗ, docs/CASE.pdf).
# Файлы в git не попадают (.gitignore), поэтому каждый участник тянет их сам:
#   bash scripts/fetch_case_data.sh
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DATA="$ROOT/data"
RESEARCH="$DATA/research"
mkdir -p "$DATA" "$RESEARCH"

# id файла в Drive -> путь назначения
FILES=(
  # Приложение 1. Данные для участников
  "12H7sbLwBcpur0fKaGHPqwPwfXfgQq10b|$DATA/df_Uid_data_hack.csv"
  "1zkvWkAS0kZqijD-og_0Cv3o01SoUMY_Z|$DATA/df_Uid_payment_hack.csv"
  "1ZJCiqgY1ma4iQ1_ltabtUuaFIBkrqnOe|$DATA/df_Fid_Uid_link_hack.csv"
  # Приложение 2. Аналитические материалы Альфа-Банка
  "1bPzix_332N2CyMY0tqCHuYtGsfJWnFGv|$RESEARCH/bond_brief_14092026.pdf"
  "1yLCBrIoyIsubLTEBvBHh6io48ysnW-wn|$RESEARCH/alfa_russian_steels_1q21_15apr2021.pdf"
  "1HZDq3WGRFdzgm1TZ5hWL23Q3I9c2Eq64|$RESEARCH/alfa_macro_stat_7m26.pdf"
  "1riR1lhsb6EY1MwGMxdBlS_tI5ed9mIiz|$RESEARCH/alfa_consumption.pdf"
  "1shHG89LtMgHT1_uEebHmQSFZe_zbeq7-|$RESEARCH/alfa_policy_rate_11sep.pdf"
  "1khcHwoS0FkG9p_0J4XoHGaGq3B4V0ime|$RESEARCH/alfa_macro_stat_1h26.pdf"
  "1hq3Fhyz9ce_sRyGUQB1MdoHeQdfAJfhK|$RESEARCH/fi_strategy_3q26_22072026.pdf"
  "1a7TXOCHARfKzUR0hn9Cm9gQSrIldxTDE|$RESEARCH/gold_digest_august26.pdf"
  "19iW3Ph8_wO0HHQY7Ik4lTfC3jRc9gjWV|$RESEARCH/inflation_tracker_02.pdf"
)

for entry in "${FILES[@]}"; do
  id="${entry%%|*}"
  out="${entry#*|}"
  name="$(basename "$out")"

  if [[ -s "$out" ]]; then
    echo "= $name — уже скачан, пропускаю"
    continue
  fi

  echo "↓ $name"
  curl -sfL --retry 3 --retry-delay 2 \
    -o "$out" \
    "https://drive.usercontent.google.com/download?id=${id}&export=download&confirm=t"

  # Drive на ошибке отдаёт HTML-страницу вместо файла — ловим это сразу.
  if head -c 64 "$out" | grep -qi "<!doctype html\|<html"; then
    echo "  ! вместо файла пришёл HTML (нет доступа или изменилась ссылка)"
    rm -f "$out"
    continue
  fi
  echo "  ок, $(du -h "$out" | cut -f1)"
done

echo
echo "Готово. Содержимое data/:"
ls -lh "$DATA" "$RESEARCH"
