package sqlite

import (
	"context"
	"database/sql"
	"errors"

	"example.com/service/internal/accounting/application"
	"example.com/service/internal/accounting/domain"
	_ "github.com/mattn/go-sqlite3"
)

type Store struct{ db *sql.DB }

var _ application.Repository = (*Store)(nil)

func Open(filename string) (*Store, error) {
	db, err := sql.Open("sqlite3", filename+"?_foreign_keys=on&_busy_timeout=5000&_txlock=immediate")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS quotas(tenant TEXT PRIMARY KEY, limit_units INTEGER NOT NULL, reserved INTEGER NOT NULL DEFAULT 0, used INTEGER NOT NULL DEFAULT 0);
 CREATE TABLE IF NOT EXISTS reservations(tenant TEXT NOT NULL REFERENCES quotas(tenant), request_id TEXT NOT NULL, units INTEGER NOT NULL, settled INTEGER, PRIMARY KEY(tenant,request_id));`)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}
func (s *Store) Close() error { return s.db.Close() }

func (s *Store) SetLimit(ctx context.Context, tenant string, limit int64) error {
	result, err := s.db.ExecContext(ctx, `INSERT INTO quotas(tenant,limit_units) VALUES(?,?) ON CONFLICT(tenant) DO UPDATE SET limit_units=excluded.limit_units WHERE quotas.used+quotas.reserved<=excluded.limit_units`, tenant, limit)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err == nil && n == 0 {
		return domain.ErrQuota
	}
	return err
}
func quota(ctx context.Context, query interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, tenant string) (domain.Quota, error) {
	q := domain.Quota{Tenant: tenant}
	err := query.QueryRowContext(ctx, `SELECT limit_units,reserved,used FROM quotas WHERE tenant=?`, tenant).Scan(&q.Limit, &q.Reserved, &q.Used)
	if errors.Is(err, sql.ErrNoRows) {
		err = application.ErrNotFound
	}
	return q, err
}
func (s *Store) Usage(ctx context.Context, tenant string) (domain.Quota, error) {
	return quota(ctx, s.db, tenant)
}

func (s *Store) Reserve(ctx context.Context, tenant, id string, units int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var previous int64
	err = tx.QueryRowContext(ctx, `SELECT units FROM reservations WHERE tenant=? AND request_id=?`, tenant, id).Scan(&previous)
	if err == nil {
		if previous != units {
			return domain.ErrConflict
		}
		return tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	q, err := quota(ctx, tx, tenant)
	if err != nil {
		return err
	}
	if err = q.CanReserve(units); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO reservations(tenant,request_id,units) VALUES(?,?,?)`, tenant, id, units); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE quotas SET reserved=reserved+? WHERE tenant=?`, units, tenant); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) Finish(ctx context.Context, tenant, id string, used int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var units int64
	var settled sql.NullInt64
	err = tx.QueryRowContext(ctx, `SELECT units,settled FROM reservations WHERE tenant=? AND request_id=?`, tenant, id).Scan(&units, &settled)
	if errors.Is(err, sql.ErrNoRows) {
		return application.ErrNotFound
	}
	if err != nil {
		return err
	}
	reservation := domain.Reservation{Units: units}
	if settled.Valid {
		reservation.Settled = &settled.Int64
	}
	if err := reservation.CheckCompletion(used); err != nil {
		return err
	}
	if settled.Valid {
		return tx.Commit()
	}
	if _, err = tx.ExecContext(ctx, `UPDATE reservations SET settled=? WHERE tenant=? AND request_id=?`, used, tenant, id); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE quotas SET reserved=reserved-?,used=used+? WHERE tenant=?`, units, used, tenant); err != nil {
		return err
	}
	return tx.Commit()
}
