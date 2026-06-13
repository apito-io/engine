package mongodb

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/apito-io/engine/models"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const projectUsersCollection = models.ProjectUsersTableName

func mongoUserPhoneOrLegacyUsername(norm string) bson.M {
	return bson.M{"$or": bson.A{
		bson.M{"$expr": bson.M{"$eq": bson.A{
			bson.M{"$toLower": bson.M{"$trim": bson.M{"input": bson.M{"$ifNull": bson.A{"$phone", ""}}}}},
			norm,
		}}},
		bson.M{"$expr": bson.M{"$and": bson.A{
			bson.M{"$eq": bson.A{
				bson.M{"$trim": bson.M{"input": bson.M{"$ifNull": bson.A{"$phone", ""}}}},
				"",
			}},
			bson.M{"$eq": bson.A{
				bson.M{"$toLower": bson.M{"$trim": bson.M{"input": bson.M{"$ifNull": bson.A{"$username", ""}}}}},
				norm,
			}},
		}}},
	}}
}

func (m *SystemMongoDriver) CreateUser(ctx context.Context, row *models.User) (*models.User, error) {
	if m == nil || m.Database == nil || row == nil {
		return nil, errors.New("mongodb: CreateUser invalid input")
	}
	if strings.TrimSpace(row.ID) == "" {
		return nil, errors.New("mongodb: user id required")
	}
	now := time.Now().UTC()
	if row.CreatedAt.IsZero() {
		row.CreatedAt = now
	}
	row.UpdatedAt = now
	if strings.TrimSpace(row.Status) == "" {
		row.Status = models.UserStatusActive
	}
	if strings.TrimSpace(row.Provider) == "" {
		row.Provider = models.UserProviderLocal
	}
	_, err := m.Database.Collection(projectUsersCollection).InsertOne(ctx, row)
	return row, err
}

func (m *SystemMongoDriver) GetUser(ctx context.Context, projectID, userID string) (*models.User, error) {
	if m == nil || m.Database == nil {
		return nil, errors.New("mongodb: nil driver")
	}
	var row models.User
	err := m.Database.Collection(projectUsersCollection).FindOne(ctx, bson.M{
		"_id":        userID,
		"project_id": projectID,
	}).Decode(&row)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

func (m *SystemMongoDriver) ListUsersByEmail(ctx context.Context, projectID, email string) ([]*models.User, error) {
	e := strings.TrimSpace(strings.ToLower(email))
	if e == "" {
		return nil, nil
	}
	filter := bson.M{
		"project_id": projectID,
		"$expr": bson.M{"$eq": bson.A{
			bson.M{"$toLower": bson.M{"$ifNull": bson.A{"$email", ""}}},
			e,
		}},
	}
	return m.findUsers(ctx, filter)
}

func (m *SystemMongoDriver) ListUsersByPhone(ctx context.Context, projectID, phone string) ([]*models.User, error) {
	norm := models.NormalizeUserPhoneKey(phone)
	if norm == "" {
		return nil, nil
	}
	filter := bson.M{"$and": bson.A{
		bson.M{"project_id": projectID},
		mongoUserPhoneOrLegacyUsername(norm),
	}}
	return m.findUsers(ctx, filter)
}

func (m *SystemMongoDriver) ListUsersByGoogleSub(ctx context.Context, projectID, googleSub string) ([]*models.User, error) {
	g := strings.TrimSpace(googleSub)
	if g == "" {
		return nil, nil
	}
	return m.findUsers(ctx, bson.M{"project_id": projectID, "google_sub": g})
}

func (m *SystemMongoDriver) findUsers(ctx context.Context, filter bson.M) ([]*models.User, error) {
	cur, err := m.Database.Collection(projectUsersCollection).Find(ctx, filter, options.Find().SetSort(bson.M{"created_at": 1}))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var out []*models.User
	for cur.Next(ctx) {
		var row models.User
		if err := cur.Decode(&row); err != nil {
			return nil, err
		}
		cp := row
		out = append(out, &cp)
	}
	return out, cur.Err()
}

func (m *SystemMongoDriver) SearchProjectUsers(ctx context.Context, projectID string, limit, offset int) ([]*models.User, int, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}
	if offset < 0 {
		offset = 0
	}
	filter := bson.M{"project_id": projectID}
	count, err := m.Database.Collection(projectUsersCollection).CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	cur, err := m.Database.Collection(projectUsersCollection).Find(ctx, filter,
		options.Find().SetSort(bson.M{"created_at": 1}).SetLimit(int64(limit)).SetSkip(int64(offset)))
	if err != nil {
		return nil, 0, err
	}
	defer cur.Close(ctx)
	var out []*models.User
	for cur.Next(ctx) {
		var row models.User
		if err := cur.Decode(&row); err != nil {
			return nil, 0, err
		}
		cp := row
		out = append(out, &cp)
	}
	return out, int(count), cur.Err()
}

func (m *SystemMongoDriver) CountProjectUsersByRole(ctx context.Context, projectID string) (map[string]int, error) {
	if m == nil || m.Database == nil {
		return nil, errors.New("mongodb: nil driver")
	}
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{"project_id": projectID}}},
		{{Key: "$group", Value: bson.M{
			"_id":   "$role",
			"count": bson.M{"$sum": 1},
		}}},
	}
	cur, err := m.Database.Collection(projectUsersCollection).Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	out := make(map[string]int)
	for cur.Next(ctx) {
		var row struct {
			Role  string `bson:"_id"`
			Count int    `bson:"count"`
		}
		if err := cur.Decode(&row); err != nil {
			return nil, err
		}
		out[row.Role] = row.Count
	}
	return out, cur.Err()
}

func (m *SystemMongoDriver) UpdateUser(ctx context.Context, row *models.User) error {
	if row == nil || row.ID == "" {
		return errors.New("mongodb: UpdateUser invalid input")
	}
	row.UpdatedAt = time.Now().UTC()
	_, err := m.Database.Collection(projectUsersCollection).UpdateOne(ctx,
		bson.M{"_id": row.ID, "project_id": row.ProjectID},
		bson.M{"$set": bson.M{
			"username": row.Username, "email": row.Email, "phone": row.Phone,
			"secret": row.Secret, "role": row.Role, "provider": row.Provider,
			"google_sub": row.GoogleSub, "status": row.Status, "updated_at": row.UpdatedAt,
		}},
	)
	return err
}

func (m *SystemMongoDriver) DeleteUser(ctx context.Context, projectID, userID string) error {
	_, err := m.Database.Collection(projectUsersCollection).DeleteOne(ctx, bson.M{"_id": userID, "project_id": projectID})
	return err
}

func (m *SystemMongoDriver) GetUserByUsername(ctx context.Context, projectID, username string) (*models.User, error) {
	u := strings.TrimSpace(username)
	if u == "" {
		return nil, nil
	}
	var row models.User
	err := m.Database.Collection(projectUsersCollection).FindOne(ctx, bson.M{
		"project_id": projectID, "username": u,
	}).Decode(&row)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}
