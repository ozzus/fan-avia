INSERT INTO public.club_dictionary (club_id, name_ru, name_en)
VALUES
  ('1', 'Спартак Москва', 'Spartak Moscow'),
  ('2', 'ПФК ЦСКА', 'PFC CSKA'),
  ('3', 'Зенит', 'Zenit'),
  ('4', 'Рубин', 'Rubin'),
  ('5', 'Локомотив', 'Lokomotiv'),
  ('7', 'Динамо Москва', 'Dynamo Moscow'),
  ('10', 'Крылья Советов', 'Krylia Sovetov'),
  ('11', 'Ростов', 'Rostov'),
  ('125', 'Динамо Махачкала', 'Dynamo Makhachkala'),
  ('444', 'Балтика', 'Baltika'),
  ('504', 'Оренбург', 'Orenburg'),
  ('525', 'Сочи', 'Sochi'),
  ('584', 'Краснодар', 'Krasnodar'),
  ('702', 'Ахмат', 'Akhmat'),
  ('704', 'Пари НН', 'Pari NN'),
  ('807', 'Акрон', 'Akron')
ON CONFLICT (club_id) DO UPDATE
SET
  name_ru = EXCLUDED.name_ru,
  name_en = EXCLUDED.name_en,
  updated_at = now();
