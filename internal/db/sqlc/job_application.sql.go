// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.29.0
// source: job_application.sql

package db

import (
	"context"
	"database/sql"
	"time"
)

const createJobApplication = `-- name: CreateJobApplication :one
INSERT INTO job_applications (user_id, job_id, message, cv)
VALUES ($1, $2, $3, $4)
RETURNING id, user_id, job_id, message, cv, status, applied_at, notification
`

type CreateJobApplicationParams struct {
	UserID  int32          `json:"user_id"`
	JobID   int32          `json:"job_id"`
	Message sql.NullString `json:"message"`
	Cv      []byte         `json:"cv"`
}

func (q *Queries) CreateJobApplication(ctx context.Context, arg CreateJobApplicationParams) (JobApplication, error) {
	row := q.db.QueryRowContext(ctx, createJobApplication,
		arg.UserID,
		arg.JobID,
		arg.Message,
		arg.Cv,
	)
	var i JobApplication
	err := row.Scan(
		&i.ID,
		&i.UserID,
		&i.JobID,
		&i.Message,
		&i.Cv,
		&i.Status,
		&i.AppliedAt,
		&i.Notification,
	)
	return i, err
}

const deleteJobApplication = `-- name: DeleteJobApplication :exec
DELETE
FROM job_applications
WHERE id = $1
`

func (q *Queries) DeleteJobApplication(ctx context.Context, id int32) error {
	_, err := q.db.ExecContext(ctx, deleteJobApplication, id)
	return err
}

const getJobApplicationForEmployer = `-- name: GetJobApplicationForEmployer :one
SELECT ja.id         AS application_id,
       j.title       AS job_title,
       j.id          AS job_id,
       ja.status     AS application_status,
       ja.applied_at AS application_date,
       ja.message    AS application_message,
       ja.cv         AS user_cv,
       ja.user_id    AS user_id,
       u.email       AS user_email,
       u.full_name   AS user_full_name,
       u.location    AS user_location,
       c.id          AS company_id
FROM job_applications ja
         JOIN jobs j ON ja.job_id = j.id
         JOIN companies c ON j.company_id = c.id
         JOIN users u ON ja.user_id = u.id
WHERE ja.id = $1
`

type GetJobApplicationForEmployerRow struct {
	ApplicationID      int32             `json:"application_id"`
	JobTitle           string            `json:"job_title"`
	JobID              int32             `json:"job_id"`
	ApplicationStatus  ApplicationStatus `json:"application_status"`
	ApplicationDate    time.Time         `json:"application_date"`
	ApplicationMessage sql.NullString    `json:"application_message"`
	UserCv             []byte            `json:"user_cv"`
	UserID             int32             `json:"user_id"`
	UserEmail          string            `json:"user_email"`
	UserFullName       string            `json:"user_full_name"`
	UserLocation       string            `json:"user_location"`
	CompanyID          int32             `json:"company_id"`
}

// this function will be used by employers
func (q *Queries) GetJobApplicationForEmployer(ctx context.Context, id int32) (GetJobApplicationForEmployerRow, error) {
	row := q.db.QueryRowContext(ctx, getJobApplicationForEmployer, id)
	var i GetJobApplicationForEmployerRow
	err := row.Scan(
		&i.ApplicationID,
		&i.JobTitle,
		&i.JobID,
		&i.ApplicationStatus,
		&i.ApplicationDate,
		&i.ApplicationMessage,
		&i.UserCv,
		&i.UserID,
		&i.UserEmail,
		&i.UserFullName,
		&i.UserLocation,
		&i.CompanyID,
	)
	return i, err
}

const getJobApplicationForUser = `-- name: GetJobApplicationForUser :one
SELECT ja.id         AS application_id,
       j.id          AS job_id,
       j.title       AS job_title,
       c.name        AS company_name,
       ja.status     AS application_status,
       ja.applied_at AS application_date,
       ja.message    AS application_message,
       ja.cv         AS user_cv,
       ja.user_id    AS user_id,
       ja.notification AS notification
FROM job_applications ja
         JOIN jobs j ON ja.job_id = j.id
         JOIN companies c ON j.company_id = c.id
WHERE ja.id = $1
`

type GetJobApplicationForUserRow struct {
	ApplicationID      int32             `json:"application_id"`
	JobID              int32             `json:"job_id"`
	JobTitle           string            `json:"job_title"`
	CompanyName        string            `json:"company_name"`
	ApplicationStatus  ApplicationStatus `json:"application_status"`
	ApplicationDate    time.Time         `json:"application_date"`
	ApplicationMessage sql.NullString    `json:"application_message"`
	UserCv             []byte            `json:"user_cv"`
	UserID             int32             `json:"user_id"`
	Notification       bool              `json:"notification"`
}

