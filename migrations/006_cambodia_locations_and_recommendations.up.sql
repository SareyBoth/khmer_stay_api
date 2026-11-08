CREATE TABLE IF NOT EXISTS provinces (
    code TEXT PRIMARY KEY,
    name_en TEXT NOT NULL UNIQUE,
    name_km TEXT NOT NULL,
    slug TEXT NOT NULL UNIQUE,
    sort_order SMALLINT NOT NULL
);

CREATE TABLE IF NOT EXISTS cities (
    id TEXT PRIMARY KEY,
    province_code TEXT NOT NULL REFERENCES provinces(code),
    name_en TEXT NOT NULL,
    name_km TEXT NOT NULL,
    slug TEXT NOT NULL,
    type TEXT NOT NULL CHECK (type IN ('capital', 'municipality', 'district', 'khan', 'island')),
    latitude DOUBLE PRECISION,
    longitude DOUBLE PRECISION,
    UNIQUE (province_code, slug)
);

CREATE TABLE IF NOT EXISTS communes (
    id TEXT PRIMARY KEY,
    city_id TEXT NOT NULL REFERENCES cities(id),
    name_en TEXT NOT NULL,
    name_km TEXT NOT NULL DEFAULT '',
    slug TEXT NOT NULL,
    type TEXT NOT NULL DEFAULT 'commune' CHECK (type IN ('commune', 'sangkat')),
    UNIQUE (city_id, slug)
);

CREATE TABLE IF NOT EXISTS destinations (
    id TEXT PRIMARY KEY,
    province_code TEXT NOT NULL REFERENCES provinces(code),
    city_id TEXT REFERENCES cities(id),
    name_en TEXT NOT NULL,
    name_km TEXT NOT NULL,
    slug TEXT NOT NULL UNIQUE,
    destination_type TEXT NOT NULL,
    description_en TEXT NOT NULL DEFAULT '',
    description_km TEXT NOT NULL DEFAULT '',
    latitude DOUBLE PRECISION,
    longitude DOUBLE PRECISION,
    featured BOOLEAN NOT NULL DEFAULT FALSE
);

ALTER TABLE properties ADD COLUMN IF NOT EXISTS province_code TEXT REFERENCES provinces(code);
ALTER TABLE properties ADD COLUMN IF NOT EXISTS city_id TEXT REFERENCES cities(id);
ALTER TABLE properties ADD COLUMN IF NOT EXISTS commune_id TEXT REFERENCES communes(id);
ALTER TABLE properties ADD COLUMN IF NOT EXISTS latitude DOUBLE PRECISION;
ALTER TABLE properties ADD COLUMN IF NOT EXISTS longitude DOUBLE PRECISION;

CREATE INDEX IF NOT EXISTS properties_location_idx
    ON properties(province_code, city_id, status);

CREATE TABLE IF NOT EXISTS property_destinations (
    property_id TEXT NOT NULL REFERENCES properties(id) ON DELETE CASCADE,
    destination_id TEXT NOT NULL REFERENCES destinations(id) ON DELETE CASCADE,
    PRIMARY KEY (property_id, destination_id)
);

