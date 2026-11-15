BEGIN;

-- Apps: 42 total. 999001 has no name and 999002 has no header image so the
-- table's fallback rendering is visible. 10 and 80 share a name to exercise
-- deterministic name-then-ID ordering.
INSERT INTO apps (steam_appid, name, header_image, created_at, updated_at) VALUES
    (730,     'Counter-Strike 2',                'https://cdn.cloudflare.steamstatic.com/steam/apps/730/header.jpg',     now(), now()),
    (570,     'Dota 2',                          'https://cdn.cloudflare.steamstatic.com/steam/apps/570/header.jpg',     now(), now()),
    (440,     'Team Fortress 2',                 'https://cdn.cloudflare.steamstatic.com/steam/apps/440/header.jpg',     now(), now()),
    (220,     'Half-Life 2',                     'https://cdn.cloudflare.steamstatic.com/steam/apps/220/header.jpg',     now(), now()),
    (400,     'Portal',                          'https://cdn.cloudflare.steamstatic.com/steam/apps/400/header.jpg',     now(), now()),
    (620,     'Portal 2',                        'https://cdn.cloudflare.steamstatic.com/steam/apps/620/header.jpg',     now(), now()),
    (10,      'Counter-Strike',                  'https://cdn.cloudflare.steamstatic.com/steam/apps/10/header.jpg',      now(), now()),
    (80,      'Counter-Strike',                  'https://cdn.cloudflare.steamstatic.com/steam/apps/80/header.jpg',      now(), now()),
    (292030,  'The Witcher 3: Wild Hunt',        'https://cdn.cloudflare.steamstatic.com/steam/apps/292030/header.jpg',  now(), now()),
    (1091500, 'Cyberpunk 2077',                  'https://cdn.cloudflare.steamstatic.com/steam/apps/1091500/header.jpg', now(), now()),
    (1086940, 'Baldur''s Gate 3',                'https://cdn.cloudflare.steamstatic.com/steam/apps/1086940/header.jpg', now(), now()),
    (1245620, 'ELDEN RING',                      'https://cdn.cloudflare.steamstatic.com/steam/apps/1245620/header.jpg', now(), now()),
    (2358720, 'Black Myth: Wukong',              'https://cdn.cloudflare.steamstatic.com/steam/apps/2358720/header.jpg', now(), now()),
    (2246340, 'Monster Hunter Wilds',            'https://cdn.cloudflare.steamstatic.com/steam/apps/2246340/header.jpg', now(), now()),
    (1903340, 'Clair Obscur: Expedition 33',     'https://cdn.cloudflare.steamstatic.com/steam/apps/1903340/header.jpg', now(), now()),
    (990080,  'Hogwarts Legacy',                 'https://cdn.cloudflare.steamstatic.com/steam/apps/990080/header.jpg',  now(), now()),
    (1174180, 'Red Dead Redemption 2',           'https://cdn.cloudflare.steamstatic.com/steam/apps/1174180/header.jpg', now(), now()),
    (271590,  'Grand Theft Auto V',              'https://cdn.cloudflare.steamstatic.com/steam/apps/271590/header.jpg',  now(), now()),
    (578080,  'PUBG: BATTLEGROUNDS',             'https://cdn.cloudflare.steamstatic.com/steam/apps/578080/header.jpg',  now(), now()),
    (252490,  'Rust',                            'https://cdn.cloudflare.steamstatic.com/steam/apps/252490/header.jpg',  now(), now()),
    (413150,  'Stardew Valley',                  'https://cdn.cloudflare.steamstatic.com/steam/apps/413150/header.jpg',  now(), now()),
    (105600,  'Terraria',                        'https://cdn.cloudflare.steamstatic.com/steam/apps/105600/header.jpg',  now(), now()),
    (322330,  'Don''t Starve Together',          'https://cdn.cloudflare.steamstatic.com/steam/apps/322330/header.jpg',  now(), now()),
    (892970,  'Valheim',                         'https://cdn.cloudflare.steamstatic.com/steam/apps/892970/header.jpg',  now(), now()),
    (1145360, 'Hades',                           'https://cdn.cloudflare.steamstatic.com/steam/apps/1145360/header.jpg', now(), now()),
    (632360,  'Risk of Rain 2',                  'https://cdn.cloudflare.steamstatic.com/steam/apps/632360/header.jpg',  now(), now()),
    (251570,  '7 Days to Die',                   'https://cdn.cloudflare.steamstatic.com/steam/apps/251570/header.jpg',  now(), now()),
    (294100,  'RimWorld',                        'https://cdn.cloudflare.steamstatic.com/steam/apps/294100/header.jpg',  now(), now()),
    (427520,  'Factorio',                        'https://cdn.cloudflare.steamstatic.com/steam/apps/427520/header.jpg',  now(), now()),
    (289070,  'Sid Meier''s Civilization VI',    'https://cdn.cloudflare.steamstatic.com/steam/apps/289070/header.jpg',  now(), now()),
    (236390,  'War Thunder',                     'https://cdn.cloudflare.steamstatic.com/steam/apps/236390/header.jpg',  now(), now()),
    (550,     'Deep Rock Galactic',              'https://cdn.cloudflare.steamstatic.com/steam/apps/550/header.jpg',     now(), now()),
    (646570,  'Slay the Spire',                  'https://cdn.cloudflare.steamstatic.com/steam/apps/646570/header.jpg',  now(), now()),
    (588650,  'Dead Cells',                      'https://cdn.cloudflare.steamstatic.com/steam/apps/588650/header.jpg',  now(), now()),
    (367520,  'Hollow Knight',                   'https://cdn.cloudflare.steamstatic.com/steam/apps/367520/header.jpg',  now(), now()),
    (391540,  'Undertale',                       'https://cdn.cloudflare.steamstatic.com/steam/apps/391540/header.jpg',  now(), now()),
    (268910,  'Cuphead',                         'https://cdn.cloudflare.steamstatic.com/steam/apps/268910/header.jpg',  now(), now()),
    (814380,  'Sekiro: Shadows Die Twice',       'https://cdn.cloudflare.steamstatic.com/steam/apps/814380/header.jpg',  now(), now()),
    (374320,  'DARK SOULS III',                  'https://cdn.cloudflare.steamstatic.com/steam/apps/374320/header.jpg',  now(), now()),
    (255710,  'Cities: Skylines',                'https://cdn.cloudflare.steamstatic.com/steam/apps/255710/header.jpg',  now(), now()),
    (108600,  'Project Zomboid',                 'https://cdn.cloudflare.steamstatic.com/steam/apps/108600/header.jpg',  now(), now()),
    (493340,  'Planet Coaster',                  'https://cdn.cloudflare.steamstatic.com/steam/apps/493340/header.jpg',  now(), now()),
    (466560,  'Northgard',                       'https://cdn.cloudflare.steamstatic.com/steam/apps/466560/header.jpg',  now(), now()),
    (386360,  'SMITE',                           'https://cdn.cloudflare.steamstatic.com/steam/apps/386360/header.jpg',  now(), now()),
    (218620,  'PAYDAY 2',                        'https://cdn.cloudflare.steamstatic.com/steam/apps/218620/header.jpg',  now(), now()),
    (304930,  'Unturned',                        'https://cdn.cloudflare.steamstatic.com/steam/apps/304930/header.jpg',  now(), now()),
    (359550,  'Tom Clancy''s Rainbow Six Siege', 'https://cdn.cloudflare.steamstatic.com/steam/apps/359550/header.jpg',  now(), now()),
    (304390,  'For Honor',                       'https://cdn.cloudflare.steamstatic.com/steam/apps/304390/header.jpg',  now(), now()),
    (582010,  'Monster Hunter: World',           'https://cdn.cloudflare.steamstatic.com/steam/apps/582010/header.jpg',  now(), now()),
    (601150,  'Devil May Cry 5',                 'https://cdn.cloudflare.steamstatic.com/steam/apps/601150/header.jpg',  now(), now()),
    (761890,  'Albion Online',                   'https://cdn.cloudflare.steamstatic.com/steam/apps/761890/header.jpg',  now(), now()),
    (1172620, 'Sea of Thieves',                  'https://cdn.cloudflare.steamstatic.com/steam/apps/1172620/header.jpg', now(), now()),
    (1326470, 'Sons of the Forest',              'https://cdn.cloudflare.steamstatic.com/steam/apps/1326470/header.jpg', now(), now()),
    (1966720, 'Lethal Company',                  'https://cdn.cloudflare.steamstatic.com/steam/apps/1966720/header.jpg', now(), now()),
    (2050650, 'V Rising',                        'https://cdn.cloudflare.steamstatic.com/steam/apps/2050650/header.jpg', now(), now()),
    (1817070, 'Marvel''s Spider-Man Remastered', 'https://cdn.cloudflare.steamstatic.com/steam/apps/1817070/header.jpg', now(), now()),
    (1237970, 'Titanfall 2',                     'https://cdn.cloudflare.steamstatic.com/steam/apps/1237970/header.jpg', now(), now()),
    (1196590, 'Resident Evil Village',           'https://cdn.cloudflare.steamstatic.com/steam/apps/1196590/header.jpg', now(), now()),
    (1426210, 'It Takes Two',                    'https://cdn.cloudflare.steamstatic.com/steam/apps/1426210/header.jpg', now(), now()),
    (1544020, 'Total War: WARHAMMER III',        'https://cdn.cloudflare.steamstatic.com/steam/apps/1544020/header.jpg', now(), now()),
    (883710,  'Resident Evil 2',                 'https://cdn.cloudflare.steamstatic.com/steam/apps/883710/header.jpg',  now(), now()),
    (999001,  NULL,                              NULL, now(), now()),
    (999002,  'Untitled Prototype',              NULL, now(), now());

