# NaymPro Backend - Серверная часть сервиса по поиску работы
## Автор: Александриди-Шандаевский Е. Д. ИКБО-20-22

## Фронтентд часть приложения - https://github.com/grannnsacker/naympro_front

## Технологии
- Go 1.24+
- PostgreSQL
- Docker
- Gin Web Framework

## Требования
- Go 1.21 или выше
- Docker и Docker Compose
- PostgreSQL 15+

## Установка и запуск

### 1. Клонирование репозитория
```bash
git clone https://github.com/your-username/job-finder-back.git
cd job-finder-back
```

### 2. Настройка окружения
Создайте файлы `.env` и ф`app.env`в корневой директории проекта:
```env
DB_SOURCE=postgresql://postgres:postgres@localhost:5432/job_finder?sslmode=disable
SERVER_ADDRESS=0.0.0.0:8080
TOKEN_SYMMETRIC_KEY=your-secret-key
ACCESS_TOKEN_DURATION=15m
REFRESH_TOKEN_DURATION=24h
```
```env
BOT_TOKEN=TOKEN
```

### 3. Запуск с помощью Docker
```bash
docker-compose up -d
```

## API Endpoints

### Пользователи
- `POST /users/register` - Регистрация нового пользователя
- `POST /users/login` - Вход в систему
- `GET /users/profile` - Получение профиля пользователя
- `PUT /users/profile` - Обновление профиля

### Вакансии
- `POST /jobs` - Создание новой вакансии
- `GET /jobs` - Получение списка вакансий
- `GET /jobs/:id` - Получение информации о вакансии
- `PUT /jobs/:id` - Обновление вакансии
- `DELETE /jobs/:id` - Удаление вакансии

### Отклики на вакансии
- `POST /applications` - Создание отклика на вакансию
- `GET /applications/user` - Получение откликов пользователя
- `GET /applications/employer` - Получение откликов работодателя
- `PUT /applications/:id/status` - Обновление статуса отклика

## Тестирование

### Запуск всех тестов
```bash
go test ./...
```

### Запуск тестов с покрытием
```bash
go test -v -cover ./...
```


## Структура проекта
```
.
├── cmd/                    # Точка входа приложения
├── internal/              # Внутренний код приложения
│   ├── api/              # API handlers и middleware
│   ├── config/           # Конфигурация приложения
│   ├── db/               # Работа с базой данных
│   └── esearch/          # Поисковая система
├── pkg/                   # Публичные пакеты
│   └── utils/            # Утилиты
├── .env                  # Конфигурация окружения сервиса уведомлений
├── app.env               # Конфигурация окружения основного сервиса
├── docker-compose.yml    # Docker конфигурация
└── go.mod               # Зависимости Go
```
