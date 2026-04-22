package repository

import (
	"context"
	"errors"

	"namenotdecidedyet/internal/domain/property"
	"namenotdecidedyet/internal/pkg/querybuilder"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PropertyRepo struct {
	pool *pgxpool.Pool
}

func NewPropertyRepo(pool *pgxpool.Pool) *PropertyRepo {
	return &PropertyRepo{pool: pool}
}

func (r *PropertyRepo) scanOne(row pgx.Row) (*property.Property, error) {
	var p property.Property
	err := row.Scan(
		&p.ID, &p.OwnerID, &p.Title, &p.Description, &p.PropertyType,
		&p.LocationLat, &p.LocationLng, &p.AddressText, &p.City, &p.Locality,
		&p.RentAmount, &p.DepositAmount, &p.TotalCapacity, &p.LifestyleTags,
		&p.Status, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, property.ErrPropertyNotFound
		}
		return nil, err
	}
	return &p, nil
}

// CreateProperty inserts a new property, storing coordinates in a PostGIS geography point.
func (r *PropertyRepo) CreateProperty(ctx context.Context, p *property.Property) (string, error) {
	const query = `
		INSERT INTO properties (
			owner_id, title, description, property_type,
			location, address_text, city, locality,
			rent_amount, deposit_amount, total_capacity, lifestyle_tags, status
		) VALUES (
			$1, $2, $3, $4,
			ST_SetSRID(ST_MakePoint($5, $6), 4326)::geography,
			$7, $8, $9, $10, $11, $12, $13, $14
		) RETURNING id`

	var id string
	err := r.pool.QueryRow(ctx, query,
		p.OwnerID, p.Title, p.Description, p.PropertyType,
		p.LocationLng, p.LocationLat, // ST_MakePoint takes (lon, lat)
		p.AddressText, p.City, p.Locality,
		p.RentAmount, p.DepositAmount, p.TotalCapacity, p.LifestyleTags, p.Status,
	).Scan(&id)

	if err != nil {
		return "", err
	}
	return id, nil
}

// GetPropertyByID fetches a property, extracting coordinates from PostGIS.
func (r *PropertyRepo) GetPropertyByID(ctx context.Context, id string) (*property.Property, error) {
	const query = `
		SELECT id, owner_id, title, description, property_type,
		       ST_Y(location::geometry) as lat, ST_X(location::geometry) as lng,
		       address_text, city, locality, rent_amount, deposit_amount,
		       total_capacity, lifestyle_tags, status, created_at, updated_at
		FROM   properties
		WHERE  id = $1
		  AND  deleted_at IS NULL`

	return r.scanOne(r.pool.QueryRow(ctx, query, id))
}

// SearchProperties uses the querybuilder to dynamically construct a spatial search.
func (r *PropertyRepo) SearchProperties(ctx context.Context, filter property.SearchFilter) ([]property.Property, error) {
	const baseSelect = `
		SELECT id, owner_id, title, description, property_type,
		       ST_Y(location::geometry) as lat, ST_X(location::geometry) as lng,
		       address_text, city, locality, rent_amount, deposit_amount,
		       total_capacity, lifestyle_tags, status, created_at, updated_at
		FROM   properties`

	qb := querybuilder.New(baseSelect)
	qb.Where("deleted_at IS NULL")
	qb.Where("status != 'delisted'")

	if filter.Lat != nil && filter.Lng != nil && filter.RadiusKm != nil {
		// Convert km to metres for ST_DWithin
		radiusMetres := *filter.RadiusKm * 1000
		qb.WhereParam("ST_DWithin(location, ST_MakePoint($?, $?)::geography, $?)", *filter.Lng, *filter.Lat, radiusMetres)
	}

	if filter.City != nil {
		qb.WhereParam("city ILIKE $?", "%"+*filter.City+"%")
	}
	if filter.Locality != nil {
		qb.WhereParam("locality ILIKE $?", "%"+*filter.Locality+"%")
	}
	if filter.MinRent != nil {
		qb.WhereParam("rent_amount >= $?", *filter.MinRent)
	}
	if filter.MaxRent != nil {
		qb.WhereParam("rent_amount <= $?", *filter.MaxRent)
	}

	qb.OrderBy("created_at DESC")
	qb.Limit(50) // Cap results to 50

	sql, args := qb.Build()

	rows, err := r.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []property.Property
	for rows.Next() {
		var p property.Property
		if err := rows.Scan(
			&p.ID, &p.OwnerID, &p.Title, &p.Description, &p.PropertyType,
			&p.LocationLat, &p.LocationLng, &p.AddressText, &p.City, &p.Locality,
			&p.RentAmount, &p.DepositAmount, &p.TotalCapacity, &p.LifestyleTags,
			&p.Status, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, err
		}
		results = append(results, p)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return results, nil
}

// UpdateStatus changes the status of a property.
func (r *PropertyRepo) UpdateStatus(ctx context.Context, id string, status string) error {
	const query = `
		UPDATE properties
		SET    status = $1
		WHERE  id = $2
		  AND  deleted_at IS NULL`

	cmd, err := r.pool.Exec(ctx, query, status, id)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return property.ErrPropertyNotFound
	}
	return nil
}
