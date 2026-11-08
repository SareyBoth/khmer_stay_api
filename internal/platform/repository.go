package platform

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("record not found")
var ErrReviewNotAllowed = errors.New("a completed stay is required to review this property")

type Repository struct{ db *pgxpool.Pool }

func NewRepository(db *pgxpool.Pool) *Repository { return &Repository{db} }
func decode(raw []byte) (map[string]any, error) {
	var value map[string]any
	err := json.Unmarshal(raw, &value)
	return value, err
}
func encode(value map[string]any) ([]byte, error) { return json.Marshal(value) }
func merge(base, updates map[string]any) map[string]any {
	for key, value := range updates {
		base[key] = value
	}
	return base
}

type PropertyFilter struct {
	Role         string
	UserID       string
	ProvinceCode string
	CityID       string
	CheckIn      string
	CheckOut     string
	Guests       int
	MinPrice     float64
	MaxPrice     float64
	Category     string
	Amenities    string
	Query        string
}

func (r *Repository) ListProperties(ctx context.Context, role, userID string) ([]map[string]any, error) {
	return r.SearchProperties(ctx, PropertyFilter{Role: role, UserID: userID})
}

func (r *Repository) ListPropertiesFiltered(ctx context.Context, role, userID, provinceCode, cityID string) ([]map[string]any, error) {
	return r.SearchProperties(ctx, PropertyFilter{Role: role, UserID: userID, ProvinceCode: provinceCode, CityID: cityID})
}