-- Every app has a self-alias.
INSERT INTO app_alias (app_id, alias_id, created_at, updated_at)
SELECT steam_appid, steam_appid, now(), now() FROM apps;

-- Aliases that resolve to the same canonical app. The removed rows and the
-- second active alias exist to prove owned/wishlist counts and lists dedupe.
INSERT INTO app_alias (app_id, alias_id, created_at, updated_at) VALUES
    (730,    999730, now(), now()),
    (730,    999731, now(), now()),
    (570,    999570, now(), now()),
    (80,     999080, now(), now()),
    (367520, 999367, now(), now()),
    (367520, 999368, now(), now()),
    (367520, 999369, now(), now());

-- Users. CaseTwin and casetwin differ only by case; CaseTwin has the newer
-- updated_at and is the one View User must resolve.
INSERT INTO steam_users (steam_id, name, wishlist_private, created_at, updated_at) VALUES
    (76561198000000001, 'SomePlayer',      false, now() - interval '30 days', now() - interval '2 hours'),
    (76561198000000002, 'PlayerTwo',       false, now() - interval '20 days', now() - interval '5 minutes'),
    (76561198000000003, 'Alice',           false, now() - interval '90 days', now() - interval '20 minutes'),
    (76561198000000004, 'BobTheBuilder',   false, now() - interval '45 days', now() - interval '1 day'),
    (76561198000000005, 'CaseTwin',        false, now() - interval '60 days', now() - interval '1 hour'),
    (76561198000000006, 'casetwin',        false, now() - interval '70 days', now() - interval '3 days'),
    (76561198000000007, 'EmptyUser',       false, now() - interval '2 days',  now() - interval '2 days'),
    (76561198000000008, 'WishlistOnly',    false, now() - interval '15 days', now() - interval '6 hours'),
    (76561198000000009, 'PrivateWishlist', true,  now() - interval '5 days',  now() - interval '8 hours');