// this function will be used by users only
func (q *Queries) GetJobApplicationForUser(ctx context.Context, id int32) (GetJobApplicationForUserRow, error) {
	row := q.db.QueryRowContext(ctx, getJobApplicationForUser, id)
	var i GetJobApplicationForUserRow
	err := row.Scan(
		&i.ApplicationID,
		&i.JobID,
		&i.JobTitle,
		&i.CompanyName,
		&i.ApplicationStatus,
		&i.ApplicationDate,
		&i.ApplicationMessage,
		&i.UserCv,
		&i.UserID,
		&i.Notification,
	)
	return i, err
}

const getJobApplicationUserID = `-- name: GetJobApplicationUserID :one
SELECT user_id
FROM job_applications
WHERE id = $1
`

func (q *Queries) GetJobApplicationUserID(ctx context.Context, id int32) (int32, error) {
	row := q.db.QueryRowContext(ctx, getJobApplicationUserID, id)
	var user_id int32
	err := row.Scan(&user_id)
	return user_id, err
}

const getJobApplicationUserIDAndStatus = `-- name: GetJobApplicationUserIDAndStatus :one
SELECT user_id, status
FROM job_applications
WHERE id = $1
`

type GetJobApplicationUserIDAndStatusRow struct {
	UserID int32             `json:"user_id"`
	Status ApplicationStatus `json:"status"`
}

func (q *Queries) GetJobApplicationUserIDAndStatus(ctx context.Context, id int32) (GetJobApplicationUserIDAndStatusRow, error) {
	row := q.db.QueryRowContext(ctx, getJobApplicationUserIDAndStatus, id)
	var i GetJobApplicationUserIDAndStatusRow
	err := row.Scan(&i.UserID, &i.Status)
	return i, err
}

const getJobIDOfJobApplication = `-- name: GetJobIDOfJobApplication :one
SELECT job_id
FROM job_applications
WHERE id = $1
`

func (q *Queries) GetJobIDOfJobApplication(ctx context.Context, id int32) (int32, error) {
	row := q.db.QueryRowContext(ctx, getJobIDOfJobApplication, id)
	var job_id int32
	err := row.Scan(&job_id)
	return job_id, err
}

const listJobApplicationsForEmployer = `-- name: ListJobApplicationsForEmployer :many
SELECT ja.id         AS application_id,
       ja.user_id    AS user_id,
       u.email       AS user_email,
       u.full_name   AS user_full_name,
       ja.status     AS application_status,
       ja.applied_at AS application_date
FROM job_applications ja
         JOIN users u ON u.id = ja.user_id
WHERE ja.job_id = $1
  AND ($4::bool = TRUE AND ja.status = $5 OR $4::bool = FALSE)
ORDER BY CASE WHEN $6::bool THEN ja.applied_at END ASC,
         CASE WHEN $7::bool THEN ja.applied_at END DESC,
         ja.applied_at DESC
LIMIT $2 OFFSET $3
`

type ListJobApplicationsForEmployerParams struct {
	JobID         int32             `json:"job_id"`
	Limit         int32             `json:"limit"`
	Offset        int32             `json:"offset"`
	FilterStatus  bool              `json:"filter_status"`
	Status        ApplicationStatus `json:"status"`
	AppliedAtAsc  bool              `json:"applied_at_asc"`
	AppliedAtDesc bool              `json:"applied_at_desc"`
}

type ListJobApplicationsForEmployerRow struct {
	ApplicationID     int32             `json:"application_id"`
	UserID            int32             `json:"user_id"`
	UserEmail         string            `json:"user_email"`
	UserFullName      string            `json:"user_full_name"`
	ApplicationStatus ApplicationStatus `json:"application_status"`
	ApplicationDate   time.Time         `json:"application_date"`
}