func (r *Repository) SearchProperties(ctx context.Context, filter PropertyFilter) ([]map[string]any, error) {
	query := `SELECT data FROM properties WHERE 1=1`
	args := []any{}

	if filter.ProvinceCode != "" {
		args = append(args, filter.ProvinceCode)
		query += fmt.Sprintf(" AND province_code=$%d", len(args))
	}
	if filter.CityID != "" {
		args = append(args, filter.CityID)
		query += fmt.Sprintf(" AND city_id=$%d", len(args))
	}
	if filter.Role == "owner" {
		args = append(args, filter.UserID)
		query += fmt.Sprintf(" AND owner_id=$%d", len(args))
	} else if filter.Role == "traveler" {
		query += " AND status='Published'"
	}
	if filter.Category != "" {
		args = append(args, filter.Category)
		query += fmt.Sprintf(" AND LOWER(data->>'category')=LOWER($%d)", len(args))
	}
	if filter.MinPrice > 0 {
		args = append(args, filter.MinPrice)
		query += fmt.Sprintf(" AND COALESCE((data->>'price')::double precision, 0) >= $%d", len(args))
	}
	if filter.MaxPrice > 0 {
		args = append(args, filter.MaxPrice)
		query += fmt.Sprintf(" AND COALESCE((data->>'price')::double precision, 0) <= $%d", len(args))
	}
	if filter.Guests > 0 {
		args = append(args, filter.Guests)
		query += fmt.Sprintf(" AND (COALESCE((data->>'guests')::int, 0) >= $%d OR EXISTS(SELECT 1 FROM jsonb_array_elements(data->'roomTypes') rt WHERE COALESCE((rt->>'guests')::int, 0) >= $%d))", len(args), len(args))
	}
	if filter.Query != "" {
		args = append(args, "%"+strings.ToLower(filter.Query)+"%")
		query += fmt.Sprintf(" AND (LOWER(data->>'name') LIKE $%d OR LOWER(data->>'nameKm') LIKE $%d OR LOWER(data->>'description') LIKE $%d OR LOWER(data->>'province') LIKE $%d OR LOWER(data->>'city') LIKE $%d)", len(args), len(args), len(args), len(args), len(args))
	}
	if filter.Amenities != "" {
		parts := strings.Split(filter.Amenities, ",")
		for _, a := range parts {
			a = strings.TrimSpace(a)
			if a != "" {
				args = append(args, "%"+strings.ToLower(a)+"%")
				query += fmt.Sprintf(" AND LOWER(data->>'amenities') LIKE $%d", len(args))
			}
		}
	}
	if filter.CheckIn != "" && filter.CheckOut != "" {
		args = append(args, filter.CheckIn, filter.CheckOut)
		idxIn := len(args) - 1
		idxOut := len(args)
		query += fmt.Sprintf(` AND id NOT IN (
			SELECT property_id FROM bookings 
			WHERE status IN ('Pending', 'New request', 'Confirmed', 'Checked in')
			  AND check_in < $%d::date AND check_out > $%d::date
			UNION
			SELECT property_id FROM blocked_dates
			WHERE date >= $%d::date AND date < $%d::date
		)`, idxOut, idxIn, idxIn, idxOut)
	}

	query += " ORDER BY updated_at DESC"
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []map[string]any{}
	for rows.Next() {
		var raw []byte
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		item, e := decode(raw)
		if e != nil {
			return nil, e
		}
		result = append(result, item)
	}
	return result, rows.Err()
}
func (r *Repository) GetProperty(ctx context.Context, id string) (map[string]any, error) {
	var raw []byte
	err := r.db.QueryRow(ctx, `SELECT data FROM properties WHERE id=$1`, id).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return decode(raw)
}
func (r *Repository) CreateProperty(ctx context.Context, ownerID, ownerName string, input map[string]any) (map[string]any, error) {
	location, err := r.resolvePropertyLocation(ctx, input)
	if err != nil {
		return nil, err
	}
	applyLocation(input, location)
	id, _ := input["id"].(string)
	if id == "" {
		id = "stay-" + uuid.NewString()
	}
	input["id"] = id
	input["ownerId"] = ownerID
	input["owner"] = ownerName
	if _, ok := input["status"]; !ok {
		input["status"] = "Draft"
	}
	input["updated"] = time.Now().Format("2006-01-02")
	raw, err := encode(input)
	if err != nil {
		return nil, err
	}
	_, err = r.db.Exec(ctx, `INSERT INTO properties(id,owner_id,status,data,province_code,city_id,latitude,longitude)VALUES($1,$2,$3,$4,NULLIF($5,''),NULLIF($6,''),NULLIF($7,0),NULLIF($8,0))`, id, ownerID, input["status"], raw, location.ProvinceCode, location.CityID, location.Latitude, location.Longitude)
	return input, err
}
func (r *Repository) UpdateProperty(ctx context.Context, id string, updates map[string]any) (map[string]any, error) {
	current, err := r.GetProperty(ctx, id)
	if err != nil {
		return nil, err
	}
	current = merge(current, updates)
	current["id"] = id
	current["updated"] = time.Now().Format("2006-01-02")
	location, err := r.resolvePropertyLocation(ctx, current)
	if err != nil {
		return nil, err
	}
	applyLocation(current, location)
	raw, _ := encode(current)
	status, _ := current["status"].(string)
	tag, err := r.db.Exec(ctx, `UPDATE properties SET data=$2,status=$3,province_code=NULLIF($4,''),city_id=NULLIF($5,''),latitude=NULLIF($6,0),longitude=NULLIF($7,0),updated_at=NOW() WHERE id=$1`, id, raw, status, location.ProvinceCode, location.CityID, location.Latitude, location.Longitude)
	if err == nil && tag.RowsAffected() == 0 {
		return nil, ErrNotFound
	}
	return current, err
}
func (r *Repository) SetPropertyStatus(ctx context.Context, id, status string) (map[string]any, error) {
	return r.UpdateProperty(ctx, id, map[string]any{"status": status})
}

