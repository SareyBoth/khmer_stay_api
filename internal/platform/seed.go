package platform

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

const adminID = "8c8db920-f837-4f54-aa97-d487895eb879"
const ownerID = "57ff14c9-a8bb-43d5-8328-d236642cf0b5"
const travelerID = "fd4b6a62-12e9-4ba4-ae83-aa4fc0d46bb2"

func (r *Repository) SeedDemo(ctx context.Context) error {
	var count int
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	hash, err := bcrypt.GenerateFromPassword([]byte("Demo123!"), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	users := [][4]string{{adminID, "admin@khmerstay.demo", "Maly Vong", "admin"}, {ownerID, "owner@khmerstay.demo", "Sokha Chea", "owner"}, {travelerID, "dara@example.demo", "Dara Sok", "traveler"}}
	for _, u := range users {
		if _, err = tx.Exec(ctx, `INSERT INTO users(id,email,password_hash,name,role,status)VALUES($1,$2,$3,$4,$5,'Active')`, u[0], u[1], string(hash), u[2], u[3]); err != nil {
			return err
		}
	}
	names := []string{"Angkor Lotus Heritage Stay", "Kampot Riverstone Guesthouse", "Kep Seabreeze Homestay", "Battambang Colonial Courtyard", "Mondulkiri Pine View Lodge", "Koh Rong Palm Cove Stay", "Kratie Mekong Dolphin House", "Ratanakiri Red Earth Retreat", "Phnom Penh Riverside Boutique Stay", "Preah Vihear Temple View Homestay", "Takeo Silk Village Stay", "Pursat Cardamom Gateway Lodge"}
	type seedLocation struct{ province, provinceKm, provinceCode, city, cityKm, cityID, destinationID string }
	locations := []seedLocation{
		{"Siem Reap", "សៀមរាប", "KH-17", "Siem Reap", "សៀមរាប", "KH-17-SIEM-REAP", "DEST-ANGKOR"},
		{"Kampot", "កំពត", "KH-07", "Kampot", "កំពត", "KH-07-KAMPOT", "DEST-KAMPOT-RIVER"},
		{"Kep", "កែប", "KH-23", "Kep", "កែប", "KH-23-KEP", "DEST-KEP-COAST"},
		{"Battambang", "បាត់ដំបង", "KH-02", "Battambang", "បាត់ដំបង", "KH-02-BATTAMBANG", "DEST-BATTAMBANG"},
		{"Mondulkiri", "មណ្ឌលគិរី", "KH-11", "Sen Monorom", "សែនមនោរម្យ", "KH-11-SEN-MONOROM", "DEST-MONDULKIRI"},
		{"Preah Sihanouk", "ព្រះសីហនុ", "KH-18", "Koh Rong", "កោះរ៉ុង", "KH-18-KOH-RONG", "DEST-KOH-RONG"},
		{"Kratie", "ក្រចេះ", "KH-10", "Kratie", "ក្រចេះ", "KH-10-KRATIE", "DEST-KRATIE-MEKONG"},
		{"Ratanakiri", "រតនគិរី", "KH-16", "Banlung", "បានលុង", "KH-16-BANLUNG", "DEST-RATANAKIRI"},
		{"Phnom Penh", "ភ្នំពេញ", "KH-12", "Phnom Penh", "ភ្នំពេញ", "KH-12-PHNOM-PENH", "DEST-PHNOM-PENH"},
		{"Preah Vihear", "ព្រះវិហារ", "KH-13", "Tbaeng Meanchey", "ត្បែងមានជ័យ", "KH-13-TBAENG-MEANCHEY", "DEST-PREAH-VIHEAR"},
		{"Takeo", "តាកែវ", "KH-21", "Doun Kaev", "ដូនកែវ", "KH-21-DOUN-KAEV", "DEST-TAKEO"},
		{"Pursat", "ពោធិ៍សាត់", "KH-15", "Pursat", "ពោធិ៍សាត់", "KH-15-PURSAT", "DEST-CARDAMOM"},
	}
	images := []string{"siem-reap-homestay.png", "kampot-riverside.png", "kep-coastal.png", "siem-reap-homestay.png", "mondulkiri-lodge.png", "kep-coastal.png"}
	statuses := []string{"Published", "Published", "Published", "Draft", "Published", "Review pending", "Published", "Paused", "Published", "Review pending", "Draft", "Published"}
	for i, name := range names {
		id := fmt.Sprintf("KS-P%03d", i+1)
		price := 24 + i*4
		image := "/images/" + images[i%len(images)]
		location := locations[i]
		property := map[string]any{"id": id, "name": name, "nameKm": name, "province": location.province, "provinceKm": location.provinceKm, "provinceCode": location.provinceCode, "city": location.city, "cityKm": location.cityKm, "cityId": location.cityID, "type": "Homestay", "description": "A locally owned Cambodian stay with welcoming service.", "descriptionKm": "កន្លែងស្នាក់នៅក្នុងស្រុក", "category": "Local stays", "image": image, "images": []string{image}, "price": price, "rating": 4.5, "reviewCount": 0, "bookings": 0, "occupancy": 0, "status": statuses[i], "owner": "Sokha Chea", "ownerId": ownerID, "rooms": 2, "roomTypes": []map[string]any{{"id": id + "-deluxe", "name": "Garden deluxe", "nameKm": "បន្ទប់ប្រណិតមើលសួន", "bed": "1 king bed", "guests": 2, "price": price, "inventory": 3}, {"id": id + "-family", "name": "Family suite", "nameKm": "បន្ទប់គ្រួសារ", "bed": "1 king bed", "guests": 4, "price": price + 18, "inventory": 1}}, "amenities": []string{"Wi-Fi", "Breakfast", "Parking"}, "updated": time.Now().Format("2006-01-02")}
		raw, _ := json.Marshal(property)
		if _, err = tx.Exec(ctx, `INSERT INTO properties(id,owner_id,status,data,province_code,city_id)VALUES($1,$2,$3,$4,$5,$6)`, id, ownerID, statuses[i], raw, location.provinceCode, location.cityID); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO property_destinations(property_id,destination_id)VALUES($1,$2)ON CONFLICT DO NOTHING`, id, location.destinationID); err != nil {
			return err
		}
	}
	bookings := []map[string]any{{"id": "KST-260900", "travelerId": travelerID, "propertyId": "KS-P001", "roomId": "KS-P001-deluxe", "guest": "Dara Sok", "guestEmail": "dara@example.demo", "guestPhone": "+855 12 345 678", "property": names[0], "room": "Garden deluxe", "checkIn": "2026-11-14", "checkOut": "2026-11-17", "guests": 2, "total": 120.00, "currency": "USD", "payment": "Pending", "status": "New request", "owner": "Sokha Chea", "ownerId": ownerID, "specialRequest": "Late arrival", "createdAt": time.Now().UTC().Format(time.RFC3339)}, {"id": "KST-260901", "travelerId": travelerID, "propertyId": "KS-P002", "roomId": "KS-P002-deluxe", "guest": "Dara Sok", "guestEmail": "dara@example.demo", "guestPhone": "+855 12 345 678", "property": names[1], "room": "Garden deluxe", "checkIn": "2026-12-02", "checkOut": "2026-12-05", "guests": 2, "total": 144.00, "currency": "USD", "payment": "Paid", "status": "Confirmed", "owner": "Sokha Chea", "ownerId": ownerID, "specialRequest": "", "createdAt": time.Now().UTC().Format(time.RFC3339)}}
	for _, booking := range bookings {
		raw, _ := json.Marshal(booking)
		if _, err = tx.Exec(ctx, `INSERT INTO bookings(id,traveler_id,owner_id,property_id,status,payment_status,amount,currency,data)VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, booking["id"], travelerID, ownerID, booking["propertyId"], booking["status"], booking["payment"], booking["total"], booking["currency"], raw); err != nil {
			return err
		}
	}
	conversationID := uuid.NewString()
	if _, err = tx.Exec(ctx, `INSERT INTO conversations(id,traveler_id,owner_id,property_id)VALUES($1,$2,$3,'KS-P001')`, conversationID, travelerID, ownerID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO messages(id,conversation_id,sender_id,body)VALUES($1,$2,$3,$4)`, uuid.NewString(), conversationID, travelerID, "Hello, can I arrive after 8 PM?"); err != nil {
		return err
	}
	for _, u := range []string{adminID, ownerID} {
		if _, err = tx.Exec(ctx, `INSERT INTO notifications(id,user_id,title,body,category)VALUES($1,$2,$3,$4,$5)`, uuid.NewString(), u, "Welcome to KhmerStay", "Your PostgreSQL workspace is ready.", "system"); err != nil {
			return err
		}
	}
	for key, value := range map[string]any{"commission": 15, "maintenance": false, "instantBooking": false, "payouts": true} {
		raw, _ := json.Marshal(value)
		if _, err = tx.Exec(ctx, `INSERT INTO settings(key,value)VALUES($1,$2)`, key, raw); err != nil {
			return err
		}
	}
	promotions := []map[string]any{{"id": "PROMO-WELCOME", "code": "WELCOME10", "discount": 10, "status": "Active", "usage": 0, "starts": "2026-01-01", "ends": "2027-12-31"}, {"id": "PROMO-KHMER", "code": "KHMERSTAY", "discount": 15, "status": "Active", "usage": 8, "starts": "2026-01-01", "ends": "2027-12-31"}}
	for _, promotion := range promotions {
		raw, _ := json.Marshal(promotion)
		if _, err = tx.Exec(ctx, `INSERT INTO promotions(id,data)VALUES($1,$2)`, promotion["id"], raw); err != nil {
			return err
		}
	}
	if _, err = tx.Exec(ctx, `INSERT INTO blocked_dates(owner_id,property_id,date)VALUES($1,'KS-P001','2026-11-20'),($1,'KS-P001','2026-11-21')`, ownerID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO reviews(id,property_id,traveler_id,rating,comment,response)VALUES($1,'KS-P001',$2,5,'Wonderful local hospitality','Thank you for staying with us!')`, uuid.NewString(), travelerID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
