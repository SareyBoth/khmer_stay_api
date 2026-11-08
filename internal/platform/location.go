package platform

import (
	"context"
	"errors"
	"strings"
)

var ErrInvalidLocation = errors.New("select a valid Cambodia province and city")

type City struct {
	ID           string  `json:"id"`
	ProvinceCode string  `json:"provinceCode"`
	Name         string  `json:"name"`
	NameKm       string  `json:"nameKm"`
	Slug         string  `json:"slug"`
	Type         string  `json:"type"`
	Latitude     float64 `json:"latitude"`
	Longitude    float64 `json:"longitude"`
}

type Province struct {
	Code   string `json:"code"`
	Name   string `json:"name"`
	NameKm string `json:"nameKm"`
	Slug   string `json:"slug"`
	Cities []City `json:"cities"`
}

type Destination struct {
	ID            string  `json:"id"`
	ProvinceCode  string  `json:"provinceCode"`
	CityID        string  `json:"cityId"`
	Name          string  `json:"name"`
	NameKm        string  `json:"nameKm"`
	Slug          string  `json:"slug"`
	Type          string  `json:"type"`
	Description   string  `json:"description"`
	DescriptionKm string  `json:"descriptionKm"`
	Latitude      float64 `json:"latitude"`
	Longitude     float64 `json:"longitude"`
	Featured      bool    `json:"featured"`
}

func (r *Repository) Provinces(ctx context.Context) ([]Province, error) {
	rows, err := r.db.Query(ctx, `
		SELECT p.code,p.name_en,p.name_km,p.slug,
		       COALESCE(c.id,''),COALESCE(c.name_en,''),COALESCE(c.name_km,''),
		       COALESCE(c.slug,''),COALESCE(c.type,''),COALESCE(c.latitude,0),COALESCE(c.longitude,0)
		FROM provinces p
		LEFT JOIN cities c ON c.province_code=p.code
		ORDER BY p.sort_order,c.name_en`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Province{}
	indexes := map[string]int{}
	for rows.Next() {
		var province Province
		var city City
		if err = rows.Scan(&province.Code, &province.Name, &province.NameKm, &province.Slug, &city.ID, &city.Name, &city.NameKm, &city.Slug, &city.Type, &city.Latitude, &city.Longitude); err != nil {
			return nil, err
		}
		index, ok := indexes[province.Code]
		if !ok {
			province.Cities = []City{}
			result = append(result, province)
			index = len(result) - 1
			indexes[province.Code] = index
		}
		if city.ID != "" {
			city.ProvinceCode = province.Code
			result[index].Cities = append(result[index].Cities, city)
		}
	}
	return result, rows.Err()
}

func (r *Repository) Destinations(ctx context.Context, provinceCode string) ([]Destination, error) {
	query := `SELECT id,province_code,COALESCE(city_id,''),name_en,name_km,slug,destination_type,description_en,description_km,COALESCE(latitude,0),COALESCE(longitude,0),featured FROM destinations`
	args := []any{}
	if provinceCode != "" {
		query += ` WHERE province_code=$1`
		args = append(args, provinceCode)
	}
	query += ` ORDER BY featured DESC,name_en`
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Destination{}
	for rows.Next() {
		var item Destination
		if err = rows.Scan(&item.ID, &item.ProvinceCode, &item.CityID, &item.Name, &item.NameKm, &item.Slug, &item.Type, &item.Description, &item.DescriptionKm, &item.Latitude, &item.Longitude, &item.Featured); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

type propertyLocation struct {
	ProvinceCode string
	Province     string
	ProvinceKm   string
	CityID       string
	City         string
	CityKm       string
	Latitude     float64
	Longitude    float64
}

func stringValue(value any) string {
	text, _ := value.(string)
	return strings.TrimSpace(text)
}

func (r *Repository) resolvePropertyLocation(ctx context.Context, input map[string]any) (propertyLocation, error) {
	provinceCode := stringValue(input["provinceCode"])
	provinceName := stringValue(input["province"])
	cityID := stringValue(input["cityId"])
	location := propertyLocation{}
	if provinceCode == "" && provinceName == "" {
		return location, nil
	}
	if provinceCode != "" {
		if err := r.db.QueryRow(ctx, `SELECT code,name_en,name_km FROM provinces WHERE code=$1`, provinceCode).Scan(&location.ProvinceCode, &location.Province, &location.ProvinceKm); err != nil {
			return location, ErrInvalidLocation
		}
	} else if err := r.db.QueryRow(ctx, `SELECT code,name_en,name_km FROM provinces WHERE LOWER(name_en)=LOWER($1)`, provinceName).Scan(&location.ProvinceCode, &location.Province, &location.ProvinceKm); err != nil {
		return location, ErrInvalidLocation
	}
	if cityID == "" {
		return location, nil
	}
	if err := r.db.QueryRow(ctx, `SELECT id,name_en,name_km,COALESCE(latitude,0),COALESCE(longitude,0) FROM cities WHERE id=$1 AND province_code=$2`, cityID, location.ProvinceCode).Scan(&location.CityID, &location.City, &location.CityKm, &location.Latitude, &location.Longitude); err != nil {
		return location, ErrInvalidLocation
	}
	return location, nil
}

func applyLocation(input map[string]any, location propertyLocation) {
	if location.ProvinceCode == "" {
		return
	}
	input["provinceCode"] = location.ProvinceCode
	input["province"] = location.Province
	input["provinceKm"] = location.ProvinceKm
	input["cityId"] = location.CityID
	input["city"] = location.City
	input["cityKm"] = location.CityKm
	if location.Latitude != 0 || location.Longitude != 0 {
		input["latitude"] = location.Latitude
		input["longitude"] = location.Longitude
	}
}
