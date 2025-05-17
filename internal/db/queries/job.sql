-- name: CreateJob :one
INSERT INTO jobs (title,
                  industry,
                  company_id,
                  description,
                  location,
                  salary_min,
                  salary_max,
                  requirements)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetJob :one
SELECT *
FROM jobs
WHERE id = $1;

-- name: GetJobDetails :one
SELECT j.*,
       c.name      AS company_name,
       c.location  AS company_location,
       c.industry  AS company_industry,
       e.id        AS employer_id,
       e.email     AS employer_email,
       e.full_name AS employer_full_name
FROM jobs j
         JOIN companies c ON j.company_id = c.id
         JOIN employers e ON c.id = e.company_id
WHERE j.id = $1;

-- name: ListJobsByTitle :many
SELECT *
FROM jobs
WHERE title ILIKE '%' || @title::text || '%'
LIMIT $1 OFFSET $2;

-- name: ListJobsByLocation :many
SELECT *
FROM jobs
WHERE location = $1
LIMIT $2 OFFSET $3;

-- name: ListJobsByIndustry :many
SELECT *
FROM jobs
WHERE industry = $1
LIMIT $2 OFFSET $3;

-- name: ListJobsByCompanyID :many
SELECT j.*,
       c.name AS company_name
FROM jobs j
         JOIN companies c ON j.company_id = c.id
WHERE j.company_id = $1
LIMIT $2 OFFSET $3;

-- name: ListJobsByCompanyExactName :many
SELECT j.*,
       c.name AS company_name
FROM jobs j
         JOIN companies c ON j.company_id = c.id
WHERE c.name = $1
LIMIT $2 OFFSET $3;

-- name: ListJobsByCompanyName :many
SELECT j.*,
       c.name AS company_name
FROM jobs j
         JOIN companies c ON j.company_id = c.id
WHERE c.name ILIKE '%' || @name::text || '%'
LIMIT $1 OFFSET $2;

-- name: ListJobsBySalaryRange :many
SELECT *
FROM jobs
WHERE salary_min >= $1
  AND salary_max <= $2
LIMIT $3 OFFSET $4;

-- name: ListJobsMatchingUserSkills :many
SELECT j.*,
       c.name AS company_name
FROM jobs j
         JOIN companies c ON j.company_id = c.id
WHERE j.id IN (SELECT job_id
               FROM job_skills
               WHERE skill IN (SELECT skill
                               FROM user_skills
                               WHERE user_id = $1))
LIMIT $2 OFFSET $3;

-- name: UpdateJob :one
UPDATE jobs
SET title        = $2,
    industry     = $3,
    company_id   = $4,
    description  = $5,
    location     = $6,
    salary_min   = $7,
    salary_max   = $8,
    requirements = $9
WHERE id = $1
RETURNING *;

-- name: DeleteJob :exec
DELETE
FROM jobs
WHERE id = $1;

-- name: ListAllJobsForES :many
SELECT j.id,
       j.title,
       j.industry,
       j.location,
       j.description,
       c.name AS company_name,
       j.salary_min,
       j.salary_max,
       j.requirements
FROM jobs j
         JOIN companies c ON j.company_id = c.id;

-- name: GetCompanyIDOfJob :one
SELECT company_id
FROM jobs
WHERE id = $1;

-- name: ListJobsForEmployer :many
SELECT id,
       title,
       industry,
       description,
       location,
       salary_min,
       salary_max,
       created_at
FROM jobs
WHERE company_id = $1
ORDER BY CASE WHEN @created_at_asc::bool THEN created_at END ASC,
         CASE WHEN @created_at_desc::bool THEN created_at END DESC,
         created_at DESC
LIMIT $2 OFFSET $3;

-- name: GetJobBasicInfo :one
SELECT
    j.title AS job_title,
    c.name AS company_name
FROM jobs j
         JOIN companies c ON j.company_id = c.id
WHERE j.id = $1;