func (r *Repository) ListBookings(ctx context.Context, role, userID string) ([]map[string]any, error) {
	query := `SELECT data FROM bookings ORDER BY created_at DESC`
	args := []any{}
	if role == "owner" {
		query = `SELECT data FROM bookings WHERE owner_id=$1 ORDER BY created_at DESC`
		args = []any{userID}
	} else if role == "traveler" {
		query = `SELECT data FROM bookings WHERE traveler_id=$1 ORDER BY created_at DESC`
		args = []any{userID}
	}
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []map[string]any{}
	for rows.Next() {
		var raw []byte
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		item, e := decode(raw)
		if e != nil {
			return nil, e
		}
		result = append(result, item)
	}
	return result, rows.Err()
}
func (r *Repository) GetBooking(ctx context.Context, id string) (map[string]any, error) {
	var raw []byte
	err := r.db.QueryRow(ctx, `SELECT data FROM bookings WHERE id=$1`, id).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return decode(raw)
}
func (r *Repository) CreateBooking(ctx context.Context, travelerID, travelerName, travelerEmail string, input map[string]any) (map[string]any, error) {
	propertyID, _ := input["propertyId"].(string)
	property, err := r.GetProperty(ctx, propertyID)
	if err != nil {
		return nil, err
	}
	ownerID, _ := property["ownerId"].(string)
	owner, _ := property["owner"].(string)

	checkInStr, _ := input["checkIn"].(string)
	checkOutStr, _ := input["checkOut"].(string)
	roomID, _ := input["roomId"].(string)

	if checkInStr != "" && checkOutStr != "" {
		checkIn, errIn := time.Parse("2006-01-02", checkInStr)
		checkOut, errOut := time.Parse("2006-01-02", checkOutStr)
		if errIn != nil || errOut != nil {
			return nil, errors.New("checkIn and checkOut must be in YYYY-MM-DD format")
		}
		if !checkOut.After(checkIn) {
			return nil, errors.New("check-out date must be after check-in date")
		}

		// Double-booking conflict check
		var conflict bool
		err = r.db.QueryRow(ctx, `
			SELECT EXISTS(
				SELECT 1 FROM bookings 
				WHERE property_id=$1 
				  AND ($2='' OR room_id=$2)
				  AND status IN ('Pending', 'New request', 'Confirmed', 'Checked in')
				  AND check_in < $4::date AND check_out > $3::date
			)`, propertyID, roomID, checkInStr, checkOutStr).Scan(&conflict)
		if err != nil {
			return nil, err
		}
		if conflict {
			return nil, errors.New("property is already reserved for the selected dates")
		}

		// Blocked dates check
		var blocked bool
		err = r.db.QueryRow(ctx, `
			SELECT EXISTS(
				SELECT 1 FROM blocked_dates 
				WHERE property_id=$1 AND date >= $2::date AND date < $3::date
			)`, propertyID, checkInStr, checkOutStr).Scan(&blocked)
		if err != nil {
			return nil, err
		}
		if blocked {
			return nil, errors.New("selected dates include blocked host dates")
		}
	}

	id := "KST-" + time.Now().Format("060102") + "-" + uuid.NewString()[:8]
	input["id"] = id
	input["travelerId"] = travelerID
	input["guest"] = travelerName
	input["guestEmail"] = travelerEmail
	input["ownerId"] = ownerID
	input["owner"] = owner
	input["property"] = property["name"]
	input["status"] = "Pending"
	input["payment"] = "Pending"
	input["createdAt"] = time.Now().UTC().Format(time.RFC3339)
	total, ok := number(input["total"])
	if !ok || total <= 0 {
		return nil, errors.New("total must be positive")
	}
	currency := "USD"
	if v, ok := input["currency"].(string); ok && (v == "USD" || v == "KHR") {
		currency = v
	}
	raw, _ := encode(input)
	_, err = r.db.Exec(ctx, `INSERT INTO bookings(id,traveler_id,owner_id,property_id,status,payment_status,amount,currency,check_in,check_out,room_id,data)VALUES($1,$2,$3,$4,'Pending','Pending',$5,$6,NULLIF($7,'')::date,NULLIF($8,'')::date,$9,$10)`, id, travelerID, ownerID, propertyID, total, currency, checkInStr, checkOutStr, roomID, raw)
	return input, err
}
func (r *Repository) SetBookingStatus(ctx context.Context, id, status string) (map[string]any, error) {
	current, err := r.GetBooking(ctx, id)
	if err != nil {
		return nil, err
	}
	current["status"] = status
	raw, _ := encode(current)
	tag, err := r.db.Exec(ctx, `UPDATE bookings SET status=$2,data=$3,updated_at=NOW() WHERE id=$1`, id, status, raw)
	if err == nil && tag.RowsAffected() == 0 {
		return nil, ErrNotFound
	}
	return current, err
}

