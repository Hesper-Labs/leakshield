package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// User is an employee row.
type User struct {
	ID         uuid.UUID
	CompanyID  uuid.UUID
	Name       string
	Surname    *string
	Email      string
	Phone      *string
	Department *string
	Role       string
	IsActive   bool
	CreatedAt  time.Time
}

// CreateUserParams holds the inputs for creating an employee.
type CreateUserParams struct {
	Name       string
	Surname    string
	Email      string
	Phone      string
	Department string
	Role       string
}

// CreateUser inserts a new employee inside the tenant transaction.
func (d *DB) CreateUser(ctx context.Context, tenantID uuid.UUID, p CreateUserParams) (*User, error) {
	if p.Role == "" {
		p.Role = "member"
	}
	u := &User{}
	err := d.WithTenant(ctx, tenantID.String(), func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			INSERT INTO users (company_id, name, surname, email, phone, department, role, is_active)
			VALUES ($1, $2, NULLIF($3,''), $4, NULLIF($5,''), NULLIF($6,''), $7, true)
			RETURNING id, company_id, name, surname, email, phone, department, role, is_active, created_at
		`, tenantID, p.Name, p.Surname, p.Email, p.Phone, p.Department, p.Role).Scan(
			&u.ID, &u.CompanyID, &u.Name, &u.Surname, &u.Email, &u.Phone, &u.Department, &u.Role, &u.IsActive, &u.CreatedAt,
		)
	})
	if err != nil {
		return nil, err
	}
	return u, nil
}

// FindUserByID loads a user inside the tenant context.
func (d *DB) FindUserByID(ctx context.Context, tenantID, id uuid.UUID) (*User, error) {
	u := &User{}
	err := d.WithTenant(ctx, tenantID.String(), func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `
			SELECT id, company_id, name, surname, email, phone, department, role, is_active, created_at
			FROM users WHERE id = $1
		`, id).Scan(&u.ID, &u.CompanyID, &u.Name, &u.Surname, &u.Email, &u.Phone, &u.Department, &u.Role, &u.IsActive, &u.CreatedAt)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	})
	if err != nil {
		return nil, err
	}
	return u, nil
}

// FindUserByEmail loads a user by email inside the tenant context.
func (d *DB) FindUserByEmail(ctx context.Context, tenantID uuid.UUID, email string) (*User, error) {
	u := &User{}
	err := d.WithTenant(ctx, tenantID.String(), func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `
			SELECT id, company_id, name, surname, email, phone, department, role, is_active, created_at
			FROM users WHERE email = $1
		`, email).Scan(&u.ID, &u.CompanyID, &u.Name, &u.Surname, &u.Email, &u.Phone, &u.Department, &u.Role, &u.IsActive, &u.CreatedAt)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	})
	if err != nil {
		return nil, err
	}
	return u, nil
}

// ListUsers returns the company's users (paged).
func (d *DB) ListUsers(ctx context.Context, tenantID uuid.UUID, limit, offset int) ([]*User, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	out := make([]*User, 0, limit)
	err := d.WithTenant(ctx, tenantID.String(), func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id, company_id, name, surname, email, phone, department, role, is_active, created_at
			FROM users
			ORDER BY created_at DESC
			LIMIT $1 OFFSET $2
		`, limit, offset)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			u := &User{}
			if err := rows.Scan(&u.ID, &u.CompanyID, &u.Name, &u.Surname, &u.Email, &u.Phone, &u.Department, &u.Role, &u.IsActive, &u.CreatedAt); err != nil {
				return err
			}
			out = append(out, u)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}
