# Архитектура и принципы работы Citadel Sentry 🏗️

В данном документе подробно описано внутреннее устройство, схема компонентов, машина состояний сидинга и протоколы взаимодействия приложения **Citadel Sentry**.

---

## 1. Общая схема компонентов

Citadel Sentry построен по гибридной архитектуре: высокопроизводительный системный бэкенд на **Go** в связке с легковесным пользовательским интерфейсом на **Vue 3 / Nuxt 3 / Tailwind CSS**, объединёнными через легковесный нативный контейнер **Wails v2**.

```mermaid
graph TD
    subgraph UI ["Пользовательский интерфейс (Frontend - Nuxt 3)"]
        Dashboard["Главный экран (Dashboard)"]
        ServerFleet["Флот серверов (ServerList)"]
        SeedConfig["Модальное окно сидинга (SeedConfigModal)"]
        PiniaStores["Хранилища состояния (Pinia)"]
        Dashboard --> PiniaStores
        ServerFleet --> PiniaStores
        SeedConfig --> PiniaStores
    end

    subgraph Bridge ["Wails v2 IPC Bridge"]
        IPC["Двунаправленная шина событий (Wails Runtime IPC)"]
        PiniaStores <--> IPC
    end

    subgraph Backend ["Системное ядро (Backend - Go)"]
        App["App Controller (app.go)"]
        SeedEngine["Движок сидинга (Seed Engine)"]
        CitadelClient["Citadel API & A2S Client"]
        SettingsMgr["Менеджер конфигурации"]

        IPC <--> App
        App --> SeedEngine
        App --> CitadelClient
        App --> SettingsMgr
    end

    subgraph OS ["Взаимодействие с системой и игрой"]
        WinAdapter["Windows Adapter (Win32 API)"]
        LinuxAdapter["Linux Adapter (D-Bus / systemd)"]
        SteamClient["Steam Protocol (steam://joinlobby/...)"]
        SquadProcess["Процесс Squad (SquadGame.exe)"]

        SeedEngine --> WinAdapter
        SeedEngine --> LinuxAdapter
        WinAdapter --> SteamClient
        LinuxAdapter --> SteamClient
        SteamClient --> SquadProcess
    end
```

---

## 2. Машина состояний движка сидинга (Seed Engine State Machine)

Сердцем приложения является детерминированный конечный автомат (State Machine), реализованный в пакете `internal/seed/engine.go`.

```mermaid
stateDiagram-v2
    [*] --> Idle: Приложение запущено

    Idle --> Preparing: Пользователь нажал «Запустить сидинг»
    
    Preparing --> Connecting: Очередь сформирована, запуск игры
    Connecting --> Active: Запуск через Steam успешен, Squad запущен
    
    state Active {
        [*] --> PollingTelemetry: Каждые 5 сек
        PollingTelemetry --> CheckThreshold: Получение текущего онлайна
        CheckThreshold --> PollingTelemetry: Онлайн < Порога
    }

    Active --> Transitioning: Сервер заполнился (Порог достигнут) & В очереди есть ещё сервер
    Transitioning --> Connecting: Закрытие Squad -> Пауза -> Запуск следующего сервера

    Active --> Completed: Все серверы очереди заполнены
    Active --> TimedOut: Истек лимит времени / Наступил дедлайн по часам

    Active --> Failed: Игра закрыта игроком / Ошибка процесса
    Active --> Stopped: Игрок нажал «Отменить сидинг»

    TimedOut --> PostAction: Выполнение политики таймаута
    Completed --> PostAction: Выполнение политики завершения (Close / Sleep / Shutdown)

    PostAction --> Idle: Сессия завершена
    Stopped --> Idle: Сессия прервана
    Failed --> Idle: Ошибка зафиксирована
```

---

## 3. Схема безопасного переключения серверов (Queue Transition)

При сидировании очереди из нескольких серверов (например, 2 или 3 сервера подряд) Squad не умеет мгновенно и без сбоев переключаться между серверами из внешних ссылок без закрытия текущей сессии. 

Поэтому Citadel Sentry выполняет строго выверенную и безопасную последовательность:

```mermaid
sequenceDiagram
    autonumber
    participant Engine as Seed Engine (Go)
    participant OS as Платформенный адаптер (OS)
    participant Squad as SquadGame.exe
    participant Steam as Steam Client

    Note over Engine: Порог онлайна на Сервере 1 достигнут!
    Engine->>Engine: Смена статуса на Transitioning (Смена сервера)
    Engine->>OS: Запрос мягкого закрытия Squad (SIGTERM / WM_CLOSE)
    OS->>Squad: Сигнал закрытия окна игры
    
    loop Ожидание завершения (до 15 сек)
        OS->>Squad: Проверка наличия процесса SquadGame.exe
    end
    Note over OS,Squad: Процесс завершился штатно
    
    Engine->>Engine: Технологическая пауза (3 сек) для освобождения сокетов
    Engine->>OS: Запуск следующего сервера через Steam
    OS->>Steam: Вызов steam://joinlobby/393380/<ServerID2>/...
    Steam->>Squad: Чистый запуск SquadGame.exe с автоподключением
    Engine->>Engine: Смена статуса на Connecting -> Active
```

---

## 4. Безопасное управление питанием (Power Management)

Citadel Sentry защищает компьютер от случайных выключений или зависаний во время сидинга.

```mermaid
flowchart TD
    StartSession([Старт сидинга]) --> PreventSleep["Блокировка спящего режима ОС на время сида"]
    PreventSleep --> MonitorProgress{"Онлайн набран или таймаут?"}

    MonitorProgress -- Нет --> KeepAwake["Поддержание активности системы"]
    KeepAwake --> MonitorProgress

    MonitorProgress -- Да --> ReleaseSleepLock["Снятие блокировки сна"]
    ReleaseSleepLock --> CheckAction{"Какое действие выбрано?"}

    CheckAction -- "Остаться в игре (do_nothing)" --> DoneStay["Игра продолжается, ПК не трогается"]
    CheckAction -- "Закрыть Squad (close_squad)" --> DoneClose["Мягкое закрытие SquadGame.exe"]
    CheckAction -- "Спящий режим (sleep)" --> ExecSleep["Перевод ПК в системный спящий режим"]
    CheckAction -- "Выключить ПК (shutdown)" --> ExecShutdown["Корректное завершение работы ОС (Win32 / systemd)"]
```

### Реализация блокировки сна:
- **Windows**: Использование системного вызова `SetThreadExecutionState(ES_CONTINUOUS | ES_SYSTEM_REQUIRED | ES_AWAYMODE_REQUIRED)`. Это предотвращает засыпание экрана и переход системы в ждущий режим во время работы миссии.
- **Linux**: Использование системной шины `systemd-inhibit` (D-Bus) с блокировкой типа `sleep` и понятным описанием причины (`Citadel Sentry seed session`).

### Реализация выключения / сна:
- **Windows**: Вызов `ExitWindowsEx` с флагами `EWX_SHUTDOWN | EWX_POWEROFF` или `SetSuspendState` для сна. Все фоновые приложения получают стандартное системное уведомление о сохранении данных.
- **Linux**: Вызов штатных команд `systemctl poweroff` / `systemctl suspend` (или через D-Bus `org.freedesktop.login1.Manager`). Никаких опасных прямых сбросов питания (`kill -9 1` или аппаратных зависаний).