func (r *Repository) Favorites(ctx context.Context, userID string) ([]string, error) {
	rows, err := r.db.Query(ctx, `SELECT property_id FROM favorites WHERE user_id=$1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []string{}
	for rows.Next() {
		var propertyID string
		if err = rows.Scan(&propertyID); err != nil {
			return nil, err
		}
		result = append(result, propertyID)
	}
	return result, rows.Err()
}

func (r *Repository) AddFavorite(ctx context.Context, userID, propertyID string) error {
	_, err := r.db.Exec(ctx, `INSERT INTO favorites(user_id,property_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, userID, propertyID)
	return err
}

func (r *Repository) RemoveFavorite(ctx context.Context, userID, propertyID string) error {
	_, err := r.db.Exec(ctx, `DELETE FROM favorites WHERE user_id=$1 AND property_id=$2`, userID, propertyID)
	return err
}

func (r *Repository) Notifications(ctx context.Context, userID string) ([]map[string]any, error) {
	rows, err := r.db.Query(ctx, `SELECT id,title,body,category,created_at,read FROM notifications WHERE user_id=$1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []map[string]any{}
	for rows.Next() {
		var id, title, body, category string
		var created time.Time
		var read bool
		if err = rows.Scan(&id, &title, &body, &category, &created, &read); err != nil {
			return nil, err
		}
		result = append(result, map[string]any{"id": id, "title": title, "body": body, "category": category, "time": created, "read": read})
	}
	return result, rows.Err()
}
func (r *Repository) ReadNotification(ctx context.Context, userID, id string) error {
	tag, err := r.db.Exec(ctx, `UPDATE notifications SET read=TRUE WHERE id=$1 AND user_id=$2`, id, userID)
	if err == nil && tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}
func (r *Repository) Conversations(ctx context.Context, role, userID string) ([]map[string]any, error) {
	where := "c.traveler_id=$1"
	if role == "owner" {
		where = "c.kind='property' AND c.owner_id=$1"
	} else if role == "admin" {
		where = "c.kind='support'"
	}
	query := `
		SELECT c.id,c.kind,c.subject,c.status,tu.name,
		       COALESCE(ou.name,'KhmerStay support'),COALESCE(c.property_id,''),
		       COALESCE(p.data->>'name','KhmerStay support'),COALESCE(au.name,''),
		       c.created_at,c.updated_at,COALESCE(last_message.body,''),last_message.created_at,
		       (SELECT COUNT(*) FROM messages unread
		        WHERE unread.conversation_id=c.id AND unread.sender_id<>$1
		          AND unread.created_at>COALESCE(
		            (SELECT cr.last_read_at FROM conversation_reads cr WHERE cr.conversation_id=c.id AND cr.user_id=$1),
		            'epoch'::timestamptz))
		FROM conversations c
		JOIN users tu ON tu.id=c.traveler_id
		LEFT JOIN users ou ON ou.id=c.owner_id
		LEFT JOIN users au ON au.id=c.assigned_admin_id
		LEFT JOIN properties p ON p.id=c.property_id
		LEFT JOIN LATERAL (
			SELECT m.body,m.created_at FROM messages m
			WHERE m.conversation_id=c.id ORDER BY m.created_at DESC LIMIT 1
		) last_message ON TRUE
		WHERE ` + where + `
		ORDER BY COALESCE(last_message.created_at,c.updated_at) DESC`
	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []map[string]any{}
	for rows.Next() {
		var id, kind, subject, status, traveler, owner, propertyID, property, assignedAdmin, preview string
		var created, updated time.Time
		var lastMessageAt *time.Time
		var unread int
		if err = rows.Scan(&id, &kind, &subject, &status, &traveler, &owner, &propertyID, &property, &assignedAdmin, &created, &updated, &preview, &lastMessageAt, &unread); err != nil {
			return nil, err
		}
		result = append(result, map[string]any{
			"id": id, "kind": kind, "subject": subject, "status": status,
			"travelerName": traveler, "ownerName": owner, "propertyId": propertyID,
			"property": property, "assignedAdminName": assignedAdmin,
			"preview": preview, "unreadCount": unread, "createdAt": created,
			"updatedAt": updated, "lastMessageAt": lastMessageAt,
		})
	}
	return result, rows.Err()
}

func (r *Repository) CreateConversation(ctx context.Context, travelerID, kind, propertyID, subject string) (map[string]any, error) {
	kind = strings.TrimSpace(strings.ToLower(kind))
	subject = strings.TrimSpace(subject)
	id := uuid.NewString()
	if kind == "property" {
		if strings.TrimSpace(propertyID) == "" {
			return nil, errors.New("propertyId is required for an owner conversation")
		}
		err := r.db.QueryRow(ctx, `
			INSERT INTO conversations(id,traveler_id,owner_id,property_id,kind,subject,status,updated_at)
			SELECT $1,$2,p.owner_id,p.id,'property',COALESCE(NULLIF($4,''),p.data->>'name','Property conversation'),'open',NOW()
			FROM properties p WHERE p.id=$3 AND p.status='Published'
			ON CONFLICT (traveler_id,owner_id,property_id) WHERE kind='property'
			DO UPDATE SET updated_at=conversations.updated_at
			RETURNING id`, id, travelerID, propertyID, subject).Scan(&id)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		if err != nil {
			return nil, err
		}
	} else if kind == "support" {
		if subject == "" {
			subject = "KhmerStay support"
		}
		err := r.db.QueryRow(ctx, `
			INSERT INTO conversations(id,traveler_id,kind,subject,status,updated_at)
			VALUES($1,$2,'support',$3,'open',NOW())
			ON CONFLICT (traveler_id) WHERE kind='support' AND status='open'
			DO UPDATE SET subject=CASE WHEN conversations.subject='' THEN EXCLUDED.subject ELSE conversations.subject END,
			              updated_at=conversations.updated_at
			RETURNING id`, id, travelerID, subject).Scan(&id)
		if err != nil {
			return nil, err
		}
	} else {
		return nil, errors.New("type must be property or support")
	}
	return map[string]any{"id": id, "kind": kind, "subject": subject, "propertyId": propertyID, "status": "open"}, nil
}

func (r *Repository) Messages(ctx context.Context, conversationID, userID, role string) ([]map[string]any, error) {
	if ok, err := r.canSeeConversation(ctx, conversationID, userID, role); err != nil || !ok {
		if err != nil {
			return nil, err
		}
		return nil, ErrNotFound
	}
	rows, err := r.db.Query(ctx, `
		SELECT m.id,m.sender_id,u.name,u.role,m.body,m.created_at
		FROM messages m JOIN users u ON u.id=m.sender_id
		WHERE m.conversation_id=$1 ORDER BY m.created_at`, conversationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []map[string]any{}
	for rows.Next() {
		var id, sender, senderName, senderRole, body string
		var created time.Time
		if err = rows.Scan(&id, &sender, &senderName, &senderRole, &body, &created); err != nil {
			return nil, err
		}
		result = append(result, map[string]any{"id": id, "conversationId": conversationID, "senderId": sender, "senderName": senderName, "senderRole": senderRole, "text": body, "fromMe": sender == userID, "createdAt": created, "sentAt": created})
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if err = r.MarkConversationRead(ctx, conversationID, userID, role); err != nil {
		return nil, err
	}
	return result, nil
}
func (r *Repository) SendMessage(ctx context.Context, conversationID, userID, role, body string) (map[string]any, error) {
	if ok, err := r.canSeeConversation(ctx, conversationID, userID, role); err != nil || !ok {
		if err != nil {
			return nil, err
		}
		return nil, ErrNotFound
	}
	id := uuid.NewString()
	created := time.Now().UTC()
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `INSERT INTO messages(id,conversation_id,sender_id,body,created_at)VALUES($1,$2,$3,$4,$5)`, id, conversationID, userID, body, created); err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, `
		UPDATE conversations SET updated_at=$2,
		status=CASE WHEN kind='support' AND status<>'open' AND $3='traveler' THEN 'open' ELSE status END
		WHERE id=$1`, conversationID, created, role); err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, `
		INSERT INTO conversation_reads(conversation_id,user_id,last_read_at) VALUES($1,$2,$3)
		ON CONFLICT(conversation_id,user_id) DO UPDATE SET last_read_at=EXCLUDED.last_read_at`, conversationID, userID, created); err != nil {
		return nil, err
	}
	var senderName, senderRole string
	if err = tx.QueryRow(ctx, `SELECT name,role FROM users WHERE id=$1`, userID).Scan(&senderName, &senderRole); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return map[string]any{"id": id, "conversationId": conversationID, "senderId": userID, "senderName": senderName, "senderRole": senderRole, "text": body, "fromMe": true, "createdAt": created, "sentAt": created}, nil
}
func (r *Repository) canSeeConversation(ctx context.Context, id, userID, role string) (bool, error) {
	var allowed bool
	err := r.db.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM conversations c WHERE c.id=$1 AND (
				($3='traveler' AND c.traveler_id=$2) OR
				($3='owner' AND c.kind='property' AND c.owner_id=$2) OR
				($3='admin' AND c.kind='support')
			)
		)`, id, userID, role).Scan(&allowed)
	return allowed, err
}

func (r *Repository) MarkConversationRead(ctx context.Context, conversationID, userID, role string) error {
	ok, err := r.canSeeConversation(ctx, conversationID, userID, role)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotFound
	}
	_, err = r.db.Exec(ctx, `
		INSERT INTO conversation_reads(conversation_id,user_id,last_read_at) VALUES($1,$2,NOW())
		ON CONFLICT(conversation_id,user_id) DO UPDATE SET last_read_at=EXCLUDED.last_read_at`, conversationID, userID)
	return err
}

