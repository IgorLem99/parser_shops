# Универсальный парсер магазинов

Собирает товары (название, цена, старая цена, цена за единицу, ссылка, картинка и любые свои поля) с каталогов интернет-магазинов.
Что и как собирать, описано в **`shops.yaml`**. Чтобы добавить новый магазин, допишите туда блок, код менять не нужно.

Страницы открываются в настоящем Chrome (chromedp), поэтому сайты, которые рисуют каталог через JavaScript, тоже работают.

## Быстрый старт

Нужны Go 1.23+ и Google Chrome (или Chromium).

```bash
go run ./cmd/parser -list                        # какие магазины есть в конфиге
go run ./cmd/parser                              # спарсить все магазины
go run ./cmd/parser -shop books_toscrape         # только один
go run ./cmd/parser -shop kuper_lenta -max-pages 3
go run ./cmd/parser -shop kuper_lenta -urls "https://kuper.ru/...,https://kuper.ru/..."
go run ./cmd/parser -shop kuper_lenta -files "*.html"   # офлайн: сохранённые страницы
```

Результат: `products_ДАТА_ВРЕМЯ.csv`, разделитель `;`, UTF-8 с BOM (открывается в Excel). Колонки:
Магазин, Категория, Наименование, Цена, Старая цена, Цена (текст), Цена за ед., Ссылка, Картинка и дальше ваши `extra`-поля.

## Как добавить магазин

```yaml
shops:
  - name: my_shop                     # имя для -shop
    urls:                             # стартовые страницы категорий
      - https://example.com/catalog/milk
    timeout: 60s                      # сколько ждать карточки на странице
    delay: 1s                         # пауза между страницами
    scroll: true                      # прокручивать, если товары подгружаются при скролле
    selectors:                        # CSS-селекторы внутри карточки товара
      card: ".product-card"           # обязательно
      name: ".title@title"            # обязательно; "@атрибут" — взять из атрибута
      price: ".price-current"
      old_price: ".price-old"
      unit: ".weight"
      link: "a.product-link"          # пусто, если сама карточка — ссылка <a>
      image: "img@src"
      remove: ".sr-only"              # что вырезать из текста (скрытые подписи)
      extra:                          # любые свои колонки
        brand: ".brand"
        sku: "[data-sku]@data-sku"
    category:                         # откуда взять название категории (одно из трёх)
      selector: "h1"
      # title_split: " в "            # часть <title> до разделителя
      # fixed: "Молоко"
    pagination:
      type: param                     # none | param | next
      param: page                     # param: ?page=2, ?page=3, ...
      # next: "a.pagination-next"     # next: идти по ссылке «Следующая»
      max_pages: 50
    blocks:                           # как распознать заглушку или блокировку
      - match: "Access denied|Доступ запрещён"
        message: "сайт заблокировал IP"
      - match: "captcha|пройдите проверку"
        message: "капча"
        manual: true                  # с -headless=false можно пройти её руками
```

Как подобрать селекторы: откройте категорию в Chrome, нажмите правой кнопкой на карточку товара → «Просмотреть код».
Если классы выглядят как `ProductCard_root__zO_B9` (хэш меняется при обновлениях сайта), используйте `[class*='ProductCard_root']`.

Пагинация останавливается сама, если очередная страница пустая или повторяет уже собранные товары, а также по `max_pages`.

## Флаги

| Флаг | Env | Описание |
|---|---|---|
| `-config` | `SHOPS_CONFIG` | файл конфига (по умолчанию `shops.yaml`) |
| `-shop` | `SHOP` | магазины через запятую (по умолчанию все) |
| `-urls` | `TARGET_URLS` | свои ссылки вместо `urls` из конфига (нужен один `-shop`) |
| `-files` | | офлайн-разбор сохранённых HTML |
| `-max-pages` | | ограничить пагинацию |
| `-proxy` | `PROXY_URL` | прокси для Chrome (`http://host:port`, `socks5://host:port`, без логина) |
| `-headless` | `HEADLESS=0` | `-headless=false` — показать окно браузера |
| `-profile` | | папка профиля Chrome с cookies (по умолчанию `.chrome-profile`) |
| `-out` | | имя CSV |

## Блокировки (Kuper)

Kuper пускает только российские IP: с других адресов он показывает «Отключите VPN». Иногда сайт показывает капчу.
Парсер распознаёт и то и другое по `blocks` и пишет понятную ошибку. Что делать:
* не из РФ: `-proxy` с российским прокси или запуск на российском сервере;
* капча: один раз запустить `-headless=false -shop kuper_lenta` и пройти её в окне. Cookies сохранятся в `.chrome-profile`.

## Docker

```bash
docker build -t shop-parser .
docker run --rm -v "$PWD:/data" shop-parser -shop books_toscrape
docker run --rm -v "$PWD:/data" -v "$PWD/shops.yaml:/etc/parser/shops.yaml" shop-parser   # свой конфиг
```

## Тесты

```bash
go test ./...
```
Тесты поднимают локальный «магазин» и проверяют пагинацию (`param` и `next`), извлечение полей, распознавание блокировки и разбор цен.

## Структура

* `cmd/parser`: точка входа, флаги
* `internal/config`: формат `shops.yaml`
* `internal/scraper`: движок (Chrome, ожидание карточек, прокрутка, пагинация), `extract.js` вытаскивает поля со страницы
* `internal/export`: CSV и разбор цен
* `cmd/categories`: дерево категорий Kuper через API (отдельная утилита)
