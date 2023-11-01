package repository

import (
	"context"
	"fmt"
	"go-form-hub/internal/database"
	"go-form-hub/internal/model"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
	"github.com/rs/zerolog/log"
)

type Form struct {
	Title     string    `db:"title"`
	ID        *int64    `db:"id"`
	AuthorID  int64     `db:"author_id"`
	CreatedAt time.Time `db:"created_at"`
}

type formDatabaseRepository struct {
	db      database.ConnPool
	builder squirrel.StatementBuilderType
}

func NewFormDatabaseRepository(db database.ConnPool, builder squirrel.StatementBuilderType) FormRepository {
	return &formDatabaseRepository{
		db:      db,
		builder: builder,
	}
}

func (r *formDatabaseRepository) FindAll(ctx context.Context) (forms []*Form, authors map[int64]*User, err error) {
	query, _, err := r.builder.Select("f.id", "f.title", "f.author_id", "f.created_at", "u.id", "u.username", "u.first_name", "u.last_name", "u.email").
		From(fmt.Sprintf("%s.form as f", r.db.GetSchema())).
		Join(fmt.Sprintf("%s.user as u ON f.author_id = u.id", r.db.GetSchema())).ToSql()

	if err != nil {
		return nil, nil, fmt.Errorf("form_repository find_all failed to build query: %e", err)
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("form_repository find_all failed to begin transaction: %e", err)
	}

	defer func() {
		switch err {
		case nil:
			err = tx.Commit(ctx)
		default:
			_ = tx.Rollback(ctx)
		}
	}()

	rows, err := tx.Query(ctx, query)
	if err != nil {
		return nil, nil, fmt.Errorf("form_repository find_all failed to execute query: %e", err)
	}

	forms, authors, err = r.fromRows(rows)

	return forms, authors, err
}

func (r *formDatabaseRepository) FindByID(ctx context.Context, id int64) (form *Form, author *User, err error) {
	query, args, err := r.builder.Select("f.id", "f.title", "f.author_id", "f.created_at", "u.id", "u.username", "u.first_name", "u.last_name", "u.email").
		From(fmt.Sprintf("%s.form as f", r.db.GetSchema())).
		Join(fmt.Sprintf("%s.user as u ON f.author_id = u.id", r.db.GetSchema())).
		Where(squirrel.Eq{"f.id": id}).ToSql()

	if err != nil {
		return nil, nil, fmt.Errorf("form_repository find_by_title failed to build query: %e", err)
	}

	log.Info().Msgf("query: %s", query)

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("form_repository find_by_title failed to begin transaction: %e", err)
	}

	defer func() {
		switch err {
		case nil:
			err = tx.Commit(ctx)
		default:
			_ = tx.Rollback(ctx)
		}
	}()

	row := tx.QueryRow(ctx, query, args...)
	if row == nil {
		err = fmt.Errorf("form_repository find_by_title failed to execute query: %e", err)
	}

	form, author, err = r.fromRow(row)

	return form, author, err
}

func (r *formDatabaseRepository) Insert(ctx context.Context, form *Form) (*int64, error) {
	query, args, err := r.builder.Insert(fmt.Sprintf("%s.form", r.db.GetSchema())).
		Columns("title", "author_id", "created_at").
		Values(form.Title, form.AuthorID, form.CreatedAt).
		Suffix("RETURNING id").ToSql()
	if err != nil {
		return nil, fmt.Errorf("form_repository insert failed to build query: %e", err)
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("form_repository insert failed to begin transaction: %e", err)
	}

	defer func() {
		switch err {
		case nil:
			err = tx.Commit(ctx)
		default:
			_ = tx.Rollback(ctx)
		}
	}()

	row := tx.QueryRow(ctx, query, args...)
	if row == nil {
		return nil, fmt.Errorf("form_repository insert failed to execute query: %e", err)
	}

	var id int64
	err = row.Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("form_repository insert failed to return id: %e", err)
	}

	return &id, nil
}

func (r *formDatabaseRepository) Update(ctx context.Context, id int64, form *Form) (result *Form, err error) {
	query, args, err := r.builder.Update(fmt.Sprintf("%s.form", r.db.GetSchema())).
		Set("title", form.Title).
		Where(squirrel.Eq{"id": id}).
		Suffix("RETURNING id, title, created_at").ToSql()
	if err != nil {
		return nil, fmt.Errorf("form_repository update failed to build query: %e", err)
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("form_repository update failed to begin transaction: %e", err)
	}

	defer func() {
		switch err {
		case nil:
			err = tx.Commit(ctx)
		default:
			_ = tx.Rollback(ctx)
		}
	}()

	rows := tx.QueryRow(ctx, query, args...)
	err = rows.Scan(&form.ID, &form.Title, &form.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("form_repository update failed to execute query: %e", err)
	}

	result = form
	return result, nil
}

func (r *formDatabaseRepository) Delete(ctx context.Context, id int64) (err error) {
	query, args, err := r.builder.Delete(fmt.Sprintf("%s.form", r.db.GetSchema())).
		Where(squirrel.Eq{"id": id}).ToSql()
	if err != nil {
		return fmt.Errorf("form_repository delete failed to build query: %e", err)
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("form_repository delete failed to begin transaction: %e", err)
	}

	defer func() {
		switch err {
		case nil:
			err = tx.Commit(ctx)
		default:
			_ = tx.Rollback(ctx)
		}
	}()

	_, err = tx.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("form_repository delete failed to execute query: %e", err)
	}

	return nil
}

func (r *formDatabaseRepository) ToModel(form *Form, author *User) *model.Form {
	return &model.Form{
		ID:    form.ID,
		Title: form.Title,
		Author: &model.UserGet{
			ID:        author.ID,
			Username:  author.Username,
			FirstName: author.FirstName,
			LastName:  author.LastName,
			Email:     author.Email,
		},
		CreatedAt: form.CreatedAt,
	}
}

func (r *formDatabaseRepository) FromModel(form *model.Form) *Form {
	return &Form{
		ID:        form.ID,
		Title:     form.Title,
		AuthorID:  form.Author.ID,
		CreatedAt: form.CreatedAt,
	}
}

func (r *formDatabaseRepository) fromRows(rows pgx.Rows) ([]*Form, map[int64]*User, error) {
	defer func() {
		rows.Close()
	}()

	forms := []*Form{}
	authors := map[int64]*User{}

	for rows.Next() {
		form, author, err := r.fromRow(rows)
		if err != nil {
			return nil, nil, fmt.Errorf("user_repository from_rows failed: %e", err)
		}

		forms = append(forms, form)
		authors[form.AuthorID] = author
	}

	return forms, authors, nil
}

func (r *formDatabaseRepository) fromRow(row pgx.Row) (*Form, *User, error) {
	form := &Form{}
	author := &User{}
	err := row.Scan(&form.ID, &form.Title, &form.AuthorID, &form.CreatedAt, &author.ID, &author.Username, &author.FirstName, &author.LastName, &author.Email)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil, nil
		}

		return nil, nil, fmt.Errorf("user_repository failed to scan row: %e", err)
	}

	return form, author, nil
}
