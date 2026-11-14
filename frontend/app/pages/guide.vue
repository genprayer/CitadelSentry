<script setup lang="ts">
import {
  BookOpen,
  Search,
  LogIn,
  Flame,
  CheckCircle2,
  Clock,
  Power,
  ShieldCheck,
  HelpCircle,
  ChevronDown,
  Layers
} from 'lucide-vue-next'
import BaseCard from '~/components/ui/BaseCard.vue'
import BaseInput from '~/components/ui/BaseInput.vue'
import BaseBadge from '~/components/ui/BaseBadge.vue'

interface GuideLegendItem {
  num: number
  title: string
  desc: string
  color?: 'gold' | 'sky' | 'purple' | 'green' | 'red'
}

interface GuideImage {
  src: string
  alt: string
  legend: GuideLegendItem[]
}

interface GuideItem {
  id: string
  category: 'basics' | 'potato' | 'joinVsSeed' | 'queue' | 'actions' | 'power' | 'safety'
  title: string
  summary: string
  content: string[]
  tips?: string[]
  badge?: string
  image?: GuideImage
}

const searchQuery = ref('')
const selectedCategory = ref<string>('all')
const expandedItems = ref<Record<string, boolean>>({
  'join-vs-seed': true,
  'target-timeout-explained': true,
  'potato-mode-explained': true,
  'recommended-settings': true,
  'power-safety-guarantee': true,
  'how-to-stop': true
})

const categories = [
  { id: 'all', labelKey: 'guide.categories.all' },
  { id: 'basics', labelKey: 'guide.categories.basics' },
  { id: 'potato', labelKey: 'potato.title' },
  { id: 'joinVsSeed', labelKey: 'guide.categories.joinVsSeed' },
  { id: 'queue', labelKey: 'guide.categories.queue' },
  { id: 'actions', labelKey: 'guide.categories.actions' },
  { id: 'power', labelKey: 'guide.categories.power' },
  { id: 'safety', labelKey: 'guide.categories.safety' }
]

