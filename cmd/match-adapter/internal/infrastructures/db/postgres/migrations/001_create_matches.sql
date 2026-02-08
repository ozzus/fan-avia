CREATE TABLE IF NOT EXISTS public.matches(
match_id bigint primary key,
kickoff_utc timestamptz not null,
city text not null,
stadium text not null,
destination_iata text not null,
tickets_link TEXT NOT NULL DEFAULT '',
club_home_id TEXT NOT NULL DEFAULT '',
club_away_id TEXT NOT NULL DEFAULT '',
created_at timestamptz not null default now(),
updated_at timestamptz not null default now()
);

CREATE INDEX if NOT EXISTS matches_kickoff_utc_idx
  on public.matches (kickoff_utc);

CREATE TABLE IF NOT EXISTS public.city_iata (
  city text primary key,
  iata text not null,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

insert into public.city_iata (city, iata)
values
  ('Saint Petersburg', 'LED'),
  ('Moscow', 'MOW'),
  ('Kaliningrad', 'KGD')
on conflict (city) do update
set iata = excluded.iata,
    updated_at = now();

