# Руководство по запуску и установке Citadel Sentry на Linux 🐧

> [!WARNING]
> ### ⚠️ Внимание: Версия для Linux находится в стадии активного бета-тестирования (Public Beta)
> Версия **Citadel Sentry для Linux** полностью функциональна, но имеет статус **Beta**:
> - Запуск Squad происходит через окружение **Proton / Steam Play**.
> - Автоматический поиск путей к `GameUserSettings.ini` оптимизирован под стандартные пути Steam Proton (`compatdata/393380/pfx/...`), но при кастомных библиотеках Steam на других дисках может потребоваться ручная проверка.
> - Действия со спящим режимом и выключением ПК используют `systemctl` / `loginctl` и зависят от прав пользователя и настроек Polkit в конкретном дистрибутиве.
> - Если вы обнаружили неточность или баг в Linux-сборке — пожалуйста, напишите разработчикам в Discord сообщества Citadel!

---

В этом руководстве вы узнаете:
1. Как запустить бинарный файл Citadel Sentry на Linux одной командой.
2. Необходимые системные зависимости.
3. Как оформить Citadel Sentry как **полноценное Linux-приложение** (ярлык в меню системы, запуск из Dash/меню приложений, закрепление в панели задач).
4. Как собрать портативный **AppImage** или `.deb` пакет.

---

## 1. Быстрый запуск из консоли

Если у вас уже скачан файл `citadel-sentry` (или собран в папке `CitadelSeeder`):

```bash
# 1. Сделать файл исполняемым (если ещё не сделано)
chmod +x ./citadel-sentry

# 2. Запустить приложение
./citadel-sentry
```

### Системные зависимости:
Приложение построено на базе Wails v2 и использует современный движок **WebKit2GTK 4.1**. В большинстве современных дистрибутивов он уже предустановлен. Если при запуске возникла ошибка об отсутствии библиотеки, установите её:

- **Ubuntu 22.04+ / Linux Mint / Debian 12+**:
  ```bash
  sudo apt update
  sudo apt install libwebkit2gtk-4.1-0 libgtk-3-0
  ```

- **Arch Linux / Manjaro**:
  ```bash
  sudo pacman -S webkit2gtk-4.1 gtk3
  ```

- **Fedora 38+**:
  ```bash
  sudo dnf install webkit2gtk4.1 gtk3
  ```

---

## 2. Можно ли сделать Citadel Sentry полноценным Linux-приложением?

**Да, абсолютно!** В Linux стандартным и самым надежным способом создания полноценного десктопного приложения является стандарт **XDG Desktop Entry** (`.desktop` файл). 

После этого Citadel Sentry:
- Появится в общем меню приложений (GNOME, KDE Plasma, XFCE, Cinnamon).
- Будет искаться по имени через клавишу Super / Win.
- Сможет быть закреплен в доке (панели избранного) как Telegram, Steam или Discord.
- Будет открываться по обычному клику мышью без необходимости открывать терминал.

---

## 3. Пошаговая интеграция в меню приложений Linux

Мы подготовили простой рецепт установки для текущего пользователя:

### Шаг 1: Размещение бинарника и иконки
Рекомендуется поместить исполняемый файл в стандартную папку `~/.local/bin/` (она есть в переменной PATH каждого пользователя):

```bash
# Создаем папки пользователя
mkdir -p ~/.local/bin
mkdir -p ~/.local/share/applications
mkdir -p ~/.local/share/icons/hicolor/512x512/apps

# Копируем бинарник
cp ./citadel-sentry ~/.local/bin/citadel-sentry
chmod +x ~/.local/bin/citadel-sentry

# Копируем иконку приложения (логотип Sentry)
cp ./build/appicon.png ~/.local/share/icons/hicolor/512x512/apps/citadel-sentry.png
```

### Шаг 2: Создание файла ярлыка (.desktop)
Создайте файл `~/.local/share/applications/citadel-sentry.desktop`:

```ini
[Desktop Entry]
Type=Application
Name=Citadel Sentry
GenericName=Squad Auto-Seeder
Comment=Автоматический сидинг серверов Squad сообщества Citadel
Exec=/home/%u/.local/bin/citadel-sentry
Icon=citadel-sentry
Terminal=false
Categories=Game;Utility;
Keywords=squad;seeder;citadel;steam;
StartupNotify=true
StartupWMClass=citadel-sentry
```
*(Замените `/home/%u` на ваш домашний каталог, например `/home/geniz`)*.

### Шаг 3: Обновление базы меню приложений
Выполните команду:
```bash
update-desktop-database ~/.local/share/applications
```

Теперь откройте меню приложений вашей системы — **Citadel Sentry** готов к запуску!

---

## 4. Сборка портативного AppImage (Для раздачи другим пользователям)

Если вы хотите раздавать игрокам на Linux один файл, который запускается в 1 клик на **любом** дистрибутиве без установки каких-либо пакетов:

1. Установите утилиту `appimagetool`:
   ```bash
   wget https://github.com/AppImage/AppImageKit/releases/download/continuous/appimagetool-x86_64.AppImage
   chmod +x appimagetool-x86_64.AppImage
   ```

2. Подготовьте структуру `CitadelSentry.AppDir`:
   ```bash
   mkdir -p CitadelSentry.AppDir/usr/bin
   cp citadel-sentry CitadelSentry.AppDir/usr/bin/citadel-sentry
   cp build/appicon.png CitadelSentry.AppDir/citadel-sentry.png
   cp ~/.local/share/applications/citadel-sentry.desktop CitadelSentry.AppDir/
   
   # Создайте скрипт запуска AppRun
   cat << 'EOF' > CitadelSentry.AppDir/AppRun
   #!/bin/sh
   SELF=$(readlink -f "$0")
   HERE=${SELF%/*}
   exec "${HERE}/usr/bin/citadel-sentry" "$@"
   EOF
   chmod +x CitadelSentry.AppDir/AppRun
   ```

3. Соберите AppImage:
   ```bash
   ./appimagetool-x86_64.AppImage CitadelSentry.AppDir Citadel-Sentry-x86_64.AppImage
   ```

Полученный файл `Citadel-Sentry-x86_64.AppImage` можно отправить любому игроку на Linux — он запускается двойным кликом.

---

## 5. Особенности запуска Squad под Linux (через Steam Proton)

Игра **Squad** является Windows-игрой, запускаемой на Linux через встроенный в Steam слой совместимости **Proton**:
- Citadel Sentry при старте сидинга вызывает команду операционной системы:
  ```bash
  xdg-open "steam://joinlobby/393380/<ServerID>/<SteamID>"
  ```
- Системный демон Linux передает этот URL работающему Linux-клиенту Steam.
- Steam запускает Squad через ваш настроенный Proton (например, GE-Proton или Proton Experimental) с автоматическим подключением на сервер Citadel.
- Все функции отслеживания онлайна, таймаутов и перехода по очереди работают на Linux **точно так же, как и на Windows**.