const guideItems: GuideItem[] = [
  {
    id: 'join-vs-seed',
    category: 'joinVsSeed',
    title: 'В чём разница между кнопками «Войти» (Join) и «Сидить» (Seed)?',
    summary: 'Кнопка «Войти» предназначена для обычной игры в Squad, а «Сидить» запускает фоновую автоматизацию.',
    badge: 'Важно знать',
    content: [
      '**Кнопка «Войти» (Join)**: Прямое мгновенное подключение к серверу через протокол Steam. Программа отправляет команду клиенту Steam, запускает Squad и подключает вас к серверу для обычной игры. Если в этот момент был запущен фоновый авто-сидинг, программа предупредит вас и остановит его, чтобы освободить место для вашей игры.',
      '**Кнопка «Сидить» (Seed)**: Запуск автоматической миссии наполнения сервера игроками. Программа контролирует весь цикл: предотвращает засыпание операционной системы, отслеживает онлайн, а когда цель достигнута — может автоматически закрыть Squad, перевести компьютер в сон или выключить его (по вашему выбору).'
    ],
    tips: [
      'Если вы хотите просто поиграть на сервере — нажимайте «Войти».',
      'Если вы сидите в Discord, работаете или уходите спать — нажимайте «Сидить».'
    ],
    image: {
      src: '/img/guide/guide_01_join_vs_seed.png',
      alt: 'Сравнение кнопок «Войти» и «Сидить» на карточке сервера',
      legend: [
        {
          num: 1,
          color: 'sky',
          title: 'Кнопка «ВОЙТИ» (Steam Direct Join)',
          desc: 'Мгновенное подключение к серверу через протокол Steam для вашей личной игры. Если активен сидинг, появится диалог подтверждения.'
        },
        {
          num: 2,
          color: 'gold',
          title: 'Кнопка «СИДИТЬ» (Seed Mission)',
          desc: 'Открытие окна настройки автоматической фоновой миссии наполнения сервера с выбором целей онлайна, очереди и питания.'
        },
        {
          num: 3,
          color: 'purple',
          title: 'Кнопка «INFO» (Информация о сервере)',
          desc: 'Отображает подробную сводку: текущую карту, режим игры, правила сервера и ссылки на сообщество.'
        },
        {
          num: 4,
          color: 'green',
          title: 'Статус «ТРЕБУЕТСЯ СИДИНГ» и онлайн',
          desc: 'Динамический индикатор потребности сервера в игроках при онлайне ниже установленного порога (до 50 игроков).'
        }
      ]
    }
  },
  {
    id: 'potato-mode-explained',
    category: 'potato',
    title: 'Как работает «Мод картошки» и почему игра не грузит процессор?',
    summary: 'Снижает потребление видеокарты и процессора Squad почти до нуля, позволяя параллельно играть в другие игры.',
    badge: 'Новинка',
    content: [
      '**Что делает мод картошки?**',
      'Когда вы запускаете сидинг, игра Squad работает в фоновом режиме. Чтобы ваш компьютер не шумел, вентиляторы не крутились на максимум, а процессор и видеокарта оставались свободными, Citadel Sentry активирует специальный профиль оптимизации:',
      '• **Лимит 1 FPS в фоне**: Ограничивает частоту кадров Squad до 1 кадра в секунду. Это снижает энергопотребление видеокарты на 95–99%.',
      '• **Минимальное разрешение 3D-рендера**: Внутреннее разрешение снижается, разгружая видеопамять.',
      '• **Низкий приоритет CPU в Windows**: Процессу `SquadGame.exe` назначается приоритет ниже среднего, поэтому Windows отдает все ресурсы вашей основной игре или рабочим программам.',
      '**Сохраняются ли мои личные настройки?**',
      'Да! Citadel Sentry автоматически создает резервную копию вашего файла `GameUserSettings.ini` перед запуском сидинга. При завершении сидинга или закрытии программы ваши оригинальные настройки графики полностью восстанавливаются.',
      '**Затрагиваются ли настройки управления и чувствительность?**',
      '**Нет, никогда.** Файл управления и чувствительности мыши (`Input.ini`) никогда не читается и не модифицируется программой. Все ваши бинды клавиш, чувствительность и раскладка остаются в абсолютной безопасности.'
    ],
    tips: [
      'Вы можете в любой момент вручную восстановить оригинальную графику в разделе «Настройки» → «Мод картошки».',
      'С включенным модом картошки можно спокойно играть в Dota 2, CS2, Cyberpunk 2077 или работать — Squad не будет мешать.'
    ],
    image: {
      src: '/img/guide/guide_02_potato_mode.png',
      alt: 'Параметры мода картошки в настройках приложения',
      legend: [
        {
          num: 1,
          color: 'gold',
          title: 'Главный переключатель мода картошки',
          desc: 'Активирует фоновые энергосберегающие настройки графики Squad при запуске любой миссии авто-сидинга.'
        },
        {
          num: 2,
          color: 'sky',
          title: 'Лимит FPS в фоне (1, 5, 10, 15, 30 FPS)',
          desc: 'Выбор частоты кадров. Значение 1 FPS снижает энергопотребление и шум видеокарты на 95–99%.'
        },
        {
          num: 3,
          color: 'purple',
          title: 'Пониженный приоритет CPU процесса в Windows',
          desc: 'Процессу SquadGame.exe задается статус «Ниже среднего», отдавая процессорное время вашим активным программам и играм.'
        },
        {
          num: 4,
          color: 'green',
          title: '«Сохранить текущие настройки как основу»',
          desc: 'Перезаписывает базовый бэкап оригинальной графики из вашего GameUserSettings.ini после любых изменений в игре.'
        },
        {
          num: 5,
          color: 'gold',
          title: 'Гарантия безопасности Input.ini',
          desc: 'Файлы управления, чувствительности мыши и биндов клавиш никогда не считываются и не изменяются программой.'
        }
      ]
    }
  },
  {
    id: 'recommended-settings',
    category: 'potato',
    title: 'Рекомендуемые настройки для сидинга и фоновой работы',
    summary: 'Какой FPS выбрать, стоит ли трогать разрешение 3D и как безопасно обновить бэкап настроек.',
    badge: 'Рекомендации',
    content: [
      '**1. Какой лимит FPS выбрать?**',
      '• **1 FPS (по умолчанию)**: Идеально для работы за ПК, сёрфинга или ночного сидинга. Видеокарта потребляет менее 1% мощности, вентиляторы бесшумны.',
      '• **5–15 FPS**: Подходит, если вы хотите параллельно играть в другую нетребовательную игру или время от времени разворачивать Squad, чтобы проверить карту без задержек отклика интерфейса.',
      '• **30 FPS**: Минимальный комфорт для быстрого визуального контроля игры.',
      '**2. Пониженный приоритет CPU**',
      'Рекомендуется всегда держать **включенным**. Windows отдаст приоритет вашим активным программам (Discord, браузер, другие игры), а Squad получит только свободные кванты времени процессора.',
      '**3. Снижать ли разрешение 3D (3D Resolution / Render Scale)?**',
      'По умолчанию эта опция **выключена**. Ограничение частоты до 1 FPS само по себе снимает 99% нагрузки с видеокарты. Изменение разрешения в Unreal Engine может сбивать оконный режим и расположение окон на нескольких мониторах. Включайте эту опцию только на очень слабых видеокартах или интегрированной графике.',
      '**4. Как и когда обновлять бэкап настроек Squad?**',
      'Sentry автоматически архивирует ваш `GameUserSettings.ini` при первом запуске до включения оптимизаций. Если вы позже зашли в игру и изменили графику (например, угол обзора FOV, гамму или текстуры), просто перейдите в «Настройки» → «Мод картошки» и нажмите **«Сохранить текущие настройки как основу»**.'
    ],
    tips: [
      'Вы можете переключать FPS картошки прямо в модалке запуска сидинга (1, 5, 10, 15, 30 FPS).'
    ],
    image: {
      src: '/img/guide/guide_03_recommended_settings.png',
      alt: 'Рекомендуемые параметры в окне запуска сидинга',
      legend: [
        {
          num: 1,
          color: 'gold',
          title: 'Тумблер «Мод картошки» при старте',
          desc: 'Позволяет включить или отключить картофельные профили индивидуально для текущего запуска сидинга.'
        },
        {
          num: 2,
          color: 'sky',
          title: 'Фоновый лимит FPS (1–30 FPS)',
          desc: 'Рекомендуется оставлять 1 FPS для тихой фоновой работы или 5–10 FPS, если требуется периодически разворачивать игру.'
        },
        {
          num: 3,
          color: 'green',
          title: 'Действие по достижении цели онлайна',
          desc: 'Что сделать, когда цель достигнута: продолжить игру, закрыть Squad, перевести ПК в спящий режим или выключить.'
        },
        {
          num: 4,
          color: 'purple',
          title: 'Действие по истечении таймера',
          desc: 'Что предпринять, если время вышло: переключиться на следующий сервер из очереди или выполнить действие питания.'
        },
        {
          num: 5,
          color: 'gold',
          title: 'Кнопка «ЗАПУСТИТЬ СИДИНГ»',
          desc: 'Инициализация миссии, создание резервной копии настроек и запуск игры через защищенный протокол Steam.'
        }
      ]
    }
  },
  {
    id: 'power-safety-guarantee',
    category: 'power',
    title: 'Как устроена защита от случайного выключения или засыпания ПК?',
    summary: 'Многоуровневая система безопасности, которая исключает преждевременный сон или выключение компьютера.',
    badge: 'Защита питания',
    content: [
      'В Citadel Sentry внедрена надежная трёхуровневая система защиты от непреднамеренного отключения ПК:',
      '1. **Предстартовая проверка онлайна**: Если сервер уже набрал необходимое количество игроков до старта сидинга, переход в спящий режим или выключение автоматически блокируются.',
      '2. **Минимум 3 минуты реального сидинга**: Компьютер физически не может уснуть или выключиться, если сидинг длился меньше 3 минут. Если произошел сбой сети или сервер сразу наполнился, действие отменяется.',
      '3. **30-секундный таймер отмены**: Перед отправкой ПК в сон или выключением Citadel Sentry сначала закрывает Squad, а затем показывает на весь экран 30-секундный обратный отсчет. Если вы сидите за ПК, вы можете просто нажать кнопку «Отменить» (или клавишу Esc), и компьютер останется включенным.'
    ],
    tips: [
      'В Windows при отмене вызова немедленно выполняется системная команда `shutdown /a`, гарантированно сбрасывающая выключение.'
    ],
    image: {
      src: '/img/guide/guide_04_power_safety.png',
      alt: '30-секундное окно предупреждения перед выключением ПК',
      legend: [
        {
          num: 1,
          color: 'gold',
          title: 'Предупреждающий баннер безопасности',
          desc: 'Сообщает о скором завершении работы Windows и активации процедуры выключения по достижении цели сидинга.'
        },
        {
          num: 2,
          color: 'red',
          title: '30-секундный таймер обратного отсчета',
          desc: 'Наглядный визуальный счетчик и прогресс-бар, дающий достаточно времени для безопасной отмены действия.'
        },
        {
          num: 3,
          color: 'green',
          title: 'Кнопка «ОТМЕНИТЬ (ОСТАТЬСЯ В WINDOWS)»',
          desc: 'Мгновенно останавливает таймер, посылает в систему команду shutdown /a и оставляет ваш компьютер включенным.'
        }
      ]
    }
  },
  {
    id: 'target-timeout-explained',
    category: 'actions',
    title: 'Что означают «Цель по онлайну» и «Если время вышло»?',
    summary: 'Простыми словами о том, как программа понимает, когда сидинг успешен и что делать, если сервер пуст.',
    badge: 'Параметры сидинга',
    content: [
      '**Сколько игроков набрать (цель)**: Количество игроков (например, 50), при наборе которого сервер считается полностью засеянным и готовым к игре. Как только число игроков на сервере достигает этой планки, программа считает задачу выполненной.',
      '**Что сделать, когда цель достигнута (онлайн набран)**:',
      '• *Остаться в игре (играть дальше)*: Squad останется открытым, вы сможете продолжить игру на наполнившемся сервере.',
      '• *Закрыть игру Squad*: Игра автоматически закроется, ПК останется включенным.',
      '• *Перевести компьютер в спящий режим*: Игра закроется, а ПК уснет для экономии энергии.',
      '• *Выключить компьютер*: Игра закроется, и компьютер штатно завершит работу.',
      '**Сколько времени ждать (таймер)**: Максимальное время (в минутах), которое вы готовы выделить на один сервер (например, 60 мин).',
      '**Что сделать, если время вышло, а сервер не набрался**:',
      '• *Остановить сидинг (остаться в игре)*: Сид завершится, вы останетесь в игре.',
      '• *Перейти к следующему серверу из очереди*: Программа не станет тратить время на пустой сервер и сразу переподключится к следующему серверу, отмеченному в очереди.',
      '• *Закрыть Squad / Сон / Выключение*: Выполнит выбранное действие безопасности.'
    ],
    image: {
      src: '/img/guide/guide_05_seed_modal_targets_queue.png',
      alt: 'Настройка целей онлайна, таймеров и очереди серверов',
      legend: [
        {
          num: 1,
          color: 'gold',
          title: 'Цель по онлайну и быстрые пресеты',
          desc: 'Количество игроков, при достижении которого сервер считается засеянным (кнопки 25, 40, 50, 60, 75 игроков).'
        },
        {
          num: 2,
          color: 'sky',
          title: 'Лимит времени ожидания на один сервер',
          desc: 'Максимальная продолжительность ожидания в минутах (30, 45, 60, 90 мин) до автоматической смены сервера.'
        },
        {
          num: 3,
          color: 'purple',
          title: 'Остановка по локальному времени ПК',
          desc: 'Возможность указать точное время (ЧЧ:ММ), когда сидинг должен быть принудительно остановлен.'
        },
        {
          num: 4,
          color: 'green',
          title: 'Очередь серверов с номерами порядка',
          desc: 'Чекбоксы выбора серверов для непрерывного авто-сидинга по очереди (#1, #2...) с кнопками «Только этот» и «Выбрать все».'
        }
      ]
    }
  },
  {
    id: 'queue-checkboxes',
    category: 'queue',
    title: 'Как работает выбор серверов через чекбоксы (Очередь)?',
    summary: 'Вы можете отметить один или несколько серверов, и Sentry пройдет по ним по порядку.',
    badge: 'Мульти-сервер',
    content: [
      'В окне настройки сидинга каждый сервер имеет галочку (чекбокс).',
      'При выборе сервера рядом с ним появляется порядковый номер запуска: **#1**, **#2**, **#3** и т.д.',
      'Программа всегда начинает с сервера **#1**. Когда на нем наберется нужное число игроков (или сработает переход по таймауту), Sentry закроет Squad и автоматически подключит вас к серверу **#2**.',
      'Быстрые кнопки: **«Только этот»** оставит только тот сервер, с которого вы открыли окно; **«Выбрать все»** добавит в очередь все доступные серверы сообщества.'
    ],
    tips: [
      'Действия с питанием ПК (выключение или спящий режим) сработают только **после завершения всей очереди**, а не после первого сервера!'
    ]
  },
  {
    id: 'night-seeding',
    category: 'power',
    title: 'Как настроить сидинг на ночь перед сном?',
    summary: 'Пошаговый сценарий ночного авто-сидинга с выключением компьютера.',
    badge: 'Сценарий',
    content: [
      '1. Откройте окно сидинга на нужном сервере.',
      '2. Если хотите помочь нескольким серверам — отметьте их галочками в очереди.',
      '3. Установите цель онлайна (рекомендуется 50–55 игроков).',
      '4. Установите лимит времени (например, 60 или 90 минут).',
      '5. В поле **«Когда цель достигнута»** выберите **«Выключить компьютер»** (или «Перевести в спящий режим»).',
      '6. Нажмите **«Запустить сидинг»** и можете идти спать.',
      'Когда серверы наберут онлайн или выйдет время, программа закроет игру и штатно завершит работу Windows/Linux.'
    ],
    tips: [
      'Программа предотвращает случайный сон ОС во время активного сидинга, поэтому игра не зависнет посреди ночи.'
    ]
  },
  {
    id: 'anti-cheat-safety',
    category: 'safety',
    title: 'Могут ли забанить в Steam или Easy Anti-Cheat (EAC)?',
    summary: 'Citadel Sentry полностью безопасен и не вмешивается в процесс игры.',
    badge: '100% Безопасно',
    content: [
      '**Бан полностью исключен.**',
      'Citadel Sentry не является читом, трейнером или модом:',
      '• Программа **не внедряет** библиотеки DLL в процесс `SquadGame.exe`.',
      '• Программа **не читает и не изменяет** оперативную память игры.',
      '• Подключение к серверу происходит исключительно через официальный протокол Steam (`steam://joinlobby/...`). Для античита EAC и системы Valve это абсолютно неотличимо от клика по ссылке в браузере или нажатия «Присоединиться к другу» в списке друзей Steam.'
    ]
  },

  {
    id: 'how-to-stop',
    category: 'basics',
    title: 'Как безопасно остановить или отменить сидинг?',
    summary: 'Вы сохраняете полный контроль над своим компьютером в любой момент.',
    content: [
      'Остановить сидинг можно двумя простыми способами:',
      '1. **Кнопка в окне Sentry**: Нажмите красную кнопку **«Остановить»** на карточке сервера или кнопку **«Отменить сидинг»** на главном экране.',
      '2. **Просто закройте Squad**: Если вы выйдете из игры через игровое меню или нажмете Alt+F4, Sentry мгновенно зафиксирует закрытие игры и мягко остановит миссию, не предпринимая никаких резких действий с питанием компьютера.'
    ],
    image: {
      src: '/img/guide/guide_06_safe_stop.png',
      alt: 'Окно подтверждения остановки сидинга при переходе в игру',
      legend: [
        {
          num: 1,
          color: 'gold',
          title: 'Предупреждение об активном сидинге',
          desc: 'Оповещает, что в данный момент уже запущена фоновая миссия сидинга на одном из серверов сообщества.'
        },
        {
          num: 2,
          color: 'sky',
          title: 'Карточка целевого сервера для входа',
          desc: 'Показывает сервер, на который вы нажали кнопку «Войти», его название, текущую карту и онлайн.'
        },
        {
          num: 3,
          color: 'red',
          title: 'Кнопка «Войти в игру (остановить сид)»',
          desc: 'Безопасно прекращает фоновый сидинг, восстанавливает настройки графики и подключает вас к серверу как обычного игрока.'
        },
        {
          num: 4,
          color: 'purple',
          title: 'Кнопка «Отмена»',
          desc: 'Закрывает модальное окно без изменений, продолжая фоновый сидинг текущего сервера.'
        }
      ]
    }
  }
]

