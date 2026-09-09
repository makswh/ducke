# Инструкция по установке и запуску Ducke на SteamOS 3.8+ (Steam Deck)

Ducke полностью оптимизирован для SteamOS 3.8.x и Steam Deck:
- **Разрешение экрана:** 1280x800 (16:10), нативное для экранов Steam Deck LCD и OLED.
- **Дисплейный стек:** Поддержка как Wayland (KDE Plasma 6 в Desktop Mode), так и X11 (Gamescope в Gaming Mode).
- **Аппаратное ускорение:** Включен режим `WebviewGpuPolicyAlways` для плавной работы интерфейса в 60/90 кадров/сек под Gamescope.
- **Поддержка контроллера:** Нативное управление геймпадом Steam Deck через `/dev/input` и `xdg-run/gamepad`.
- **Экранная клавиатура:** Автоматический вызов виртуальной клавиатуры SteamOS по нажатию `(X)`.
- **Файловая система:** Полный доступ к MicroSD (`/run/media/...`), внутренней памяти (`/home/deck`) и внешним накопителям.

---

## Вариант 1: Установка через Flatpak (Рекомендуемый для SteamOS)

Так как SteamOS имеет неизменяемую (read-only) системную файловую систему, установка через Flatpak — официальный и надежный способ Valve, который переживает обновления ОС и не требует отключения защиты `steamos-readonly`.

### Установка в один клик:
1. Скопируйте файлы `build/bin/Ducke.flatpak` и `build/bin/install_steamdeck.sh` на Steam Deck (в папку Загрузки или любую другую).
2. В **Desktop Mode** нажмите правой кнопкой на `install_steamdeck.sh` → **Run in Terminal** (или дважды кликните по `Ducke.flatpak`).
3. Скрипт сам настроит репозиторий Flathub, подтянет среду `org.gnome.Platform//46` и установит приложение.

### Установка вручную через терминал (Konsole):
```bash
flatpak remote-add --if-not-exists --user flathub https://dl.flathub.org/repo/flathub.flatpakrepo
flatpak mask --user org.freedesktop.Platform.openh264
flatpak install --user flathub org.gnome.Platform//46
flatpak install --user Ducke.flatpak
```

---

## Вариант 2: Добавление в Игровой режим (Gaming Mode)

1. Откройте **Steam** в **Desktop Mode**.
2. В верхнем меню нажмите **Игры (Games)** → **«Добавить стороннюю игру в мою библиотеку...» (Add a Non-Steam Game to My Library...)**.
3. Найдите в списке **Ducke** и нажмите **«Добавить выбранные»**.
4. Переключитесь в **Игровой режим (Gaming Mode)**.
5. Запускайте Ducke прямо из библиотеки Steam с полной поддержкой управления с геймпада!

---

## Сборка проекта (Windows / WSL)

Для сборки проекта на ПК в один клик:
- Запустите `build_linux.bat` из корня проекта на Windows.
- Скрипт автоматически соберет веб-интерфейс, скомпилирует бинарник с WebKitGTK 4.1 и упакует Flatpak-бандл в WSL.