-- SomePlayer owns 34 canonical apps (page 2 at 25 per page) plus two extra
-- aliases for app 730: one active duplicate and one removed.
INSERT INTO steam_user_apps (steam_id, app_id, removed, created_at, updated_at)
SELECT 76561198000000001, alias_id, removed, now(), now()
FROM (VALUES
    (730, false), (570, false), (440, false), (220, false), (400, false),
    (620, false), (10, false), (80, false), (292030, false), (1091500, false),
    (1086940, false), (1245620, false), (2358720, false), (2246340, false),
    (1903340, false), (990080, false), (1174180, false), (271590, false),
    (578080, false), (252490, false), (413150, false), (105600, false),
    (322330, false), (892970, false), (1145360, false), (632360, false),
    (251570, false), (294100, false), (427520, false), (550, false),
    (646570, false), (588650, false), (999001, false), (999002, false),
    (999730, false), (999731, true)
) AS v(alias_id, removed);

INSERT INTO steam_user_apps (steam_id, app_id, removed, created_at, updated_at)
SELECT 76561198000000002, alias_id, removed, now(), now()
FROM (VALUES
    (730, false), (570, false), (440, false), (292030, false),
    (1091500, false), (413150, false), (105600, false), (550, false),
    (999570, true)
) AS v(alias_id, removed);

INSERT INTO steam_user_apps (steam_id, app_id, removed, created_at, updated_at)
SELECT 76561198000000003, alias_id, false, now(), now()
FROM (VALUES (220), (400), (620), (1245620), (1086940)) AS v(alias_id);

INSERT INTO steam_user_apps (steam_id, app_id, removed, created_at, updated_at)
SELECT 76561198000000004, alias_id, removed, now(), now()
FROM (VALUES (10, false), (80, false), (322330, false), (999080, true)) AS v(alias_id, removed);

INSERT INTO steam_user_apps (steam_id, app_id, removed, created_at, updated_at)
SELECT 76561198000000005, alias_id, false, now(), now()
FROM (VALUES (730), (570)) AS v(alias_id);

