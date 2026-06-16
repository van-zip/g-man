<div align="center">

# ⚙️ Базовый оркестратор Steam

### Жизненный цикл клиента, маршрутизация протоколов и диспетчеризация событий

#### 🇺🇸 [English](README.md) • 🇷🇺 [Русский](README_RU.md)

</div>

Пакет `steam` является координирующим ядром фреймворка G-man. Он объединяет низкозадержечный бинарный обмен данными по CM-сокетам с WebAPI и скрейпингом Сообщества в единый потокобезопасный `steam.Client`.

## ⚙️ Зоны ответственности

* **Унифицированная маршрутизация:** Предоставляет единый интерфейс `Client.Do()`. Если клиент подключен к сокету CM, запросы сериализуются в пакеты `EMsg`. При отсутствии соединения или если конечная точка требует HTTPS, запросы автоматически перенаправляются на WebAPI.
* **Управление авторизацией:** Координирует полный цикл сессий (OAuth2, генерация кук, получение ключей API и обработка Steam Guard), автоматически выполняя обновление токенов для поддержания активного подключения.
* **Публикация событий:** Интегрируется с шиной событий (`pkg/bus`) для трансляции изменений состояния (например, разрывов соединения, успешного входа или входящих пакетов сокета) зарегистрированным модулям.

## 🚀 Пример использования: Авторизация и RPC-вызовы

В примере ниже показано, как инициализировать клиент, подключиться к сети Steam, выполнить вход и вызвать Unified Service API с использованием protobuf-структур:

```go
package main

import (
	"context"

	"github.com/lemon4ksan/g-man/pkg/log"
	pb "github.com/lemon4ksan/g-man/pkg/protobuf/steam"
	"github.com/lemon4ksan/g-man/pkg/steam"
	"github.com/lemon4ksan/g-man/pkg/steam/auth"
	"github.com/lemon4ksan/g-man/pkg/steam/service"
	"github.com/lemon4ksan/g-man/pkg/steam/sys/directory"
	"google.golang.org/protobuf/proto"
)

func main() {
	ctx := context.Background()
	logger := log.New(log.DefaultConfig(log.LevelInfo))
	defer logger.Close()

	// 1. Инициализируем оркестратор
	cfg := steam.DefaultConfig()
	client, err := steam.NewClient(cfg, steam.WithLogger(logger))
	if err != nil {
		logger.Error("Не удалось инициализировать оркестратор", log.Err(err))
		return
	}
	defer func() {
		_ = client.Close()
		client.Wait()
	}()

	// Запускаем фоновые циклы
	if err := client.Run(); err != nil {
		panic(err)
	}

	// 2. Получаем оптимальные адреса серверов CM
	dir := directory.New(client.Service())
	server, err := dir.GetOptimalCMServer(ctx)
	if err != nil {
		logger.Error("Не удалось получить адрес сервера CM", log.Err(err))
		return
	}

	// 3. Подготавливаем учетные данные
	details := &auth.LogOnDetails{
		AccountName: "your_username",
		Password:    "your_password",
	}

	logger.Info("Подключение к сети Steam...")
	if err := client.ConnectAndLogin(ctx, server, details); err != nil {
		logger.Error("Ошибка входа", log.Err(err))
		return
	}
	defer func() {
		_ = client.Disconnect()
	}()

	// 4. Вызываем Unified WebAPI Service через роутер протоколов
	req := &pb.CPlayer_GetNickname_Request{
		Steamid: proto.Uint64(76561198000000000),
	}

	res, err := service.Unified[pb.CPlayer_GetNickname_Response](ctx, client.Service(), req)
	if err != nil {
		logger.Error("Ошибка вызова RPC", log.Err(err))
		return
	}

	logger.Info("Никнейм получен через Service Router", log.String("nickname", res.GetNickname()))
}
```

## 🛠️ Архитектурные паттерны оркестратора

### 1. Функциональные опции для регистрации модулей
Избегайте ручной регистрации модулей после создания клиента. Передавайте ваши расширения (например, движки автоматизации, торговые менеджеры) в виде функциональных опций при инициализации:

```go
client, err := steam.NewClient(cfg,
    steam.WithLogger(logger),
    webtrading.WithModule(webtrading.DefaultConfig()),
)
```

### 2. Подписка на шину событий
Для обработки жизненного цикла сессий и входящих пакетов используйте встроенную шину событий, исключающую блокировки основного потока:

```go
sub := client.Bus().Subscribe(&auth.LoggedOnEvent{})
go func() {
    for event := range sub.C() {
        if ev, ok := event.(*auth.LoggedOnEvent); ok {
            logger.Info("Сессия успешно установлена", log.Uint64("steam_id", ev.SteamID))
        }
    }
}()
```