func (r *Repository) SetConversationStatus(ctx context.Context, conversationID, adminID, status string) (map[string]any, error) {
	if status != "open" && status != "resolved" && status != "closed" {
		return nil, errors.New("status must be open, resolved, or closed")
	}
	var subject string
	err := r.db.QueryRow(ctx, `
		UPDATE conversations SET status=$3,assigned_admin_id=$2,updated_at=NOW()
		WHERE id=$1 AND kind='support'
		RETURNING subject`, conversationID, adminID, status).Scan(&subject)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return map[string]any{"id": conversationID, "subject": subject, "status": status}, nil
}

func (r *Repository) ConversationRecipients(ctx context.Context, conversationID, senderID string) ([]string, error) {
	var kind, travelerID string
	var ownerID *string
	if err := r.db.QueryRow(ctx, `SELECT kind,traveler_id,owner_id FROM conversations WHERE id=$1`, conversationID).Scan(&kind, &travelerID, &ownerID); err != nil {
		return nil, err
	}
	ids := []string{}
	seen := map[string]bool{senderID: true}
	add := func(id string) {
		if id != "" && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	add(travelerID)
	if kind == "property" {
		if ownerID != nil {
			add(*ownerID)
		}
		return ids, nil
	}
	rows, err := r.db.Query(ctx, `SELECT id FROM users WHERE role='admin' AND status='Active'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			return nil, err
		}
		add(id)
	}
	return ids, rows.Err()
}

func (r *Repository) Promotions(ctx context.Context) ([]map[string]any, error) {
	rows, err := r.db.Query(ctx, `SELECT data FROM promotions ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []map[string]any{}
	for rows.Next() {
		var raw []byte
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		item, e := decode(raw)
		if e != nil {
			return nil, e
		}
		result = append(result, item)
	}
	return result, rows.Err()
}
func (r *Repository) CreatePromotion(ctx context.Context, input map[string]any) (map[string]any, error) {
	id := "PROMO-" + uuid.NewString()[:8]
	input["id"] = id
	raw, _ := encode(input)
	_, err := r.db.Exec(ctx, `INSERT INTO promotions(id,data)VALUES($1,$2)`, id, raw)
	return input, err
}
func (r *Repository) BlockedDates(ctx context.Context, userID, role string) ([]map[string]any, error) {
	query := `SELECT property_id,date FROM blocked_dates ORDER BY date`
	args := []any{}
	if role == "owner" {
		query = `SELECT property_id,date FROM blocked_dates WHERE owner_id=$1 ORDER BY date`
		args = []any{userID}
	}
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []map[string]any{}
	for rows.Next() {
		var propertyID string
		var date time.Time
		if err = rows.Scan(&propertyID, &date); err != nil {
			return nil, err
		}
		result = append(result, map[string]any{"propertyId": propertyID, "date": date.Format("2006-01-02")})
	}
	return result, rows.Err()
}
func (r *Repository) ToggleBlockedDate(ctx context.Context, ownerID, propertyID, date string) (bool, error) {
	var exists bool
	if err := r.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM blocked_dates WHERE property_id=$1 AND date=$2)`, propertyID, date).Scan(&exists); err != nil {
		return false, err
	}
	if exists {
		_, err := r.db.Exec(ctx, `DELETE FROM blocked_dates WHERE property_id=$1 AND date=$2`, propertyID, date)
		return false, err
	}
	_, err := r.db.Exec(ctx, `INSERT INTO blocked_dates(owner_id,property_id,date)VALUES($1,$2,$3)`, ownerID, propertyID, date)
	return true, err
}
func (r *Repository) Settings(ctx context.Context) (map[string]any, error) {
	rows, err := r.db.Query(ctx, `SELECT key,value FROM settings`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := map[string]any{}
	for rows.Next() {
		var key string
		var raw []byte
		if err = rows.Scan(&key, &raw); err != nil {
			return nil, err
		}
		var value any
		if json.Unmarshal(raw, &value) != nil {
			value = string(raw)
		}
		result[key] = value
	}
	return result, rows.Err()
}
func (r *Repository) UpdateSettings(ctx context.Context, updates map[string]any) (map[string]any, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	for key, value := range updates {
		raw, _ := json.Marshal(value)
		if _, err = tx.Exec(ctx, `INSERT INTO settings(key,value)VALUES($1,$2)ON CONFLICT(key)DO UPDATE SET value=EXCLUDED.value`, key, raw); err != nil {
			return nil, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return r.Settings(ctx)
}
func (r *Repository) Reviews(ctx context.Context, propertyID string) ([]map[string]any, error) {
	query := `SELECT r.id,r.property_id,u.name,r.rating,r.comment,r.response,r.created_at FROM reviews r JOIN users u ON u.id=r.traveler_id ORDER BY r.created_at DESC`
	args := []any{}
	if propertyID != "" {
		query = `SELECT r.id,r.property_id,u.name,r.rating,r.comment,r.response,r.created_at FROM reviews r JOIN users u ON u.id=r.traveler_id WHERE r.property_id=$1 ORDER BY r.created_at DESC`
		args = []any{propertyID}
	}
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []map[string]any{}
	for rows.Next() {
		var id, pid, name, comment, responseText string
		var rating int
		var created time.Time
		if err = rows.Scan(&id, &pid, &name, &rating, &comment, &responseText, &created); err != nil {
			return nil, err
		}
		result = append(result, map[string]any{"id": id, "propertyId": pid, "guest": name, "author": name, "rating": rating, "comment": comment, "response": responseText, "date": created.Format("2006-01-02")})
	}
	return result, rows.Err()
}

func (r *Repository) CreateReview(ctx context.Context, travelerID, propertyID string, rating int, comment string) (map[string]any, error) {
	var allowed bool
	if err := r.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM bookings WHERE traveler_id=$1 AND property_id=$2 AND status='Completed')`, travelerID, propertyID).Scan(&allowed); err != nil {
		return nil, err
	}
	if !allowed {
		return nil, ErrReviewNotAllowed
	}
	var author string
	if err := r.db.QueryRow(ctx, `SELECT name FROM users WHERE id=$1`, travelerID).Scan(&author); err != nil {
		return nil, err
	}
	id := uuid.NewString()
	created := time.Now().UTC()
	_, err := r.db.Exec(ctx, `INSERT INTO reviews(id,property_id,traveler_id,rating,comment,created_at) VALUES($1,$2,$3,$4,$5,$6)`, id, propertyID, travelerID, rating, comment, created)
	if err != nil {
		return nil, err
	}
	_, _ = r.db.Exec(ctx, `UPDATE properties SET data=jsonb_set(jsonb_set(data,'{rating}',to_jsonb((SELECT COALESCE(AVG(rating),0)::float8 FROM reviews WHERE property_id=$1))),'{reviewCount}',to_jsonb((SELECT COUNT(*) FROM reviews WHERE property_id=$1))),updated_at=NOW() WHERE id=$1`, propertyID)
	return map[string]any{"id": id, "propertyId": propertyID, "guest": author, "author": author, "rating": rating, "comment": comment, "response": "", "date": created.Format("2006-01-02")}, nil
}
func (r *Repository) RespondReview(ctx context.Context, id, responseText string) error {
	tag, err := r.db.Exec(ctx, `UPDATE reviews SET response=$2 WHERE id=$1`, id, responseText)
	if err == nil && tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}
func (r *Repository) Dashboard(ctx context.Context, role, userID string) (map[string]any, error) {
	where := ""
	args := []any{}
	if role == "owner" {
		where = " WHERE owner_id=$1"
		args = []any{userID}
	}
	result := map[string]any{}
	for key, query := range map[string]string{"properties": "SELECT COUNT(*) FROM properties" + where, "bookings": "SELECT COUNT(*) FROM bookings" + where, "revenue": "SELECT COALESCE(SUM(amount),0) FROM bookings" + where + func() string {
		if where == "" {
			return " WHERE payment_status='Paid'"
		}
		return " AND payment_status='Paid'"
	}()} {
		var value any
		if key == "revenue" {
			var n float64
			if err := r.db.QueryRow(ctx, query, args...).Scan(&n); err != nil {
				return nil, err
			}
			value = n
		} else {
			var n int
			if err := r.db.QueryRow(ctx, query, args...).Scan(&n); err != nil {
				return nil, err
			}
			value = n
		}
		result[key] = value
	}
	return result, nil
}
func number(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case int:
		return float64(v), true
	case json.Number:
		n, e := v.Float64()
		return n, e == nil
	default:
		return 0, false
	}
}