CREATE TABLE IF NOT EXISTS recommendation_events (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    property_id TEXT NOT NULL REFERENCES properties(id) ON DELETE CASCADE,
    event_type TEXT NOT NULL CHECK (event_type IN ('view', 'favorite', 'unfavorite', 'booking_started', 'booking_completed', 'booking_cancelled')),
    weight SMALLINT NOT NULL,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS recommendation_events_user_idx
    ON recommendation_events(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS recommendation_events_property_idx
    ON recommendation_events(property_id, created_at DESC);

INSERT INTO provinces(code,name_en,name_km,slug,sort_order) VALUES
('KH-01','Banteay Meanchey','បន្ទាយមានជ័យ','banteay-meanchey',1),
('KH-02','Battambang','បាត់ដំបង','battambang',2),
('KH-03','Kampong Cham','កំពង់ចាម','kampong-cham',3),
('KH-04','Kampong Chhnang','កំពង់ឆ្នាំង','kampong-chhnang',4),
('KH-05','Kampong Speu','កំពង់ស្ពឺ','kampong-speu',5),
('KH-06','Kampong Thom','កំពង់ធំ','kampong-thom',6),
('KH-07','Kampot','កំពត','kampot',7),
('KH-08','Kandal','កណ្ដាល','kandal',8),
('KH-09','Koh Kong','កោះកុង','koh-kong',9),
('KH-10','Kratie','ក្រចេះ','kratie',10),
('KH-11','Mondulkiri','មណ្ឌលគិរី','mondulkiri',11),
('KH-12','Phnom Penh','ភ្នំពេញ','phnom-penh',12),
('KH-13','Preah Vihear','ព្រះវិហារ','preah-vihear',13),
('KH-14','Prey Veng','ព្រៃវែង','prey-veng',14),
('KH-15','Pursat','ពោធិ៍សាត់','pursat',15),
('KH-16','Ratanakiri','រតនគិរី','ratanakiri',16),
('KH-17','Siem Reap','សៀមរាប','siem-reap',17),
('KH-18','Preah Sihanouk','ព្រះសីហនុ','preah-sihanouk',18),
('KH-19','Stung Treng','ស្ទឹងត្រែង','stung-treng',19),
('KH-20','Svay Rieng','ស្វាយរៀង','svay-rieng',20),
('KH-21','Takeo','តាកែវ','takeo',21),
('KH-22','Oddar Meanchey','ឧត្តរមានជ័យ','oddar-meanchey',22),
('KH-23','Kep','កែប','kep',23),
('KH-24','Pailin','ប៉ៃលិន','pailin',24),
('KH-25','Tboung Khmum','ត្បូងឃ្មុំ','tboung-khmum',25)
ON CONFLICT (code) DO NOTHING;

INSERT INTO cities(id,province_code,name_en,name_km,slug,type,latitude,longitude) VALUES
('KH-01-SEREI-SAOPHOAN','KH-01','Serei Saophoan','សិរីសោភ័ណ','serei-saophoan','municipality',13.5859,102.9737),
('KH-02-BATTAMBANG','KH-02','Battambang','បាត់ដំបង','battambang','municipality',13.0957,103.2022),
('KH-03-KAMPONG-CHAM','KH-03','Kampong Cham','កំពង់ចាម','kampong-cham','municipality',11.9934,105.4635),
('KH-04-KAMPONG-CHHNANG','KH-04','Kampong Chhnang','កំពង់ឆ្នាំង','kampong-chhnang','municipality',12.2500,104.6667),
('KH-05-CHBAR-MON','KH-05','Chbar Mon','ច្បារមន','chbar-mon','municipality',11.4709,104.5066),
('KH-06-STUENG-SAEN','KH-06','Stueng Saen','ស្ទឹងសែន','stueng-saen','municipality',12.7111,104.8887),
('KH-07-KAMPOT','KH-07','Kampot','កំពត','kampot','municipality',10.6104,104.1815),
('KH-08-TA-KHMAU','KH-08','Ta Khmau','តាខ្មៅ','ta-khmau','municipality',11.4833,104.9500),
('KH-09-KHEMARAK-PHOUMIN','KH-09','Khemarak Phoumin','ខេមរភូមិន្ទ','khemarak-phoumin','municipality',11.6153,102.9838),
('KH-10-KRATIE','KH-10','Kratie','ក្រចេះ','kratie','municipality',12.4881,106.0188),
('KH-11-SEN-MONOROM','KH-11','Sen Monorom','សែនមនោរម្យ','sen-monorom','municipality',12.4558,107.1881),
('KH-12-PHNOM-PENH','KH-12','Phnom Penh','ភ្នំពេញ','phnom-penh','capital',11.5564,104.9282),
('KH-13-TBAENG-MEANCHEY','KH-13','Tbaeng Meanchey','ត្បែងមានជ័យ','tbaeng-meanchey','district',13.8073,104.9805),
('KH-14-PREY-VENG','KH-14','Prey Veng','ព្រៃវែង','prey-veng','municipality',11.4868,105.3253),
('KH-15-PURSAT','KH-15','Pursat','ពោធិ៍សាត់','pursat','municipality',12.5388,103.9192),
('KH-16-BANLUNG','KH-16','Banlung','បានលុង','banlung','municipality',13.7394,106.9873),
('KH-17-SIEM-REAP','KH-17','Siem Reap','សៀមរាប','siem-reap','municipality',13.3633,103.8564),
('KH-18-SIHANOUKVILLE','KH-18','Sihanoukville','ព្រះសីហនុ','sihanoukville','municipality',10.6253,103.5234),
('KH-18-KOH-RONG','KH-18','Koh Rong','កោះរ៉ុង','koh-rong','island',10.7060,103.2317),
('KH-19-STUNG-TRENG','KH-19','Stung Treng','ស្ទឹងត្រែង','stung-treng','municipality',13.5259,105.9683),
('KH-20-SVAY-RIENG','KH-20','Svay Rieng','ស្វាយរៀង','svay-rieng','municipality',11.0879,105.7994),
('KH-21-DOUN-KAEV','KH-21','Doun Kaev','ដូនកែវ','doun-kaev','municipality',10.9908,104.7849),
('KH-22-SAMRAONG','KH-22','Samraong','សំរោង','samraong','municipality',14.1818,103.5176),
('KH-23-KEP','KH-23','Kep','កែប','kep','municipality',10.4829,104.3167),
('KH-24-PAILIN','KH-24','Pailin','ប៉ៃលិន','pailin','municipality',12.8489,102.6093),
('KH-25-SUONG','KH-25','Suong','សួង','suong','municipality',11.9118,105.6582)
ON CONFLICT (id) DO NOTHING;

INSERT INTO destinations(id,province_code,city_id,name_en,name_km,slug,destination_type,description_en,featured,latitude,longitude) VALUES
('DEST-ANGKOR','KH-17','KH-17-SIEM-REAP','Angkor & Siem Reap','អង្គរ និងសៀមរាប','angkor-siem-reap','heritage','Temple heritage, local culture and countryside stays.',TRUE,13.4125,103.8670),
('DEST-KAMPOT-RIVER','KH-07','KH-07-KAMPOT','Kampot Riverside','មាត់ទន្លេកំពត','kampot-riverside','riverside','Riverside stays, food and Bokor day trips.',TRUE,10.6104,104.1815),
('DEST-KEP-COAST','KH-23','KH-23-KEP','Kep Coast','ឆ្នេរកែប','kep-coast','beach','Quiet coastal stays and the crab market.',TRUE,10.4829,104.3167),
('DEST-BATTAMBANG','KH-02','KH-02-BATTAMBANG','Battambang Heritage','បេតិកភណ្ឌបាត់ដំបង','battambang-heritage','heritage','Creative city experiences and preserved architecture.',TRUE,13.0957,103.2022),
('DEST-MONDULKIRI','KH-11','KH-11-SEN-MONOROM','Mondulkiri Highlands','តំបន់ខ្ពង់រាបមណ្ឌលគិរី','mondulkiri-highlands','mountain','Forest, waterfalls and highland eco stays.',TRUE,12.4558,107.1881),
('DEST-KOH-RONG','KH-18','KH-18-KOH-RONG','Koh Rong','កោះរ៉ុង','koh-rong','island','Island beaches and community-owned stays.',TRUE,10.7060,103.2317),
('DEST-KRATIE-MEKONG','KH-10','KH-10-KRATIE','Kratie Mekong','មេគង្គក្រចេះ','kratie-mekong','riverside','Mekong river life and dolphin experiences.',TRUE,12.4881,106.0188),
('DEST-RATANAKIRI','KH-16','KH-16-BANLUNG','Ratanakiri Nature','ធម្មជាតិរតនគិរី','ratanakiri-nature','eco','Lakes, waterfalls and red-earth landscapes.',TRUE,13.7394,106.9873),
('DEST-PHNOM-PENH','KH-12','KH-12-PHNOM-PENH','Phnom Penh Riverside','មាត់ទន្លេភ្នំពេញ','phnom-penh-riverside','city','Culture, food and stays near the capital riverside.',TRUE,11.5700,104.9300),
('DEST-PREAH-VIHEAR','KH-13','KH-13-TBAENG-MEANCHEY','Preah Vihear Heritage','បេតិកភណ្ឌព្រះវិហារ','preah-vihear-heritage','heritage','Northern temple landscapes and community stays.',FALSE,13.8073,104.9805),
('DEST-TAKEO','KH-21','KH-21-DOUN-KAEV','Takeo Countryside','ជនបទតាកែវ','takeo-countryside','countryside','Village crafts, farms and quiet local stays.',FALSE,10.9908,104.7849),
('DEST-CARDAMOM','KH-15','KH-15-PURSAT','Cardamom Gateway','ច្រកភ្នំក្រវាញ','cardamom-gateway','eco','A gateway to forest and community tourism.',FALSE,12.5388,103.9192)
ON CONFLICT (id) DO NOTHING;

UPDATE properties p
SET province_code = pr.code
FROM provinces pr
WHERE p.province_code IS NULL
  AND LOWER(p.data->>'province') = LOWER(pr.name_en);

UPDATE properties
SET province_code='KH-18',
    data=jsonb_set(jsonb_set(data,'{province}',to_jsonb('Preah Sihanouk'::text)),'{provinceKm}',to_jsonb('ព្រះសីហនុ'::text))
WHERE LOWER(data->>'province')='koh rong';

WITH mapping(property_id,city_id,destination_id) AS (VALUES
('KS-P001','KH-17-SIEM-REAP','DEST-ANGKOR'),
('KS-P002','KH-07-KAMPOT','DEST-KAMPOT-RIVER'),
('KS-P003','KH-23-KEP','DEST-KEP-COAST'),
('KS-P004','KH-02-BATTAMBANG','DEST-BATTAMBANG'),
('KS-P005','KH-11-SEN-MONOROM','DEST-MONDULKIRI'),
('KS-P006','KH-18-KOH-RONG','DEST-KOH-RONG'),
('KS-P007','KH-10-KRATIE','DEST-KRATIE-MEKONG'),
('KS-P008','KH-16-BANLUNG','DEST-RATANAKIRI'),
('KS-P009','KH-12-PHNOM-PENH','DEST-PHNOM-PENH'),
('KS-P010','KH-13-TBAENG-MEANCHEY','DEST-PREAH-VIHEAR'),
('KS-P011','KH-21-DOUN-KAEV','DEST-TAKEO'),
('KS-P012','KH-15-PURSAT','DEST-CARDAMOM'))
UPDATE properties p
SET city_id=m.city_id,
    province_code=c.province_code,
    latitude=c.latitude,
    longitude=c.longitude,
    data=jsonb_set(jsonb_set(jsonb_set(jsonb_set(p.data,'{provinceCode}',to_jsonb(c.province_code)),'{cityId}',to_jsonb(c.id)),'{city}',to_jsonb(c.name_en)),'{cityKm}',to_jsonb(c.name_km))
FROM mapping m JOIN cities c ON c.id=m.city_id
WHERE p.id=m.property_id;

INSERT INTO property_destinations(property_id,destination_id)
SELECT m.property_id,m.destination_id
FROM (VALUES
('KS-P001','DEST-ANGKOR'),('KS-P002','DEST-KAMPOT-RIVER'),('KS-P003','DEST-KEP-COAST'),
('KS-P004','DEST-BATTAMBANG'),('KS-P005','DEST-MONDULKIRI'),('KS-P006','DEST-KOH-RONG'),
('KS-P007','DEST-KRATIE-MEKONG'),('KS-P008','DEST-RATANAKIRI'),('KS-P009','DEST-PHNOM-PENH'),
('KS-P010','DEST-PREAH-VIHEAR'),('KS-P011','DEST-TAKEO'),('KS-P012','DEST-CARDAMOM')) AS m(property_id,destination_id)
JOIN properties p ON p.id=m.property_id
ON CONFLICT DO NOTHING;
