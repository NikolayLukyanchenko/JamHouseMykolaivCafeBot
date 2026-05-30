# JamHouseMykolaivCafeBot

Telegram-бот для молодіжного кафе **JamHouse** у Миколаєві: **@JamHouseCafeMykolaiv**. Увесь інтерфейс бота — українською мовою.

## 1. Опис бота та можливостей

Бот допомагає касирам та адміністраторам кафе:
- авторизуватись строго за Telegram User ID;
- вести продажі через калькулятор замовлення;
- автоматично списувати залишки товарів у SQLite;
- переглядати меню для клієнтів, залишки та власні продажі за день;
- формувати запити на закупку;
- керувати товарами через `/admin`;
- формувати звіт за день із виручкою, собівартістю та прибутком.

## 2. Вимоги

- **Go 1.23+**
- SQLite-файл `data.db`
- Telegram-бот, створений через **@BotFather**

## 3. Як налаштувати бота через @BotFather

1. Відкрийте **@BotFather** у Telegram.
2. Виконайте команду `/newbot`.
3. Вкажіть назву та username бота.
4. Скопіюйте виданий токен — це значення для `TELEGRAM_BOT_TOKEN`.

## 4. Змінні оточення

- `TELEGRAM_BOT_TOKEN` — **обов'язково**, токен Telegram-бота.
- `DB_PATH` — необов'язково, шлях до SQLite-файлу. За замовчуванням: `data.db`.
- `ADMIN_IDS` — список Telegram User ID адміністраторів через кому.

## 5. Як додати першого адміністратора

Перші адміністратори додаються автоматично зі змінної `ADMIN_IDS` під час старту бота.

Приклад:

```env
ADMIN_IDS=123456789,987654321
```

Щоб дізнатись власний Telegram User ID, можна скористатися ботом **@userinfobot**.

## 6. Локальний запуск

1. Склонуйте репозиторій.
2. Створіть `.env` на основі `.env.example`.
3. Встановіть залежності:

```bash
go mod tidy
```

4. Запустіть бота:

```bash
go run .
```

## 7. Деплой на Railway / Render / Fly.io

Бот працює через **long polling**, тому його слід запускати як **worker/background service**, а не як HTTP-вебсервіс. Також обов'язково використайте **persistent volume**, інакше файл `data.db` буде втрачатися між деплоями.

### Railway

1. Створіть новий проєкт і підключіть репозиторій.
2. Додайте змінні `TELEGRAM_BOT_TOKEN`, `ADMIN_IDS`, за потреби `DB_PATH`.
3. Підключіть volume і змонтуйте його, наприклад, у `/data`.
4. Вкажіть `DB_PATH=/data/data.db`.
5. Запуск: `go run .` або попередньо зібраний бінарник.

### Render

1. Створіть **Background Worker**.
2. Підключіть репозиторій.
3. Додайте environment variables: `TELEGRAM_BOT_TOKEN`, `ADMIN_IDS`, `DB_PATH`.
4. Додайте persistent disk і змонтуйте його, наприклад, у `/var/data`.
5. Вкажіть `DB_PATH=/var/data/data.db`.
6. Build command: `go build -o app .`
7. Start command: `./app`

### Fly.io

1. Створіть застосунок командою `fly launch --no-deploy`.
2. Створіть volume для SQLite-файлу.
3. Додайте секрети `TELEGRAM_BOT_TOKEN` і `ADMIN_IDS` через `fly secrets set`.
4. Вкажіть `DB_PATH`, що посилається на volume, наприклад `/data/data.db`.
5. Запустіть застосунок як worker-процес.

## 8. Приклад `.env.example`

```env
TELEGRAM_BOT_TOKEN=your_bot_token_here
DB_PATH=data.db
ADMIN_IDS=123456789,987654321
```

## 9. Структура проєкту

```text
main.go
config/
bot/
  handlers/
  keyboards/
models/
storage/
reports/
utils/
README.md
.env.example
```
