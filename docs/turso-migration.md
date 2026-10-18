# Переезд базы с Railway Volume на Turso

Сейчас база — SQLite-файл `/app/data/data.db` на Railway Volume (`DB_PATH=/app/data/data.db`).
После переезда она будет в Turso под именем **`bot-jam-house-cafe`**.

Как бот выбирает базу:

| Переменные в Railway | База |
|---|---|
| `TURSO_DATABASE_URL` не задана | SQLite-файл из `DB_PATH` (как сейчас) |
| `TURSO_DATABASE_URL` + `TURSO_AUTH_TOKEN` заданы | Turso |

Пока вы не добавите переменные Turso, бот после деплоя этой версии работает как раньше.

Для переезда в бот добавлены две команды. Обе доступны только админу:
- `/backup` присылает в чат копию базы `bot-jam-house-cafe.db`. Копия уже в режиме WAL, который нужен Turso для импорта, и под нужным именем.
- `/dbinfo` показывает, где сейчас база (SQLite или Turso), и сколько в ней пользователей, товаров, чеков и позиций.

---

## Этап 0. Подготовка (бот работает как обычно)

1. **Влейте PR и дождитесь деплоя.** В логах Railway должно быть:
   ```
   [config] DB=sqlite /app/data/data.db ...
   [storage] users=… products=… sales=… sale_items=… purchase_items=…
   ```
2. **Установите Turso CLI** на свой компьютер:
   - macOS: `brew install tursodatabase/tap/turso`
   - Linux / Windows (WSL): `curl -sSfL https://get.tur.so/install.sh | bash`
3. **Войдите:** `turso auth login`. Если аккаунта нет — `turso auth signup`.
4. **Выберите регион** поближе к региону сервиса в Railway: `turso db locations`. Чем ближе регион, тем быстрее бот отвечает.
5. **Репетиция (рекомендуется).** Пройдите шаги 3–5 этапа 1 с тестовой базой:
   ```
   turso db create bot-jam-house-cafe-test --from-file ./bot-jam-house-cafe.db
   turso db shell bot-jam-house-cafe-test "SELECT COUNT(*) FROM sales;"
   turso db destroy bot-jam-house-cafe-test
   ```
   Так вы заранее убедитесь, что импорт работает, а боевой переезд займёт минуты.

## Этап 1. Переезд (в нерабочее время, ~15 минут)

Всё, что продано после шага 3, в Turso не попадёт. Поэтому переезжайте, когда продаж нет.

1. **Предупредите персонал**, что 15 минут не проводить продажи.
2. **Зафиксируйте цифры.** Отправьте боту `/dbinfo` и запишите количество чеков, товаров и пользователей.
3. **Сделайте копию.** Отправьте боту `/backup` и сохраните присланный файл `bot-jam-house-cafe.db`. Не переименовывайте его.
4. **Создайте базу в Turso из файла:**
   ```
   turso db create bot-jam-house-cafe --from-file ./bot-jam-house-cafe.db
   ```
   Альтернатива — `turso db import ./bot-jam-house-cafe.db`: имя базы берётся из имени файла.
5. **Сверьте данные** с цифрами из шага 2:
   ```
   turso db shell bot-jam-house-cafe "SELECT (SELECT COUNT(*) FROM users), (SELECT COUNT(*) FROM products), (SELECT COUNT(*) FROM sales), (SELECT COUNT(*) FROM sale_items);"
   ```
6. **Получите адрес и токен:**
   ```
   turso db show bot-jam-house-cafe --url      # libsql://bot-jam-house-cafe-<ваш-org>.turso.io
   turso db tokens create bot-jam-house-cafe   # длинная строка — это секрет
   ```
7. **Добавьте переменные в Railway** (сервис бота → Variables):
   ```
   TURSO_DATABASE_URL=libsql://bot-jam-house-cafe-<ваш-org>.turso.io
   TURSO_AUTH_TOKEN=<токен из шага 6>
   ```
   `DB_PATH` и Volume пока не трогайте: это путь отката. Railway сам перезапустит сервис.
8. **Проверьте логи** Railway:
   ```
   [config] DB=turso libsql://bot-jam-house-cafe-...
   [storage] users=… products=… sales=…   ← те же цифры, что в шаге 2
   ```
   Если там `база даних недоступна`, значит неверный URL или токен. Исправьте переменную, Railway перезапустит бот.
9. **Проверьте в Telegram:**
   - `/dbinfo` должен показать «Turso» и те же цифры;
   - откройте «Відправити звіт» → «📅 Інша дата» → любой прошлый день: продажи должны быть на месте;
   - откройте «Залишки товарів».
10. **Разрешите продажи.**

## Этап 2. Наблюдение (1–2 недели)

Volume и `DB_PATH` остаются, но бот их уже не использует.

**Откат.** Удалите в Railway `TURSO_DATABASE_URL` и `TURSO_AUTH_TOKEN`, и бот вернётся к `/app/data/data.db`. Учтите: продажи, сделанные уже в Turso, в старом файле отсутствуют. Если откатываться приходится после нескольких дней работы, сначала выгрузите данные из Turso: `turso db shell bot-jam-house-cafe .dump > turso-dump.sql`. Это нужно, чтобы их можно было перенести.

## Этап 3. Уборка

Когда всё работает стабильно:

1. Удалите переменную `DB_PATH` в Railway.
2. Отключите и удалите Volume (сервис → Settings → Volumes).
3. Удалите сохранённые файлы `bot-jam-house-cafe.db` с компьютеров или храните их как архив в надёжном месте. В них вся история продаж.

## Резервные копии после переезда

- Turso хранит историю изменений (Point-in-Time Recovery; глубина зависит от тарифа). Восстановление делается в панели Turso или через `turso db create <новое-имя> --from-db bot-jam-house-cafe --timestamp <время>`.
- Ручная копия в SQL: `turso db shell bot-jam-house-cafe .dump > backup-$(date +%F).sql`.
- `/backup` в боте после переезда не делает файл: он подскажет, что копии делает Turso.

## Безопасность токена

- Токен даёт полный доступ к базе. Храните его только в переменных Railway, не пишите в чаты и не коммитьте в репозиторий.
- Если токен утёк, выполните `turso db tokens invalidate bot-jam-house-cafe`, затем создайте новый (шаг 6) и обновите переменную в Railway.
