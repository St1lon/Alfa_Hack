-- 00002_seed_sources — стартовый набор внешних лент (критерий 05: непрерывный поток).
--
-- Каждая ссылка проверена вживую 20.10.2026: отдаёт RSS и парсится воркером.
-- Если лента начинает отдавать 403/404 или HTML вместо XML — выключаем флагом
-- enabled, а не удаляем строку: история парсинга остаётся воспроизводимой.
--
-- Три ленты (Коммерсантъ, Ведомости, РИА) заведены выключенными: с нашего
-- окружения они не отвечают (блокируют не-браузерный доступ). Оставлены в
-- таблице, чтобы включить одним UPDATE, если на внешнем сервере заработают.

INSERT INTO news_sources (code, title, kind, url, enabled, poll_interval_sec) VALUES
    ('rbc',        'РБК — новости',               'rss', 'https://rssexport.rbc.ru/rbcnews/news/30/full.rss', true,  300),
    ('interfax',   'Интерфакс',                   'rss', 'https://www.interfax.ru/rss.asp',                   true,  300),
    ('tass',       'ТАСС',                        'rss', 'https://tass.ru/rss/v2.xml',                        true,  300),
    ('lenta_eco',  'Лента.ру — экономика',        'rss', 'https://lenta.ru/rss/news/economics',               true,  600),
    ('finam',      'Финам — новости рынков',      'rss', 'https://www.finam.ru/net/analysis/conews/rsspoint', true,  600),
    ('finmarket',  'Финмаркет — главные новости', 'rss', 'https://www.finmarket.ru/rss/mainnews.asp',         true,  600),
    ('forbes',     'Forbes Россия',               'rss', 'https://www.forbes.ru/newrss.xml',                  true,  900),
    ('kommersant', 'Коммерсантъ — новости',       'rss', 'https://www.kommersant.ru/RSS/news.xml',            false, 300),
    ('vedomosti',  'Ведомости',                   'rss', 'https://www.vedomosti.ru/rss/news',                 false, 300),
    ('ria_eco',    'РИА Новости — экономика',     'rss', 'https://ria.ru/export/rss2/economy/index.xml',      false, 300)
ON CONFLICT (code) DO NOTHING;
