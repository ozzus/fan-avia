ALTER TABLE public.club_dictionary
  ADD COLUMN IF NOT EXISTS logo TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS city TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS airport_iata TEXT NOT NULL DEFAULT '';

INSERT INTO public.club_dictionary (club_id, name_ru, name_en, logo, city, airport_iata)
VALUES
  ('1', 'Спартак Москва', 'Spartak Moscow', '1.svg', 'Москва', 'MOW'),
  ('2', 'ПФК ЦСКА', 'PFC CSKA', '2.svg', 'Москва', 'MOW'),
  ('3', 'Зенит', 'Zenit', '3.svg', 'Санкт-Петербург', 'LED'),
  ('4', 'Рубин', 'Rubin', '4.svg', 'Казань', 'KZN'),
  ('5', 'Локомотив', 'Lokomotiv', '5.svg', 'Москва', 'MOW'),
  ('7', 'Динамо Москва', 'Dynamo Moscow', '7.svg', 'Москва', 'MOW'),
  ('10', 'Крылья Советов', 'Krylia Sovetov', '10.svg', 'Самара', 'KUF'),
  ('11', 'Ростов', 'Rostov', '11.svg', 'Ростов-на-Дону', 'ROV'),
  ('125', 'Динамо Махачкала', 'Dynamo Makhachkala', '125.svg', 'Каспийск', 'MCX'),
  ('444', 'Балтика', 'Baltika', '444.svg', 'Калининград', 'KGD'),
  ('504', 'Оренбург', 'Orenburg', '504.svg', 'Оренбург', 'REN'),
  ('525', 'Сочи', 'Sochi', '525.svg', 'Сочи', 'AER'),
  ('584', 'Краснодар', 'Krasnodar', '584.svg', 'Краснодар', 'KRR'),
  ('702', 'Ахмат', 'Akhmat', '702.svg', 'Грозный', 'GRV'),
  ('704', 'Пари НН', 'Pari NN', '704.svg', 'Нижний Новгород', 'GOJ'),
  ('807', 'Акрон', 'Akron', '807.svg', 'Самара', 'KUF')
ON CONFLICT (club_id) DO UPDATE
SET
  name_ru = EXCLUDED.name_ru,
  name_en = EXCLUDED.name_en,
  logo = EXCLUDED.logo,
  city = EXCLUDED.city,
  airport_iata = EXCLUDED.airport_iata,
  updated_at = now();