func (q *Queries) ListJobApplicationsForEmployer(ctx context.Context, arg ListJobApplicationsForEmployerParams) ([]ListJobApplicationsForEmployerRow, error) {
	rows, err := q.db.QueryContext(ctx, listJobApplicationsForEmployer,
		arg.JobID,
		arg.Limit,
		arg.Offset,
		arg.FilterStatus,
		arg.Status,
		arg.AppliedAtAsc,
		arg.AppliedAtDesc,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []ListJobApplicationsForEmployerRow{}
	for rows.Next() {
		var i ListJobApplicationsForEmployerRow
		if err := rows.Scan(
			&i.ApplicationID,
			&i.UserID,
			&i.UserEmail,
			&i.UserFullName,
			&i.ApplicationStatus,
			&i.ApplicationDate,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const listJobApplicationsForUser = `-- name: ListJobApplicationsForUser :many
SELECT ja.user_id    AS user_id,
       ja.id         AS application_id,
       ja.notification AS notification,
       j.title       AS job_title,
       j.id          AS job_id,
       c.name        AS company_name,
       ja.status     AS application_status,
       ja.applied_at AS application_date
FROM job_applications ja
         JOIN jobs j ON ja.job_id = j.id
         JOIN companies c ON j.company_id = c.id
WHERE ja.user_id = $1
  AND ($4::bool = TRUE AND ja.status = $5 OR $4::bool = FALSE)
ORDER BY CASE WHEN $6::bool THEN ja.applied_at END ASC,
         CASE WHEN $7::bool THEN ja.applied_at END DESC,
         ja.applied_at DESC
LIMIT $2 OFFSET $3
`

type ListJobApplicationsForUserParams struct {
	UserID        int32             `json:"user_id"`
	Limit         int32             `json:"limit"`
	Offset        int32             `json:"offset"`
	FilterStatus  bool              `json:"filter_status"`
	Status        ApplicationStatus `json:"status"`
	AppliedAtAsc  bool              `json:"applied_at_asc"`
	AppliedAtDesc bool              `json:"applied_at_desc"`
}

type ListJobApplicationsForUserRow struct {
	UserID            int32             `json:"user_id"`
	ApplicationID     int32             `json:"application_id"`
	Notification      bool              `json:"notification"`
	JobTitle          string            `json:"job_title"`
	JobID             int32             `json:"job_id"`
	CompanyName       string            `json:"company_name"`
	ApplicationStatus ApplicationStatus `json:"application_status"`
	ApplicationDate   time.Time         `json:"application_date"`
}

func (q *Queries) ListJobApplicationsForUser(ctx context.Context, arg ListJobApplicationsForUserParams) ([]ListJobApplicationsForUserRow, error) {
	rows, err := q.db.QueryContext(ctx, listJobApplicationsForUser,
		arg.UserID,
		arg.Limit,
		arg.Offset,
		arg.FilterStatus,
		arg.Status,
		arg.AppliedAtAsc,
		arg.AppliedAtDesc,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []ListJobApplicationsForUserRow{}
	for rows.Next() {
		var i ListJobApplicationsForUserRow
		if err := rows.Scan(
			&i.UserID,
			&i.ApplicationID,
			&i.Notification,
			&i.JobTitle,
			&i.JobID,
			&i.CompanyName,
			&i.ApplicationStatus,
			&i.ApplicationDate,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const updateJobApplication = `-- name: UpdateJobApplication :one
UPDATE job_applications
SET message = COALESCE($2, message),
    cv      = COALESCE($3, cv)
WHERE id = $1
RETURNING id, user_id, job_id, message, cv, status, applied_at, notification
`

type UpdateJobApplicationParams struct {
	ID      int32          `json:"id"`
	Message sql.NullString `json:"message"`
	Cv      []byte         `json:"cv"`
}

func (q *Queries) UpdateJobApplication(ctx context.Context, arg UpdateJobApplicationParams) (JobApplication, error) {
	row := q.db.QueryRowContext(ctx, updateJobApplication, arg.ID, arg.Message, arg.Cv)
	var i JobApplication
	err := row.Scan(
		&i.ID,
		&i.UserID,
		&i.JobID,
		&i.Message,
		&i.Cv,
		&i.Status,
		&i.AppliedAt,
		&i.Notification,
	)
	return i, err
}

const updateJobApplicationNote = `-- name: UpdateJobApplicationNote :exec
UPDATE job_applications
SET notification = $2
WHERE id = $1
`

type UpdateJobApplicationNoteParams struct {
	ID           int32 `json:"id"`
	Notification bool  `json:"notification"`
}

func (q *Queries) UpdateJobApplicationNote(ctx context.Context, arg UpdateJobApplicationNoteParams) error {
	_, err := q.db.ExecContext(ctx, updateJobApplicationNote, arg.ID, arg.Notification)
	return err
}

const updateJobApplicationStatus = `-- name: UpdateJobApplicationStatus :exec
UPDATE job_applications
SET status = $2
WHERE id = $1
`

type UpdateJobApplicationStatusParams struct {
	ID     int32             `json:"id"`
	Status ApplicationStatus `json:"status"`
}

func (q *Queries) UpdateJobApplicationStatus(ctx context.Context, arg UpdateJobApplicationStatusParams) error {
	_, err := q.db.ExecContext(ctx, updateJobApplicationStatus, arg.ID, arg.Status)
	return err
}
