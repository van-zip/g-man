<div align="center">

# 🤖 G-MAN

### Высокопроизводительный фреймворк на Go для автоматизации Steam и многопоточных торговых систем

[![Go Reference](https://img.shields.io/badge/go-reference-007d9c?logo=go&logoColor=white&style=flat-square)](https://pkg.go.dev/github.com/lemon4ksan/g-man)
[![Go Report Card](https://goreportcard.com/badge/github.com/lemon4ksan/g-man?style=flat-square)](https://goreportcard.com/report/github.com/lemon4ksan/g-man)
[![License](https://img.shields.io/github/license/lemon4ksan/g-man?style=flat-square)](LICENSE)
[![GitHub Stars](https://img.shields.io/github/stars/lemon4ksan/g-man?style=flat-square)](https://github.com/lemon4ksan/g-man/stargazers)

> _"Правильный бот в неправильном месте может изменить весь рынок скинов."_

#### 🇺🇸 [English](README.md) • 🇷🇺 [Русский](README_RU.md)

</div>

**G-man** — это высокопроизводительный, промышленный SDK клиентской части Steam и фреймворк для автоматизации игровых операций на Go. Разработанный для высокочастотного трейдинга, масштабного управления инвентарем и отказоустойчивой сетевой работы, G-man объединяет сеть Steam и игровые координаторы (Game Coordinators) в единый потокобезопасный оркестратор. Он бесшовно сочетает протоколы **Socket (CM)**, **WebAPI** и **игровые координаторы**, обеспечивая непрерывную работу ваших автоматизированных цепочек в режиме 24/7.

```shell
go get github.com/lemon4ksan/g-man@latest
```

## 🛠 Архитектурный обзор

Система спроектирована на основе слабосвязанной событийно-ориентированной архитектуры с использованием модели CSP в Go. Объект `Client` выступает центральным оркестратором, распределяя события между изолированными потокобезопасными модулями и автоматически балансируя нагрузку:

```mermaid
flowchart LR
    classDef steam fill:#1b2838,stroke:#66c0f4,stroke-width:2px,color:#fff;
    classDef transport fill:#2a475e,stroke:#66c0f4,stroke-width:1px,color:#c7d5e0;
    classDef core fill:#171a21,stroke:#cba6f7,stroke-width:2px,color:#cdd6f4;
    classDef module fill:#313244,stroke:#a6e3a1,stroke-width:1px,color:#cdd6f4;
    classDef pipeline fill:#45475a,stroke:#f9e2af,stroke-width:1px,color:#f9e2af,stroke-dasharray: 5 5;
    classDef action fill:#a6e3a1,stroke:#a6e3a1,stroke-width:2px,color:#11111b;

    subgraph External [Steam Сеть]
        Steam((Steam Cloud))
    end
    class External steam;

    subgraph Transport [Транспортный движок]
        direction TB
        Socket[CM-клиент Socket]
        WebAPI[REST / WebAPI]
    end
    class Transport,Socket,WebAPI transport;

    subgraph Core [G-MAN Оркестратор]
        Router{Сервисный роутер}
        Bus([Шина событий Bus])
    end
    class Core,Router,Bus core;

    subgraph Modules [Доменные модули]
        direction TB
        GameGC[Игровой GC Диспетчер]
        Social[Чат & Друзья]
        Ach[Достижения]
    end
    class Modules,GameGC,Social,Ach module;

    subgraph TradeEngine [Торговый движок Onion]
        direction LR
        P1[Дедупликация] --> P2[Черный список] --> P3[Проверка цены] --> P4[Вердикты]
    end
    class TradeEngine,P1,P2,P3,P4 pipeline;

    Verdict{Вердикт}
    class Verdict action;

    Steam <--> Socket & WebAPI
    Socket & WebAPI <--> Router
    Router <--> Bus

    Bus <--> GameGC & Social & Ach

    GameGC -- "Новое предложение" --> P1
    P4 --> Verdict

    Verdict -- "Принять/Отклонить" --> Router
    Router -- "Выполнить" --> Steam
```

## ⚡ Ключевые возможности

### 🔄 Самовосстанавливающиеся сессии (Silent Re-auth)
Простой бота — это потерянная прибыль. G-man в фоновом режиме отслеживает состояние веб-сессий и токенов доступа. Если веб-куки истекают прямо во время выполнения запроса, оркестратор автоматически приостанавливает активные операции, атомарно обновляет OAuth2-сессию, сохраняет новые токены в хранилище и возобновляет запросы прозрачно для пользователя. Ваша бизнес-логика никогда не столкнется с ошибками `401 Unauthorized` или разрывом сессии.

### 🌐 Двухстековый транспортный движок
Больше не нужно выбирать между WebAPI и сокетами Connection Manager (CM). Протокольно-независимый слой маршрутизации G-man динамически выбирает оптимальный путь: **TCP/WebSocket CM-каналы** для синхронизации состояния в реальном времени с минимальной задержкой или **HTTPS WebAPI** для массовых транзакций и минимизации лимитов запросов. При разрыве сокета движок автоматически и плавно переходит на HTTP.

### 🧅 Конвейерная система сделок (Onion Trade Middleware)
Стройте сложную логику проверки сделок в виде независимых middleware-компонентов. Обрабатывайте входящие предложения через цепочку фильтров: `Deduplicator` $\rightarrow$ `SecurityEscrowCheck` $\rightarrow$ `BlacklistFilter` $\rightarrow$ `PriceValidator` $\rightarrow$ `Verdict`. Если любой из middleware выносит окончательный вердикт (Принять/Отклонить/Контр-предложение), цепочка прерывается, исключая состояние гонки.

### 🌡️ Защитный веб-скрейпинг
Steam часто возвращает «мягкие ошибки» — HTML-страницы со статусом `200 OK`, содержащие текст с предупреждением (например, «Превышен лимит запросов», блок «Семейного просмотра» или форму авторизации). Защитный скрейпер модуля `community` сканирует тела ответов, переводит двусмысленный HTML в строго типизированные ошибки Go и инициирует обработчики безопасности.

### 🛠️ Надежная зависимость модулей
Модули встраивают `module.Base` и топологически сортируются с помощью трехцветного алгоритма поиска в глубину (DFS) при запуске. Это гарантирует, что модули инициализируются и запускаются в строго определенном порядке, а циклические зависимости вызывают немедленный отказ при старте с информативной ошибкой.

## 📂 Структура проекта

```text
pkg/
├── steam/            # Ядро протокола Steam и управление жизненным циклом
│   ├── auth/         # Авторизация OAuth2, персистентность и фоновое обновление токенов
│   ├── socket/       # Стейтфул-клиент TCP/WebSocket Connection Manager (CM)
│   ├── protocol/     # Форматы сообщений Steam, скомпилированные protobuf-файлы и спецификации
│   ├── transport/    # Двухстековый транспортный мост (маршрутизатор Socket/HTTP)
│   ├── social/       # Чат в реальном времени, статусы пользователей и списки друзей
│   ├── community/    # Защитные скрейперы (Инвентари, Маркет, Steam Guard)
│   └── sys/          # Внутренние подсистемы (диспетчер игрового координатора, директория)
├── behavior/         # Общее автоматизированное поведение ботов
│   └── achievements/ # Эмулятор достижений с человекоподобным поведением
├── trading/          # Унифицированный движок торговых предложений
│   └── engine/       # Конвейер обработки (Onion) с контекстом TradeContext
├── bus/              # Потокобезопасная шина событий для слабой связи модулей
├── rest/             # REST-клиент с санитаризацией и типизацией ответов
├── command/          # Потокобезопасная обработка и маршрутизация команд
├── jobs/             # Асинхронные задачи и планировщик заданий
├── crypto/           # Криптографические утилиты (Steam TOTP, шифрование)
└── storage/          # Стандартные адаптеры хранилища ключ-значение (Память, Локальный JSON)
```

## 🚀 Быстрый старт

### 1. Инициализация и запуск клиента

Подключитесь к сети Steam, пройдите авторизацию и запустите ваши модули всего несколькими строками кода:

```go
package main

import (
	"context"
	"os"

	"github.com/lemon4ksan/g-man/pkg/log"
	"github.com/lemon4ksan/g-man/pkg/steam"
	"github.com/lemon4ksan/g-man/pkg/steam/auth"
	"github.com/lemon4ksan/g-man/pkg/steam/sys/directory"
	"github.com/lemon4ksan/g-man/pkg/storage/jsonfile"
	webtrading "github.com/lemon4ksan/g-man/pkg/trading/web"
)

func main() {
	// 1. Настраиваем хранилище сессий в JSON-файле для сохранения токенов авторизации
	store, _ := jsonfile.New("storage.json")
	logger := log.New(log.DefaultConfig(log.LevelInfo))

	// 2. Инициализируем оркестратор с базовыми модулями
	client, _ := steam.NewClient(steam.Config{Storage: store},
		steam.WithLogger(logger),
		webtrading.WithModule(webtrading.Config{}),
	)
	defer client.Close()

	// 3. Подключаем торговый менеджер сделок
	webTradeManager := client.Module("trading").(*webtrading.Manager)
	
	// Настройте ваш движок и обработчик здесь...
	// Для торговли в TF2 вы можете подключить пакет g-man-tf2 прямо на этом этапе!

	// 4. Запрашиваем оптимальный сервер и выполняем вход
	dir := directory.New(client.Service())
	server, _ := dir.GetOptimalCMServer(context.Background())
	login := auth.NewLogOnDetails(os.Getenv("STEAM_USER"), os.Getenv("STEAM_PASS"))

	if err := client.Run(); err != nil {
		panic(err)
	}

	if err := client.ConnectAndLogin(context.Background(), server, login); err != nil {
		panic(err)
	}

	client.Wait()
}
```

### 2. Конфигурация промежуточного ПО сделок (Onion Trade Middlewares)

Вы можете построить сложную цепочку фильтров сделок с помощью независимых middleware. Вот пример простого фильтра, проверяющего ценность сделки:

```go
package main

import (
	"github.com/lemon4ksan/g-man/pkg/trading/engine"
	"github.com/lemon4ksan/g-man/pkg/trading/reason"
)

// PriceValidationMiddleware контролирует общую стоимость сделки
func PriceValidationMiddleware(priceLimit int) engine.Middleware {
	return func(next engine.Handler) engine.Handler {
		return func(ctx *engine.TradeContext) error {
			totalGiveValue := 0
			for _, item := range ctx.Offer.ItemsToGive {
				// Доступ к метаданным цен, установленным ранее другими middleware в контексте
				if price, ok := ctx.Get("price_" + item.SKU); ok {
					totalGiveValue += price.(int)
				}
			}

			if totalGiveValue > priceLimit {
				// Стоимость превышает лимит: отправляем на ручную проверку или отклоняем
				ctx.Review(reason.ReviewEngineError)
				return nil // Прерываем цепочку
			}

			return next(ctx)
		}
	}
}
```

## 🎮 Расширения для поддержки игр

G-MAN спроектирован с возможностью легкой адаптации под нужные игры с помощью слабосвязанных модулей. Основным доступным расширением является:

* **[g-man-tf2](https://github.com/lemon4ksan/g-man-tf2)**: Пакет автоматизации торговли и экономики Team Fortress 2
  - **Металлическая арифметика:** Точные расчеты с ключами, очищенными, восстановленными металлами и ломом.
  - **Автоплавка:** Динамическое объединение оружия и плавка металлов для формирования точной сдачи.
  - **Синхронизация с Backpack.tf:** Фоновое управление активными листингами и демпинг цен конкурентов.
  - **Парсер схем:** Динамическое O(1)-индексируемое приведение предметов к нормальной SKU.

## 🚀 Производительность и эффективность памяти

G-MAN оптимизирован для работы в высоконагруженных промышленных средах с минимальным потреблением ресурсов:
- **Базовое ядро бота:** Потребляет всего **~4.5 МБ** активной памяти кучи (включая шину событий, социальные модули и менеджеры торговли).
- **Минимум аллокаций:** Использование пулов буферов и строгая оптимизация сетевых интерфейсов снижают нагрузку на GC.
- **Детерминированный запуск:** Исключены гонки инициализации и избыточные блокировки благодаря топологической сортировке.

## 🏗 Дорожная карта

### Инфраструктура ядра
- [x] **Маршрутизация транспорта:** Потокобезопасная отправка запросов через сокеты или HTTP.
- [x] **Сессии WebSession:** Фоновый авто-запуск keep-alive для веб-кук и API-ключей.
- [x] **Тихая повторная авторизация:** Автоматическое фоновое восстановление просроченных JWT.
- [x] **Топологическая сортировка:** Детерминированный порядок запуска модулей и контроль циклов.
- [x] **Прокси-туннелирование:** Чистая интеграция SOCKS5/HTTP для всех модулей из коробки.
- [ ] **Загрузчик Steam CDN:** Динамическое скачивание манифестов и игровых ассетов.

### Торговые протоколы
- [x] **Onion Trade Middleware:** Гибкий конвейер из независимых фильтров для входящих сделок.
- [x] **Защитный веб-скрейпинг:** Конвертация HTML-страниц с ошибками в строго типизированные ошибки Go.
- [ ] **Координатор CS2:** GC-рукопожатие, парсинг скинов и лобби-менеджер.
- [ ] **Координатор Dota 2:** Поддержка SOCache-пакетов предметов и управление кастомными лобби.

## 🤝 Участие в разработке

Мы рады новым участникам! Если вы хотите расширить поддержку баз данных, добавить новые структуры GC для Dota 2 / CS2 или оптимизировать скрейпинг:

1. Ознакомьтесь с философией проектирования в [CONTRIBUTING.md](CONTRIBUTING.md).
2. Минимизируйте количество внешних сетевых зависимостей, пропуская трафик через интерфейс `transport.Doer`.
3. Напишите тесты и убедитесь в потокобезопасности изменений с помощью `go test -race ./...`.

## ☕ Поддержка проекта

Создание масштабируемых и отказоустойчивых ботов для Steam требует сотен часов обратной разработки протоколов. Если G-MAN помог сэкономить ресурсы ваших серверов или автоматизировать ваши торговые операции, вы можете поддержать проект:

<div align="center">

[![Trade Offer](https://img.shields.io/badge/Steam-Trade_Offer-blue?style=for-the-badge&logo=steam)](https://steamcommunity.com/tradeoffer/new/?partner=1141078357&token=HjsTJQFX)

> _"Пожертвования... не обязательны, но... соответствуют условиям нашего... соглашения."_

</div>

## ⚖️ Лицензия и правовая информация

**Дисклеймер:** Это программное обеспечение **не** связано с корпорацией **Valve**, не поддерживается и не одобряется ей. Steam, Team Fortress 2 и все сопутствующие товарные знаки являются собственностью Valve Corporation. Вы используете данный фреймворк на свой страх и риск.

Проект распространяется под лицензией **BSD 3-Clause License**. Подробности в файле [LICENSE](LICENSE).

---

<div align="center">
  <sub>Prepare for unforeseen consequences... or just prepare for the next Steam Sale.</sub>
</div>