INSERT INTO steam_user_apps (steam_id, app_id, removed, created_at, updated_at)
SELECT 76561198000000006, alias_id, false, now(), now()
FROM (VALUES (440), (220), (400), (620), (10), (80), (292030), (1091500), (1086940)) AS v(alias_id);

INSERT INTO steam_user_apps (steam_id, app_id, removed, created_at, updated_at)
SELECT 76561198000000009, 730, false, now(), now();

-- SomePlayer's wishlist has 27 canonical apps, including three alias rows for
-- 367520 where two are active (deduped) and one is removed.
INSERT INTO steam_user_wishlist (steam_id, app_id, date_added, removed, created_at, updated_at)
SELECT 76561198000000001, alias_id, now() - interval '10 days', removed, now(), now()
FROM (VALUES
    (367520, false), (391540, false), (268910, false), (814380, false),
    (374320, false), (255710, false), (108600, false), (493340, false),
    (466560, false), (386360, false), (218620, false), (304930, false),
    (359550, false), (304390, false), (582010, false), (601150, false),
    (761890, false), (1172620, false), (1326470, false), (1966720, false),
    (2050650, false), (1817070, false), (1237970, false), (1196590, false),
    (1426210, false), (1544020, false), (883710, false),
    (999367, false), (999368, true), (999369, false)
) AS v(alias_id, removed);

INSERT INTO steam_user_wishlist (steam_id, app_id, date_added, removed, created_at, updated_at)
SELECT 76561198000000002, alias_id, now() - interval '3 days', false, now(), now()
FROM (VALUES (367520), (391540), (108600), (761890)) AS v(alias_id);

INSERT INTO steam_user_wishlist (steam_id, app_id, date_added, removed, created_at, updated_at)
SELECT 76561198000000003, alias_id, now() - interval '1 day', false, now(), now()
FROM (VALUES (268910), (814380)) AS v(alias_id);

INSERT INTO steam_user_wishlist (steam_id, app_id, date_added, removed, created_at, updated_at)
SELECT 76561198000000008, alias_id, now() - interval '4 days', false, now(), now()
FROM (VALUES (367520), (391540), (268910), (814380), (374320), (255710)) AS v(alias_id);

-- Import jobs. SomePlayer and Bob have an older job so the UI demonstrates
-- that "latest" means most recently created, not most recently finished.
INSERT INTO import_jobs (
    steam_id, status, total_apps, processed_apps, skipped_apps, failed_apps,
    wishlist_total_apps, wishlist_processed_apps, wishlist_skipped_apps, wishlist_failed_apps,
    error, created_at, started_at, finished_at
) VALUES
    (76561198000000001, 'failed', 30, 27, 2, 1, 24, 20, 3, 1,
     'Steam returned an unexpected response while importing app details.',
     now() - interval '3 days', now() - interval '3 days', now() - interval '3 days' + interval '2 minutes'),
    (76561198000000001, 'succeeded', 34, 34, 0, 0, 27, 27, 0, 0,
     NULL, now() - interval '3 hours', now() - interval '3 hours', now() - interval '2 hours'),
    (76561198000000002, 'queued', 0, 0, 0, 0, 0, 0, 0, 0,
     NULL, now() - interval '5 minutes', NULL, NULL),
    (76561198000000003, 'running', 7, 3, 0, 0, 2, 1, 0, 0,
     NULL, now() - interval '20 minutes', now() - interval '19 minutes', NULL),
    (76561198000000004, 'failed', 4, 0, 0, 0, 0, 0, 0, 0,
     'Steam API rate limit exceeded while importing user details.',
     now() - interval '1 day', now() - interval '1 day', now() - interval '1 day' + interval '30 seconds'),
    (76561198000000004, 'succeeded', 3, 3, 0, 0, 0, 0, 0, 0,
     NULL, now() - interval '2 days', now() - interval '2 days', now() - interval '2 days' + interval '1 minute'),
    (76561198000000005, 'succeeded', 2, 2, 0, 0, 0, 0, 0, 0,
     NULL, now() - interval '2 hours', now() - interval '2 hours', now() - interval '1 hour'),
    (76561198000000006, 'failed', 9, 4, 1, 1, 0, 0, 0, 0,
     'The Steam profile could not be read.',
     now() - interval '4 hours', now() - interval '4 hours', now() - interval '3 hours'),
    (76561198000000008, 'succeeded', 0, 0, 0, 0, 6, 6, 0, 0,
     NULL, now() - interval '7 hours', now() - interval '7 hours', now() - interval '6 hours'),
    (76561198000000009, 'succeeded', 1, 1, 0, 0, 0, 0, 0, 0,
     NULL, now() - interval '9 hours', now() - interval '9 hours', now() - interval '8 hours');

COMMIT;

ANALYZE;