const filteredItems = computed(() => {
  const query = searchQuery.value.trim().toLowerCase()
  return guideItems.filter((item) => {
    const matchesCategory = selectedCategory.value === 'all' || item.category === selectedCategory.value
    if (!matchesCategory) return false

    if (!query) return true
    const inTitle = item.title.toLowerCase().includes(query)
    const inSummary = item.summary.toLowerCase().includes(query)
    const inContent = item.content.some((line) => line.toLowerCase().includes(query))
    return inTitle || inSummary || inContent
  })
})

const toggleItem = (id: string) => {
  expandedItems.value[id] = !expandedItems.value[id]
}
</script>

<template>
  <div class="space-y-6 max-w-5xl mx-auto font-sans pb-12">
    <!-- Header -->
    <div class="flex items-center justify-between pb-3 border-b border-accent-secondary/20">
      <div>
        <h2 class="text-xl sm:text-2xl font-bold uppercase tracking-wide text-primary flex items-center space-x-2.5">
          <BookOpen class="w-6 h-6 text-accent" />
          <span>{{ $t('guide.title') }}</span>
        </h2>
        <p class="text-sm text-primary/70 mt-1">
          {{ $t('guide.subtitle') }}
        </p>
      </div>
    </div>

    <!-- Search & Filters -->
    <div class="space-y-4 bg-surface p-5 rounded-xl border border-accent-secondary/20">
      <BaseInput
        v-model="searchQuery"
        :placeholder="$t('guide.searchPlaceholder')"
        class="w-full text-sm"
      >
        <template #prefix>
          <Search class="w-5 h-5 text-primary/40 ml-2" />
        </template>
      </BaseInput>

      <!-- Category Filter Pills -->
      <div class="flex items-center flex-wrap gap-2 text-sm pt-1">
        <button
          v-for="cat in categories"
          :key="cat.id"
          type="button"
          :class="[
            'px-3.5 py-1.5 rounded-lg border transition-all',
            selectedCategory === cat.id
              ? 'bg-accent/20 text-accent border-accent/50 font-bold shadow-sm'
              : 'bg-surface/80 text-primary/75 border-accent-secondary/20 hover:text-primary hover:border-accent-secondary/50'
          ]"
          @click="selectedCategory = cat.id"
        >
          {{ $t(cat.labelKey) }}
        </button>
      </div>
    </div>

    <!-- Guide Items List -->
    <div v-if="filteredItems.length > 0" class="space-y-4">
      <BaseCard
        v-for="item in filteredItems"
        :key="item.id"
        variant="panel"
        padding="none"
        class="overflow-hidden border-accent-secondary/20 hover:border-accent/40 transition-colors"
      >
        <!-- Card Header (Click to toggle) -->
        <button
          type="button"
          class="w-full p-5 flex items-center justify-between text-left space-x-4 select-none hover:bg-surface/50 transition-colors"
          @click="toggleItem(item.id)"
        >
          <div class="space-y-1.5 min-w-0 flex-1">
            <div class="flex items-center space-x-2.5 flex-wrap gap-y-1">
              <BaseBadge v-if="item.badge" variant="brand" size="md">
                {{ item.badge }}
              </BaseBadge>
              <h3 class="text-base sm:text-lg font-bold text-primary">
                {{ item.title }}
              </h3>
            </div>
            <p class="text-sm text-primary/70">
              {{ item.summary }}
            </p>
          </div>

          <div class="flex-shrink-0 text-primary/50">
            <ChevronDown
              :class="[
                'w-5 h-5 transition-transform duration-200',
                expandedItems[item.id] ? 'rotate-180 text-accent' : ''
              ]"
            />
          </div>
        </button>

        <!-- Expanded Content -->
        <div
          v-if="expandedItems[item.id]"
          class="px-6 pb-6 pt-2 border-t border-accent-secondary/15 space-y-3.5 text-sm sm:text-base text-primary/90 leading-relaxed bg-surface/30"
        >
          <div
            v-for="(para, idx) in item.content"
            :key="idx"
            class="space-y-1"
          >
            <p v-html="para.replace(/\*\*(.*?)\*\*/g, '<strong class=\'text-primary font-bold\'>$1</strong>').replace(/\*(.*?)\*/g, '<em class=\'text-accent font-mono\'>$1</em>')" />
          </div>

          <!-- Helpful Tips Box -->
          <div
            v-if="item.tips && item.tips.length > 0"
            class="p-4 mt-3.5 rounded-xl bg-emerald-500/10 border border-emerald-500/30 text-emerald-200 text-sm font-sans space-y-1.5"
          >
            <p class="font-bold flex items-center space-x-2 text-emerald-300">
              <CheckCircle2 class="w-4 h-4 flex-shrink-0" />
              <span>Полезный совет:</span>
            </p>
            <ul class="list-disc list-inside space-y-1 text-emerald-100 pl-1">
              <li v-for="(tip, tipIdx) in item.tips" :key="tipIdx">
                {{ tip }}
              </li>
            </ul>
          </div>

          <!-- Showcase Image & Numbered Legend -->
          <div v-if="item.image" class="mt-5 pt-4 border-t border-accent-secondary/20 space-y-4">
            <!-- Image Frame -->
            <div class="rounded-xl overflow-hidden border border-accent-secondary/30 bg-black/40 shadow-xl p-2 sm:p-3">
              <img
                :src="item.image.src"
                :alt="item.image.alt"
                class="w-full h-auto rounded-lg object-contain block mx-auto border border-white/5"
              />
            </div>

            <!-- Numbered Legend -->
            <div
              v-if="item.image.legend && item.image.legend.length > 0"
              class="p-4 rounded-xl bg-surface/80 border border-accent-secondary/25 space-y-3"
            >
              <div class="flex items-center space-x-2 text-xs uppercase tracking-wider font-bold text-accent font-sans">
                <Layers class="w-4 h-4" />
                <span>Обозначения на схеме</span>
              </div>

              <div class="grid grid-cols-1 md:grid-cols-2 gap-2.5">
                <div
                  v-for="leg in item.image.legend"
                  :key="leg.num"
                  class="flex items-start space-x-3 p-3 rounded-lg bg-background/60 border border-accent-secondary/15 hover:border-accent/30 transition-colors"
                >
                  <span
                    :class="[
                      'w-6 h-6 rounded-full flex items-center justify-center font-bold text-xs font-mono flex-shrink-0 mt-0.5 shadow-sm',
                      leg.color === 'gold' ? 'bg-amber-500/20 text-amber-400 border border-amber-500/50' :
                      leg.color === 'sky' ? 'bg-sky-500/20 text-sky-400 border border-sky-500/50' :
                      leg.color === 'purple' ? 'bg-purple-500/20 text-purple-400 border border-purple-500/50' :
                      leg.color === 'green' ? 'bg-emerald-500/20 text-emerald-400 border border-emerald-500/50' :
                      leg.color === 'red' ? 'bg-red-500/20 text-red-400 border border-red-500/50' :
                      'bg-accent/20 text-accent border border-accent/40'
                    ]"
                  >
                    {{ leg.num }}
                  </span>
                  <div class="space-y-0.5 min-w-0">
                    <p class="text-xs sm:text-sm font-bold text-primary font-sans leading-snug">
                      {{ leg.title }}
                    </p>
                    <p class="text-xs text-primary/75 leading-relaxed font-sans">
                      {{ leg.desc }}
                    </p>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </BaseCard>
    </div>

    <!-- Empty State -->
    <div
      v-else
      class="p-12 text-center bg-surface rounded-xl border border-accent-secondary/20 space-y-2"
    >
      <p class="text-base font-bold text-primary font-sans">
        Ничего не найдено
      </p>
      <p class="text-sm text-primary/60">
        Попробуйте изменить поисковый запрос или выбрать категорию «Все темы».
      </p>
      <button
        type="button"
        class="mt-2 px-4 py-2 rounded-lg bg-accent/20 text-accent text-sm font-medium hover:bg-accent/30 transition-colors"
        @click="searchQuery = ''; selectedCategory = 'all'"
      >
        Сбросить поиск
      </button>
    </div>
  </div>
</template>
