package platform

import (
	"context"
	"encoding/json"
	"math"
	"sort"
	"time"

	"github.com/google/uuid"
)

var recommendationWeights = map[string]int{
	"view": 1, "favorite": 3, "unfavorite": -2,
	"booking_started": 6, "booking_completed": 10, "booking_cancelled": -3,
}

func (r *Repository) RecordRecommendationEvent(ctx context.Context, userID, propertyID, eventType string, metadata map[string]any) error {
	weight, ok := recommendationWeights[eventType]
	if !ok {
		return nil
	}
	if metadata == nil {
		metadata = map[string]any{}
	}
	raw, _ := json.Marshal(metadata)
	_, err := r.db.Exec(ctx, `INSERT INTO recommendation_events(id,user_id,property_id,event_type,weight,metadata) VALUES($1,$2,$3,$4,$5,$6)`, uuid.NewString(), userID, propertyID, eventType, weight, raw)
	return err
}

type recommendationCandidate struct {
	data          map[string]any
	provinceCode  string
	cityID        string
	category      string
	price         float64
	rating        float64
	reviewCount   float64
	bookingCount  float64
	favoriteCount float64
	updatedAt     time.Time
	score         float64
	reason        string
}

func (r *Repository) Recommendations(ctx context.Context, userID, provinceCode, cityID string, limit int) ([]map[string]any, error) {
	if limit < 1 || limit > 50 {
		limit = 12
	}
	query := `
		SELECT p.data,p.province_code,COALESCE(p.city_id,''),COALESCE(p.data->>'category',''),
		       COALESCE((p.data->>'price')::double precision,0),COALESCE((p.data->>'rating')::double precision,0),
		       COALESCE((p.data->>'reviewCount')::double precision,0),p.updated_at,
		       COUNT(DISTINCT b.id),COUNT(DISTINCT f.user_id)
		FROM properties p
		LEFT JOIN bookings b ON b.property_id=p.id
		LEFT JOIN favorites f ON f.property_id=p.id
		WHERE p.status='Published' AND ($1='' OR p.province_code=$1) AND ($2='' OR p.city_id=$2)
		GROUP BY p.id,p.data,p.province_code,p.city_id,p.updated_at
		ORDER BY p.updated_at DESC`
	rows, err := r.db.Query(ctx, query, provinceCode, cityID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	candidates := []recommendationCandidate{}
	for rows.Next() {
		var raw []byte
		var candidate recommendationCandidate
		if err = rows.Scan(&raw, &candidate.provinceCode, &candidate.cityID, &candidate.category, &candidate.price, &candidate.rating, &candidate.reviewCount, &candidate.updatedAt, &candidate.bookingCount, &candidate.favoriteCount); err != nil {
			return nil, err
		}
		if candidate.data, err = decode(raw); err != nil {
			return nil, err
		}
		candidates = append(candidates, candidate)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}

	type preference struct {
		province, city, category string
		priceTotal, weight       float64
	}
	pref := preference{}
	provinceScores, cityScores, categoryScores := map[string]float64{}, map[string]float64{}, map[string]float64{}
	prefRows, err := r.db.Query(ctx, `
		SELECT COALESCE(p.province_code,''),COALESCE(p.city_id,''),COALESCE(p.data->>'category',''),
		       COALESCE((p.data->>'price')::double precision,0),e.weight
		FROM recommendation_events e JOIN properties p ON p.id=e.property_id
		WHERE e.user_id=$1 AND e.created_at > NOW()-INTERVAL '180 days'`, userID)
	if err != nil {
		return nil, err
	}
	for prefRows.Next() {
		var province, city, category string
		var price, weight float64
		if err = prefRows.Scan(&province, &city, &category, &price, &weight); err != nil {
			prefRows.Close()
			return nil, err
		}
		if weight <= 0 {
			continue
		}
		provinceScores[province] += weight
		cityScores[city] += weight
		categoryScores[category] += weight
		pref.priceTotal += price * weight
		pref.weight += weight
	}
	prefRows.Close()
	pref.province = highestPreference(provinceScores)
	pref.city = highestPreference(cityScores)
	pref.category = highestPreference(categoryScores)
	averagePrice := 0.0
	if pref.weight > 0 {
		averagePrice = pref.priceTotal / pref.weight
	}

	for index := range candidates {
		candidate := &candidates[index]
		candidate.score = math.Min(candidate.rating/5, 1)*25 + math.Min(candidate.bookingCount/20, 1)*20 + math.Min(candidate.favoriteCount/10, 1)*10 + math.Min(candidate.reviewCount/50, 1)*5
		if pref.province != "" && candidate.provinceCode == pref.province {
			candidate.score += 12
			candidate.reason = "Matches a province you explored"
		}
		if pref.city != "" && candidate.cityID == pref.city {
			candidate.score += 8
			candidate.reason = "Popular in a city you explored"
		}
		if pref.category != "" && candidate.category == pref.category {
			candidate.score += 5
			if candidate.reason == "" {
				candidate.reason = "Similar to stays you viewed"
			}
		}
		if averagePrice > 0 {
			difference := math.Abs(candidate.price-averagePrice) / math.Max(averagePrice, 1)
			candidate.score += math.Max(0, 15*(1-difference))
		}
		if time.Since(candidate.updatedAt) <= 30*24*time.Hour {
			candidate.score += 5
		}
		if candidate.reason == "" {
			candidate.reason = "Highly rated by KhmerStay guests"
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool { return candidates[i].score > candidates[j].score })
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}
	result := make([]map[string]any, 0, len(candidates))
	for _, candidate := range candidates {
		candidate.data["recommendationScore"] = math.Round(candidate.score*10) / 10
		candidate.data["recommendationReason"] = candidate.reason
		result = append(result, candidate.data)
	}
	return result, nil
}

func highestPreference(scores map[string]float64) string {
	best := ""
	bestScore := 0.0
	for key, score := range scores {
		if key != "" && score > bestScore {
			best, bestScore = key, score
		}
	}
	return best
}
